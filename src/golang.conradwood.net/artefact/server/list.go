package main

import (
	"context"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/apis/common"
	"golang.conradwood.net/artefact/buildrepo"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/errors"
	"sort"
	"sync"
)

// this lists all the repositories and their current versions
func (e *artefactServer) List2(ctx context.Context, req *common.Void) (*pb.ArtefactList, error) {
	fmt.Printf("listing all artefacts...\n")
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
	fmt.Printf("Getting data for reports..\n")
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
			createArtefactLink(af)
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


