package main

import (
	"context"
	"flag"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/apis/common"
	"golang.conradwood.net/artefact/buildrepo"
	"golang.conradwood.net/artefact/db"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/authremote"
	"golang.conradwood.net/go-easyops/cache"
	"golang.conradwood.net/go-easyops/errors"
	"golang.conradwood.net/go-easyops/server"
	"golang.conradwood.net/go-easyops/utils"
	"google.golang.org/grpc"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	use_v2 = flag.Bool("use_v2", true, "use version2")
	debug  = flag.Bool("debug", false, "debug mode")
	//	bdomain     = flag.String("buildrepo_domain", "", "in order to maintain unique ids each buildrepo needs a unique prefix")
	port        = flag.Int("port", 10000, "The grpc server port")
	idstore     *db.DBArtefactID
	idcache     = cache.NewResolvingCache("idcache", time.Duration(4)*time.Hour, 10000)
	brepo       *buildrepo.BuildRepo
	idcachelock sync.Mutex
)

type artefactServer struct {
}

func main() {
	flag.Parse()
	fmt.Printf("Starting ArtefactServiceServer...\n")
	var err error
	idstore = db.DefaultDBArtefactID()

	fmt.Printf("Starting buildrepo connections...\n")
	brepo = buildrepo.CreateBuildrepo()
	sd := server.NewServerDef()
	sd.Port = *port
	sd.Register = server.Register(
		func(server *grpc.Server) error {
			e := new(artefactServer)
			pb.RegisterArtefactServiceServer(server, e)
			return nil
		},
	)
	err = server.ServerStartup(sd)
	utils.Bail("Unable to start server", err)
	os.Exit(0)
}

/************************************
* grpc functions
************************************/
func (e *artefactServer) GetRepoVersion(ctx context.Context, req *pb.GetVersionRequest) (*pb.Contents, error) {
	adminAccess := auth.IsRoot(ctx)
	_, xerr := requestAccess(ctx, req.Name, req.Domain)
	if xerr != nil {
		return nil, xerr
	}

	lfr, t, err := brepo.ListFiles(ctx, req.Domain, &br.ListFilesRequest{
		Repository: req.Name,
		Branch:     "master",
		BuildID:    req.Version,
		Recursive:  false,
		Dir:        "",
	})
	if err != nil {
		return nil, err
	}
	fmt.Printf("Got version %d\n", req.Version)
	for _, f := range lfr.Entries {
		fmt.Printf(" #%v\n", f)
	}
	ct := &pb.Contents{
		Name:        req.Name,
		Version:     req.Version,
		Type:        pb.ContentType_Artefact,
		AdminAccess: adminAccess,
		Domain:      req.Domain,
		BuildRepo:   t,
	}
	createArtefactReference(ct)

	return ct, nil
}

func (e *artefactServer) Find(ctx context.Context, req *pb.FindRequest) (*pb.ArtefactList, error) {
	u := auth.GetUser(ctx)
	if u == nil {
		return nil, errors.Unauthenticated(ctx, "need user account to find stuff")
	}
	repos, err := brepo.ListRepos(ctx)
	if err != nil {
		return nil, err
	}
	all := &pb.ArtefactList{}
	nm := strings.ToLower(charsOnly(req.NameMatch))
	for _, be := range repos.Entries {
		an := strings.ToLower(charsOnly(be.Name))
		//fmt.Printf("Checking %s\n", an)
		if strings.Contains(an, nm) {
			a := &pb.Contents{
				Name:        be.Name,
				AdminAccess: false,
				Type:        pb.ContentType_Artefact,
				Domain:      be.Domain,
				BuildRepo:   be.Server,
			}
			all.Artefacts = append(all.Artefacts, a)
		}
	}
	fmt.Printf("Found %d artefacts matching \"%s\"\n", len(all.Artefacts), nm)
	cf := ContentFiller{ctx: ctx, warningOnAccessDenied: true}
	for _, a := range all.Artefacts {
		cf.fillContent(a)
	}
	res := &pb.ArtefactList{Artefacts: cf.withRead}
	p := fmt.Sprintf("[%s %s] ", auth.Description(u), u.Email)
	fmt.Printf("%sFound %d matches for findrequest by name \"%s\"\n", p, len(res.Artefacts), req.NameMatch)
	if err != nil {
		return nil, err
	}
	sort.Slice(res.Artefacts, func(i, j int) bool {
		return res.Artefacts[i].Name < res.Artefacts[j].Name
	})
	return res, nil
}

func (e *artefactServer) List(ctx context.Context, req *common.Void) (*pb.ArtefactList, error) {
	if *use_v2 {
		return e.List2(ctx, req)
	}
	u := auth.GetUser(ctx)
	if u == nil {
		return nil, errors.Unauthenticated(ctx, "no user for List()")
	}

	var err error
	resp := &pb.ArtefactList{}
	repos, err := brepo.ListRepos(ctx)
	if err != nil {
		return nil, err
	}
	adminAccess := auth.IsRoot(ctx)
	var wg sync.WaitGroup
	for _, entry := range repos.Entries {
		wg.Add(1)
		go func(e *buildrepo.RepoEntry) {
			defer wg.Done()

			rid, xerr := requestAccess(ctx, e.Name, e.Domain)
			if xerr != nil {
				return
			}

			af := &pb.Contents{
				Name:        e.Name,
				AdminAccess: adminAccess,
				Type:        pb.ContentType_Artefact,
				Domain:      e.Domain,
				ArtefactID:  &pb.ArtefactID{ID: rid},
				BuildRepo:   e.Server,
			}
			resp.Artefacts = append(resp.Artefacts, af)
			glv, lerr := brepo.GetLatestVersion(ctx, af.Domain, &br.GetLatestVersionRequest{
				Repository: af.Name,
				Branch:     "master",
			})
			if lerr != nil {
				err = lerr
			} else {
				af.Version = glv.BuildID
				if glv.BuildMeta != nil {
					af.RepositoryID = glv.BuildMeta.RepositoryID
				}
			}
			createArtefactReference(af)
			if *debug {
				fmt.Printf("Entry: %#v\n", e)
			}
		}(entry)
	}
	wg.Wait()
	if err != nil {
		return nil, err
	}
	sort.Slice(resp.Artefacts, func(i, j int) bool {
		return resp.Artefacts[i].Name < resp.Artefacts[j].Name
	})
	return resp, nil
}

// given an artefactid, get the metadata
func (e *artefactServer) MetaByID(ctx context.Context, req *pb.ID) (*pb.ArtefactMeta, error) {
	af, err := db.DefaultDBArtefactID().ByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	am, err := create_artefact_meta(ctx, af)
	if err != nil {
		return nil, err
	}
	return am, nil
}
func (e *artefactServer) GetContents(ctx context.Context, req *pb.Reference) (*pb.Contents, error) {
	if *use_v2 && strings.HasPrefix(req.Reference, "/") {
		return e.GetContents2(ctx, req)
	}

	ref, err := parseReference(ctx, req.Reference)
	if err != nil {
		return nil, err
	}
	artefact_id, err := requestAccess(ctx, ref.repository, ref.domain)
	if err != nil {
		return nil, err
	}
	af := &pb.ArtefactID{ID: artefact_id, Domain: ref.domain, Name: ref.repository}
	if ref.IsDirectory() {
		return e.GetDirContent(ctx, req, ref)
	}
	if !ref.IsArtefact() {
		return nil, fmt.Errorf("nonartefacts not implemented yet")
	}
	/***********************************************************************
		// render top-level artefact
	***********************************************************************/
	if *debug {
		fmt.Printf("Rendering top level artefact\r\n")
	}
	aa := auth.IsRoot(ctx)
	v := ref.Version()
	dir := "/"
	lf := &br.ListFilesRequest{
		Repository: ref.Repository(),
		Branch:     ref.Branch(),
		BuildID:    v,
		Dir:        dir, // artefact top-level
		Recursive:  false,
	}
	glv, err := brepo.GetLatestVersion(ctx, ref.domain, &br.GetLatestVersionRequest{Repository: lf.Repository, Branch: lf.Branch})
	if err != nil {
		return nil, err
	}
	if glv.BuildMeta == nil {
		return nil, errors.FailedPrecondition(ctx, "buildmeta unavailable")
	}
	lr, _, err := brepo.ListFiles(ctx, ref.domain, lf)
	if err != nil {
		return nil, errors.NotFound(ctx, "repository \"%s\" not found (%s)", ref.Repository(), err)
	}
	res := &pb.Contents{
		Name:         ref.Repository(),
		Version:      v,
		Type:         pb.ContentType_Artefact,
		AdminAccess:  aa,
		Domain:       ref.domain,
		BuildRepo:    ref.buildrepo,
		ArtefactID:   af,
		RepositoryID: glv.BuildMeta.RepositoryID,
	}
	createArtefactReference(res)
	backref := &pb.ArtefactRef{Name: res.Name, Version: res.Version}
	for _, entry := range lr.Entries {
		if entry.Dir != "" {
			continue
		}
		c := &pb.Contents{
			ReferenceVersion: "VERSION_REFERENCE",
			ReferenceLatest:  "VERSION_LATEST",
			Name:             entry.Name,
			Version:          v,
			AdminAccess:      aa,
			Artefact:         backref,
			Path:             entry.Dir,
			Domain:           ref.domain,
			BuildRepo:        ref.buildrepo,
			RepositoryID:     glv.BuildMeta.RepositoryID,
		}
		if entry.Type == 1 {
			c.Type = pb.ContentType_File
		} else if entry.Type == 2 {
			c.Type = pb.ContentType_Directory
		}
		createArtefactReference(c)

		res.Entries = append(res.Entries, c)
		fmt.Printf("%#v\n", entry)
	}
	sortEntries(res)
	fmt.Printf("Contents for reference: %s (%s)\n", req.Reference, ref.String())
	return res, nil
}

func (e *artefactServer) GetDirContent(ctx context.Context, req *pb.Reference, ref *reference) (*pb.Contents, error) {
	fmt.Printf("Artefact: %s, Dir: %s, path: %s\n", ref.repository, ref.name, ref.path)
	dir := fmt.Sprintf("%s/%s", ref.path, ref.name)
	// we make sure "dir" always starts with a /
	if dir[0] != '/' {
		dir = "/" + dir
	}
	lf := &br.ListFilesRequest{
		Repository: ref.repository,
		Branch:     ref.Branch(),
		BuildID:    ref.Version(),
		Dir:        dir, // artefact top-level
		Recursive:  false,
	}
	lr, _, err := brepo.ListFiles(ctx, ref.domain, lf)
	if err != nil {
		return nil, errors.NotFound(ctx, "dir \"%s\" in repository \"%s\" not found (%s)", dir, ref.Repository(), err)
	}
	res := &pb.Contents{
		Name:      ref.Repository(),
		Version:   ref.Version(),
		Type:      pb.ContentType_Artefact,
		Path:      dir,
		BuildRepo: ref.buildrepo,
	}
	backref := &pb.ArtefactRef{Name: res.Name, Version: res.Version}
	aa := false
	for _, entry := range lr.Entries {
		ed := entry.Dir
		if ed == "" {
			ed = "/"
		}
		if ed[0] != '/' {
			ed = "/" + ed
		}

		if ed != dir {
			fmt.Printf("%s: %s!=%s\n", entry.Name, entry.Dir, dir)
			continue
		}
		c := &pb.Contents{
			ReferenceVersion: "VERSION_REFERENCE",
			ReferenceLatest:  "VERSION_LATEST",
			Name:             entry.Name,
			Path:             entry.Dir,
			Version:          ref.Version(),
			AdminAccess:      aa,
			Artefact:         backref,
			Domain:           ref.domain,
			BuildRepo:        ref.buildrepo,
		}
		if entry.Type == 1 {
			c.Type = pb.ContentType_File
			c.Downloadable = true
		} else if entry.Type == 2 {
			c.Type = pb.ContentType_Directory
		}
		createArtefactReference(c)
		res.Entries = append(res.Entries, c)
		fmt.Printf("Entry: %s/%s\n", entry.Dir, entry.Name)
	}
	sortEntries(res)
	return res, nil
}

func sortEntries(c *pb.Contents) {
	sort.Slice(c.Entries, func(i, j int) bool {
		e1 := c.Entries[i]
		e2 := c.Entries[j]
		if e1.Type == e2.Type {
			return e1.Name < e2.Name
		}
		return e1.Type < e2.Type
	})
}

/************************************
* helpers
************************************/
func artefactToID(artefactName string, domain string) (uint64, error) {
	if domain == "" {
		return 0, fmt.Errorf("missing domain for artefact %s", artefactName)
	}
	if artefactName == "" {
		return 0, fmt.Errorf("missing artefactname")
	}
	idcachelock.Lock()
	defer idcachelock.Unlock()
	o, err := idcache.Retrieve(artefactName, func(s string) (interface{}, error) {
		ctx := authremote.Context()
		o, err := idstore.ByName(ctx, artefactName)
		if err != nil {
			return nil, err
		}
		for _, aa := range o {
			if aa.Domain == domain {
				return aa, nil
			}
		}
		a := &pb.ArtefactID{Domain: domain, Name: s}
		_, err = idstore.Save(ctx, a)
		if err != nil {
			return nil, err
		}
		return a, nil

	})
	if err != nil {
		return 0, err
	}
	aid := o.(*pb.ArtefactID)
	return aid.ID, nil
}

func charsOnly(in string) string {
	valid := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	res := ""
	for _, c := range in {
		isvalid := false
		for _, iv := range valid {
			if c == iv {
				isvalid = true
				break
			}
		}
		if !isvalid {
			continue
		}
		res = res + string(c)
	}
	return res
}

// use to asynchronously get details
type ContentFiller struct {
	// fill this in:
	ctx                   context.Context
	warningOnAccessDenied bool
	// in response you'll get:
	err      error
	withRead []*pb.Contents
}

func (cf *ContentFiller) fillContent(af *pb.Contents) {
	u := auth.GetUser(cf.ctx)
	adminAccess := auth.IsRoot(cf.ctx)
	if af.ArtefactID == nil {
		rid, _ := artefactToID(af.Name, af.Domain)
		af.ArtefactID = &pb.ArtefactID{ID: rid, Domain: af.Domain, Name: af.Name}
	}
	_, xerr := requestAccess(cf.ctx, af.Name, af.Domain)
	if xerr != nil {
		if cf.warningOnAccessDenied {
			afid := af.ArtefactID.ID
			fmt.Printf("WARNING: Access denied for user %s to artefact %d(%s): %s", auth.Description(u), afid, af.Name, xerr)
		}
		return
	}
	af.AdminAccess = adminAccess
	cf.withRead = append(cf.withRead, af)

	glv, e := brepo.GetLatestVersion(cf.ctx, af.Domain, &br.GetLatestVersionRequest{
		Repository: af.Name,
		Branch:     "master",
	})
	if e != nil {
		cf.err = e
	} else {
		af.Version = glv.BuildID
		if glv.BuildMeta != nil {
			af.RepositoryID = glv.BuildMeta.RepositoryID
		}
	}
	createArtefactReference(af)
}
