package buildrepo

import (
	"context"
	"flag"
	"fmt"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/apis/common"
	"golang.conradwood.net/go-easyops/authremote"
	"golang.conradwood.net/go-easyops/client"
	"io"
	"strings"
	"sync"
)

/*
* consolidate multiple repositories
 */
var (
	user_buildrepos    = flag.String("buildrepos", "", "if set, a comma delimited mapping with buildrepos addresses")
	default_buildrepos = []string{"buildrepo.vpn.conrad.localdomain", "scbuildrepo.singingcat.localdomain"}

	clients = make(map[string]br.BuildRepoManagerClient)
	debug   = flag.Bool("debug_repos", false, "debug repo code")
	br_meta = make(map[string]*build_repo_meta)
)

type build_repo_meta struct {
	Address string
	Domain  string
}

func get_list_of_buildrepos() []string {
	var res []string
	if *user_buildrepos == "" {
		return default_buildrepos
	}
	for _, brepoadr := range strings.Split(*user_buildrepos, ",") {
		brepoadr = strings.Trim(brepoadr, " ")
		res = append(res, brepoadr)
	}
	return res
}
func get_build_repo_map() map[string]*build_repo_meta {
	return br_meta
}
func CreateBuildrepo() *BuildRepo {
	m := get_list_of_buildrepos()
	debugf("Creating buildrepo clients for %d repos\n", len(m))
	if len(m) == 0 {
		panic("need at least one buildrepo")
	}
	for _, v := range m {
		adr := fmt.Sprintf("%s:5005", v)
		fmt.Printf("Connecting to buildrepo at: %s\n", adr)
		c, err := client.ConnectWithIP(adr)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to buildrepo @ %s: %s", v, err))
		}
		brm := br.NewBuildRepoManagerClient(c)
		clients[v] = brm
		ctx := authremote.Context()
		mi, err := brm.GetManagerInfo(ctx, &common.Void{})
		if err != nil {
			fmt.Printf("failed to get manager info from buildrepo %s: %s\n", adr, err)
		} else {
			fmt.Printf("buildrepo at %s serves domain %s\n", v, mi.Domain)
			br_meta[v] = &build_repo_meta{Address: v, Domain: mi.Domain}
		}
		debugf("Connected to %s\n", adr)
	}
	res := &BuildRepo{}
	return res
}

type BuildRepo struct {
}
type RepoList struct {
	Entries []*RepoEntry
}
type RepoEntry struct {
	*br.RepoEntry
	Server string
}

// get repos from all build servers
func (b *BuildRepo) ListRepos(ctx context.Context) (*RepoList, error) {
	debugf("Listing repos in %d clients\n", len(clients))
	var wg sync.WaitGroup
	var terr error
	res := &RepoList{}
	var addlock sync.Mutex
	for target, c := range clients {
		wg.Add(1)
		go func(t string, bc br.BuildRepoManagerClient) {
			defer wg.Done()
			lr, err := bc.ListRepos(ctx, &br.ListReposRequest{})
			if err != nil {
				terr = err
				return
			}
			addlock.Lock()
			for _, e := range lr.Entries {
				// compatibility - older buildrepos do not provide domains for entries (yet)
				// if so use a default.
				// TODO: update buildrepo server
				if e.Domain == "" {
					d := GetDomainForBuildRepo(t)
					fmt.Printf("WARNING: Buildrepo server %s did not provide domain for \"%s\", using \"%s\"\n", t, e.Name, d)
					e.Domain = d
				}
				re := &RepoEntry{e, t}
				res.Entries = append(res.Entries, re)
			}
			addlock.Unlock()
		}(target, c)
	}
	wg.Wait()
	if terr != nil {
		return nil, terr
	}
	return res, nil
}

// returns: files, buildrepo, error
func (b *BuildRepo) ListFiles(ctx context.Context, domain string, blvr *br.ListFilesRequest) (*br.ListFilesResponse, string, error) {
	if domain == "" {
		return nil, "", fmt.Errorf("missing domain for artefact %s", blvr.Repository)
	}
	t := GetBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, "", fmt.Errorf("(1) no buildrepo server serving %s in domain %s", blvr.Repository, domain)
	}
	lfr, err := c.ListFiles(ctx, blvr)
	return lfr, t, err
}

func (b *BuildRepo) DoesFileExist(ctx context.Context, domain string, blvr *br.GetFileRequest) (*br.FileExistsInfo, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", blvr.File.Repository)
	}
	t := GetBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("(2) no buildrepo server serving %s in domain %s", blvr.File.Repository, domain)
	}
	res, err := c.DoesFileExist(ctx, blvr)
	return res, err

}

func (b *BuildRepo) GetRepositoryMeta(ctx context.Context, domain string, blvr *br.GetRepoMetaRequest) (*br.RepoMetaInfo, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", blvr.Path)
	}

	t := GetBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("(3) no buildrepo server serving %s in domain %s", blvr.Path, domain)
	}
	res, err := c.GetRepositoryMeta(ctx, blvr)
	return res, err

}

// write a file to a writer
func (b *BuildRepo) GetFile(ctx context.Context, domain string, blvr *br.GetFileRequest, target io.Writer) error {
	if domain == "" {
		return fmt.Errorf("missing domain for artefact %s", blvr.File.Repository)
	}
	t := GetBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return fmt.Errorf("(4) no buildrepo server serving %s in domain %s", blvr.File.Repository, domain)
	}
	stream, err := c.GetFileAsStream(ctx, blvr)
	if err != nil {
		return err
	}
	for {
		block, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		_, err = target.Write(block.Data[:block.Size])
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *BuildRepo) GetLatestVersion(ctx context.Context, domain string, glvr *br.GetLatestVersionRequest) (*br.GetLatestVersionResponse, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", glvr.Repository)
	}
	t := GetBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("(5) no buildrepo server serving %s in domain %s", glvr.Repository, domain)
	}
	v, err := c.GetLatestVersion(ctx, glvr)
	if err != nil {
		return nil, err
	}
	return v, nil
}
func (b *BuildRepo) ListVersions(ctx context.Context, domain string, glvr *br.ListVersionsRequest) (*br.ListVersionsResponse, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", glvr.Repository)
	}
	t := GetBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("(6) no buildrepo server serving %s in domain %s", glvr.Repository, domain)
	}
	v, err := c.ListVersions(ctx, glvr)
	if err != nil {
		return nil, err
	}
	return v, nil
}
func GetBuildRepoForDomain(domain string) string {
	for k, v := range get_build_repo_map() {
		if v.Domain == domain {
			return k
		}
	}
	fmt.Printf("WARNING: no buildrepo for domain \"%s\"\n", domain)
	for k, v := range get_build_repo_map() {
		fmt.Printf("Address %s serves %s\n", k, v)
	}
	return ""
}

func GetDomainForBuildRepo(target string) string {
	for k, v := range get_build_repo_map() {
		if k == target {
			return v.Domain
		}
	}
	return ""
}

func (b *BuildRepo) GetBuildRepoManagerClient(repo string, domain string) br.BuildRepoManagerClient {
	a := GetBuildRepoForDomain(domain)
	if a == "" {
		return nil
	}
	return clients[a]
}



