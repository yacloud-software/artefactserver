package main

import (
	"context"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/go-easyops/errors"
	"golang.conradwood.net/go-easyops/utils"
)

type reference struct {
	name            string // e.g. file or directory name (full path), empty if artefact
	Type            pb.ContentType
	repository      string
	path            string
	branch          string
	version         uint64 // 0 == latest
	resolvedVersion uint64 // either == version or latest
	domain          string
	buildrepo       string
}

// serialiser helpers
func toReference(r *pb.SerialReference) (string, string, error) {
	s1, err := utils.Marshal(r)
	if err != nil {
		return "", "", err
	}
	r.Version = 0
	s2, err := utils.Marshal(r)
	if err != nil {
		return "", "", err
	}
	return s1, s2, nil

}

// given a content proto this will create a natural key for it which can serve as a reference in urls
func createArtefactReference(a *pb.Contents) {
	if a.Domain == "" {
		s := fmt.Sprintf("Unable to create reference without domain for %s", a.Name)
		panic(s)
	}
	if a.BuildRepo == "" {
		s := fmt.Sprintf("Unable to create reference without buildrepo for %s", a.Name)
		panic(s)
	}
	var err error
	res := &pb.SerialReference{BuildRepo: a.BuildRepo, RefType: uint32(a.Type), Version: a.Version, Domain: a.Domain}
	if a.Type == pb.ContentType_Artefact {
		res.Texts = []string{a.Name}
		a.ReferenceVersion, a.ReferenceLatest, err = toReference(res)
	} else if a.Type == pb.ContentType_Directory {
		res.Texts = []string{a.Name, a.Artefact.Name, a.Path}
		a.ReferenceVersion, a.ReferenceLatest, err = toReference(res)
	} else if a.Type == pb.ContentType_File {
		res.Texts = []string{a.Name, a.Artefact.Name, a.Path}
		a.ReferenceVersion, a.ReferenceLatest, err = toReference(res)
	} else {
		fmt.Printf("creating-reference: Invalid type: %v\n", a.Type)
		a.ReferenceVersion = fmt.Sprintf("NONE_%v", a.Type)
		a.ReferenceLatest = fmt.Sprintf("NONE_%v", a.Type)
	}
	if err != nil {
		fmt.Printf("Failed to convert reference :%s\n", err)
	}
}

// this turns a url reference into a reference object
func parseReference(ctx context.Context, ref string) (*reference, error) {
	if ref == "" {
		return nil, errors.InvalidArgs(ctx, "reference missing but required", "reference missing but required")
	}
	sr := &pb.SerialReference{}
	err := utils.Unmarshal(ref, sr)
	if err != nil {
		return nil, err
	}
	etype := pb.ContentType(sr.RefType)
	res := &reference{branch: "master", Type: etype, domain: sr.Domain, buildrepo: sr.BuildRepo}
	res.version = sr.Version
	if etype == pb.ContentType_Artefact {
		if len(sr.Texts) != 1 {
			return nil, errors.InvalidArgs(ctx, "invalid reference (1)", "invalid reference (%s has %d parts not 1)", ref, len(sr.Texts))
		}
		res.repository = sr.Texts[0]
	} else if etype == pb.ContentType_Directory || etype == pb.ContentType_File {
		if len(sr.Texts) != 3 {
			return nil, errors.InvalidArgs(ctx, "invalid reference (2)", "invalid reference (%s has %d parts not 2)", ref, len(sr.Texts))
		}
		res.name = sr.Texts[0]
		res.repository = sr.Texts[1]
		res.path = sr.Texts[2]
	} else {
		return nil, errors.InvalidArgs(ctx, "invalid reference (3)", "invalid reference for type %v", res.Type)
	}

	if res.domain == "" {
		return nil, errors.InvalidArgs(ctx, "reference has no domain", "reference for %s is missing a domain", res.repository)
	}

	// get latest
	if res.version != 0 {
		res.resolvedVersion = res.version
	} else {
		b := brepo.GetBuildRepoManagerClient(res.repository, res.domain)
		glv, err := b.GetLatestVersion(ctx, &br.GetLatestVersionRequest{
			Repository: res.Repository(),
			Branch:     "master",
		})
		if err != nil {
			return nil, errors.InvalidArgs(ctx, "invalid reference (4)", "reference, rejected by buildrepo (%s) (ref: %s)", err, res.String())
		}
		res.resolvedVersion = glv.BuildID

	}
	return res, nil
}

func (r *reference) String() string {
	return fmt.Sprintf("Repository=%s, Type=%d on \"%s\"", r.Repository(), r.Type, r.buildrepo)
}

// true it references an artefact (rather than a dir or file)
func (r *reference) IsArtefact() bool {
	return r.Type == pb.ContentType_Artefact
}
func (r *reference) IsDirectory() bool {
	return r.Type == pb.ContentType_Directory
}

// return true if this reference "latest" (rather than a specific version)
func (r *reference) IsLatest() bool {
	return r.version == 0
}

func (r *reference) Repository() string {
	return r.repository
}
func (r *reference) Branch() string {
	return r.branch
}
func (r *reference) Version() uint64 {
	return r.resolvedVersion
}



