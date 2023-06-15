package main

import (
	"context"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/go-easyops/errors"
	"golang.conradwood.net/go-easyops/utils"
	"regexp"
	"strconv"
	"strings"
)

const (
	URL_PREFIX = "/artefacts/"
	DL_PREFIX  = "/builds/downloads/"
)

// the struct to refer to a reference (file/repo/or so)
type LinkReference struct {
	artefactid uint64
	version    uint64 // 0->latest
	path       string
	artefact   *pb.ArtefactID
	glv        *br.GetLatestVersionResponse
}

func ParseLinkReference(ctx context.Context, link string) (*LinkReference, error) {
	strips := []string{URL_PREFIX, DL_PREFIX}
	ref := ""
	for _, s := range strips {
		idx := strings.Index(link, s)
		if idx != -1 {
			ref = link[idx+len(s):]
			break
		}
	}
	if ref == "" {
		return nil, errors.InvalidArgs(ctx, "invalid path in linkreference", "invalid path in linkreference: '%s'", link)

	}
	lr := &LinkReference{}
	fmt.Printf("Refpath: '%s'\n", ref)
	err := parseURL(ctx, lr, ref)
	if err != nil {
		return nil, err
	}
	if lr.artefactid == 0 {
		return nil, errors.InvalidArgs(ctx, "invalid path in linkreference", "no repositoryid in path  in linkreference: '%s'", ref)
	}

	af, err := idstore.ByID(ctx, lr.artefactid)
	if err != nil {
		return nil, err
	}
	lr.artefact = af

	glv, err := brepo.GetLatestVersion(ctx, lr.artefact.Domain, &br.GetLatestVersionRequest{Repository: lr.artefact.Name, Branch: lr.Branch()})
	if err != nil {
		fmt.Printf("Failed to get latest version for %s: %s\n", lr.String(), utils.ErrorString(err))
		return nil, err
	}
	if glv.BuildMeta == nil {
		fmt.Printf("Got no meta: %s\n", lr.String())
		return nil, err
	}

	lr.glv = glv

	return lr, nil
}

func parseURL(ctx context.Context, lr *LinkReference, path string) error {
	var err error

	regex := regexp.MustCompile(`artefactid/(\d+)/version/latest/(.*)`)
	matches := regex.FindStringSubmatch(path)
	if len(matches) > 1 {
		lr.artefactid, _ = strconv.ParseUint(matches[1], 10, 64)
		lr.path = matches[2]
		return nil
	}

	regex = regexp.MustCompile(`artefactid/(\d+)/version/latest`)
	matches = regex.FindStringSubmatch(path)
	if len(matches) > 0 {
		lr.artefactid, _ = strconv.ParseUint(matches[1], 10, 64)
		lr.path = ""
		return nil
	}

	regex = regexp.MustCompile(`artefactid/(\d+)/version/(\d+)/(.*)`)
	matches = regex.FindStringSubmatch(path)
	if len(matches) > 2 {
		lr.artefactid, _ = strconv.ParseUint(matches[1], 10, 64)
		lr.version, err = strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			return err
		}
		lr.path = matches[3]
		return nil
	}

	return errors.InvalidArgs(ctx, "invalid path", "no matches for path: '%s'", path)

}

// create link to dir
func createDirLink(af *pb.Contents) {
	if af.ArtefactID == nil {
		panic("no artefact id!")
	}
	p := af.Path + "/" + af.Name
	af.LinkToVersion = fmt.Sprintf(URL_PREFIX+"artefactid/%d/version/%d/%s", af.ArtefactID.ID, af.Version, p)
	af.LinkToLatest = fmt.Sprintf(URL_PREFIX+"artefactid/%d/version/latest/%s", af.ArtefactID.ID, p)
}

// create link to download a file
func createFileLink(af *pb.Contents) {
	if af.ArtefactID == nil {
		panic("no artefact id!")
	}
	p := af.Path + "/" + af.Name
	af.LinkToVersion = fmt.Sprintf(DL_PREFIX+"artefactid/%d/version/%d/%s", af.ArtefactID.ID, af.Version, p)
	af.LinkToLatest = fmt.Sprintf(DL_PREFIX+"artefactid/%d/version/latest/%s", af.ArtefactID.ID, p)
}

// create link to artefact
func createArtefactLink(af *pb.Contents) {
	if af.ArtefactID == nil {
		panic("no artefact id!")
	}
	af.LinkToVersion = fmt.Sprintf(URL_PREFIX+"artefactid/%d/version/%d/%s", af.ArtefactID.ID, af.Version, af.Path)
	af.LinkToLatest = fmt.Sprintf(URL_PREFIX+"artefactid/%d/version/latest/%s", af.ArtefactID.ID, af.Path)
}

func (lr *LinkReference) String() string {
	return fmt.Sprintf("artefactid:%d,name:'%s',domain:'%s',path:'%s'", lr.artefactid, lr.artefact.Name, lr.artefact.Domain, lr.path)
}
func (lr *LinkReference) GetArtefact() *pb.ArtefactID {
	return lr.artefact
}
func (lr *LinkReference) Branch() string {
	return "master"
}

// rather than 0 as "latest" will return the actual latest version
// if version is not 0 will return just that
func (lr *LinkReference) ResolvedVersion(ctx context.Context) uint64 {
	if lr.version != 0 {
		return lr.version
	}
	return lr.glv.BuildID
}
func (lr *LinkReference) ArtefactName() string {
	return lr.artefact.Name
}

func (lr *LinkReference) Domain() string {
	return lr.artefact.Domain
}

func (lr *LinkReference) RepositoryID() uint64 {
	return lr.glv.BuildMeta.RepositoryID
}
func (lr *LinkReference) Path() string {
	if lr.path == "" {
		return "/"
	}
	l := lr.path
	if strings.HasPrefix(l, "/") {
		l = "/" + l
	}
	return l
}
