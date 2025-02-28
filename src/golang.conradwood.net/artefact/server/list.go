package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/apis/common"
	"golang.conradwood.net/artefact/buildrepo"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/errors"
)

type timing struct {
	sync.Mutex
	access time.Duration
	latest time.Duration
}

// this lists all the repositories and their current versions
func (e *artefactServer) List2(ctx context.Context, req *common.Void) (*pb.ArtefactList, error) {
	fmt.Printf("listing all artefacts...\n")
	u := auth.GetUser(ctx)
	if u == nil {
		return nil, errors.Unauthenticated(ctx, "no user for List()")
	}

	var err error
	resp := &pb.ArtefactList{}
	started := time.Now()
	repos, err := brepo.ListRepos(ctx)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Time to listrepos: %0.1fs\n", time.Since(started).Seconds())
	started = time.Now()
	fmt.Printf("Getting data for reports from %d entries..\n", len(repos.Entries))
	adminAccess := auth.IsRoot(ctx)
	tim := &timing{}
	var wg sync.WaitGroup
	for _, entry := range repos.Entries {
		wg.Add(1)
		go func(e *buildrepo.RepoEntry) {
			defer wg.Done()
			timer := time.Now()
			rid, xerr := requestAccess(ctx, e.Name, e.Domain)
			if xerr != nil {
				return
			}
			tim.AddAccess(time.Since(timer))

			timer = time.Now()

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
			tim.AddLatest(time.Since(timer))

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
	fmt.Printf("Time to get versions and access: %0.1fs (%s)\n", time.Since(started).Seconds(), tim.String())
	sort.Slice(resp.Artefacts, func(i, j int) bool {
		return resp.Artefacts[i].Name < resp.Artefacts[j].Name
	})
	return resp, nil
}

func (t *timing) AddAccess(dur time.Duration) {
	t.Lock()
	defer t.Unlock()
	t.access = t.access + dur
}
func (t *timing) AddLatest(dur time.Duration) {
	t.Lock()
	defer t.Unlock()
	t.latest = t.latest + dur
}
func (t *timing) String() string {
	return fmt.Sprintf("access=%0.1fs, latest=%0.1fs", t.access.Seconds(), t.latest.Seconds())
}
