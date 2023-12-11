package main

import (
	"context"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"path/filepath"
	"strings"
)

func (e *artefactServer) GetContents2(ctx context.Context, req *pb.Reference) (*pb.Contents, error) {
	fmt.Printf("Get Contents: Reference: \"%#v\"\n", req)
	lr, err := ParseLinkReference(ctx, req.Reference)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Parsed Reference: %s\n", lr.String())
	err = requestAccessLinkReference(ctx, lr)
	if err != nil {
		return nil, err
	}

	res := &pb.Contents{
		Name:    lr.GetArtefact().Name,
		Version: lr.ResolvedVersion(ctx),
		Path:    lr.Path(),
	}

	lfr, t, err := brepo.ListFiles(ctx, lr.Domain(), &br.ListFilesRequest{
		Repository: lr.ArtefactName(),
		Branch:     "master",
		BuildID:    res.Version,
		Recursive:  false,
		Dir:        lr.Path(),
	})
	if err != nil {
		return nil, err
	}
	fmt.Printf("Buildrepo: %s has %d entries\n", t, len(lfr.Entries))
	// now create contents protos for all files
	res.Entries, err = filesToEntries(ctx, lr, lfr.Entries)
	if err != nil {
		return nil, err
	}
	sortEntries(res)
	return res, nil
}

func filesToEntries(ctx context.Context, lr *LinkReference, files []*br.RepoEntry) ([]*pb.Contents, error) {
	var res []*pb.Contents
	v := lr.ResolvedVersion(ctx)
	aa := false

	for _, entry := range files {
		if !isInDir(lr.Path(), entry.Dir+"/"+entry.Name) {

			continue
		}

		c := &pb.Contents{
			ReferenceVersion: "",
			ReferenceLatest:  "",
			Name:             entry.Name,
			Version:          v,
			AdminAccess:      aa,
			Artefact:         &pb.ArtefactRef{},
			Path:             entry.Dir,
			Domain:           lr.Domain(),
			BuildRepo:        lr.Domain(),
			RepositoryID:     lr.RepositoryID(),
			ArtefactID:       lr.GetArtefact(),
		}
		if entry.Type == 1 {
			c.Type = pb.ContentType_File
			c.Downloadable = true
			createFileLink(c)
		} else if entry.Type == 2 {
			createDirLink(c)
			c.Type = pb.ContentType_Directory
		}
		res = append(res, c)
	}

	return res, nil
}

// check if path is in refdir as a direct child (assumes absolute files)
// example:
// TRUE: ("/dist/firmware", "/dist")
// TRUE: ("dist/firmware", "/dist")
// FALSE: ("/dist/firmware/foo", "/dist")
func isInDir(refdir string, path string) bool {
	if !strings.HasPrefix(refdir, "/") {
		refdir = "/" + refdir
	}
	p := filepath.Dir(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if refdir == p {
		return true
	}
	//fmt.Printf("Skipped \"%s\", because \"%s\",not equal to \"%s\"\n", path, p, refdir)
	return false
}


