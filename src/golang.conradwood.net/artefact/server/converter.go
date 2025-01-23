package main

import (
	"context"
	"fmt"
	"time"

	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/apis/gitserver"
	"golang.conradwood.net/artefact/db"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/cache"
	"golang.conradwood.net/go-easyops/errors"
	"golang.conradwood.net/go-easyops/utils"
)

var (
	repo_artefact_cache = cache.New("repo_to_artefact", time.Duration(30)*time.Minute, 5000)
	artefact_repo_cache = cache.New("artefact_to_repo", time.Duration(30)*time.Minute, 5000)
)

type repo_artefact_cache_entry struct {
	response *pb.ID
}

func (e *artefactServer) GetArtefactIDForRepo(ctx context.Context, id *pb.ID) (*pb.ArtefactID, error) {
	id, err := e.GetArtefactForRepo(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.GetArtefactByID(ctx, id)
}

// return the artefactid from a repoid
func (e *artefactServer) GetArtefactForRepo(ctx context.Context, id *pb.ID) (*pb.ID, error) {
	rafid, err := try_resolve_repoid_by_url(ctx, id)
	if err == nil {
		return rafid, nil
	}
	debugf("getting artefact for repo %d\n", id.ID)
	repos, err := brepo.ListRepos(ctx)
	if err != nil {
		debugf("error listing repos: %s", utils.ErrorString(err))
		return nil, err
	}
	debugf("Found %d repos\n", len(repos.Entries))
	afid := uint64(0)
	for _, r := range repos.Entries {
		if *debug {
			fmt.Printf("Checking %s against %d\n", r.Name, id.ID)
		}
		glv, err := brepo.GetLatestVersion(ctx, r.Domain, &br.GetLatestVersionRequest{
			Repository: r.Name,
			Branch:     "master",
		})
		if err != nil {
			return nil, err
		}
		if glv.BuildMeta != nil && glv.BuildMeta.RepositoryID == id.ID {
			//			artefact_repo_cache.Put(fmt.Sprintf("%d",id),
			fmt.Printf("Name: %s, Domain: %s\n", r.Name, r.Domain)
			afid, err = artefactToID(r.Name, r.Domain)
			if err != nil {
				return nil, err
			}
		}
	}
	if afid == 0 {
		return nil, errors.NotFound(ctx, "no artefact for repo")
	}
	return &pb.ID{ID: afid}, nil
}
func (e *artefactServer) GetRepoForArtefact(ctx context.Context, id *pb.ID) (*pb.ID, error) {
	key := fmt.Sprintf("%d", id.ID)
	ro := repo_artefact_cache.Get(key)
	if ro != nil {
		return (ro.(*repo_artefact_cache_entry)).response, nil
	}
	svc := auth.GetService(ctx)
	if svc != nil {
		fmt.Printf("GetRepoForArtefact(%d) Called by service #%s \"%s\"\n", id.ID, svc.ID, svc.Email)
	}
	// cannot ask for permissions here because I am being called by objectauth!
	af, err := idstore.ByID(ctx, id.ID)
	if err != nil {
		return nil, err
	}
	rmi, err := brepo.GetRepositoryMeta(ctx, af.Domain, &br.GetRepoMetaRequest{Path: af.Name})
	if err == nil {
		res := &pb.ID{ID: rmi.RepositoryID}
		repo_artefact_cache.Put(key, &repo_artefact_cache_entry{response: res})
		return res, nil
	}
	glv, err := brepo.GetLatestVersion(ctx, af.Domain, &br.GetLatestVersionRequest{
		Repository: af.Name,
		Branch:     "master",
	})
	if err != nil {
		return nil, err
	}
	if glv.BuildMeta != nil {
		res := &pb.ID{ID: glv.BuildMeta.RepositoryID}
		repo_artefact_cache.Put(key, &repo_artefact_cache_entry{response: res})
		return res, nil
	}
	return nil, errors.Unavailable(ctx, "buildrepo information unavailable")
}

func try_resolve_repoid_by_url(ctx context.Context, id *pb.ID) (*pb.ID, error) {
	git_repo, err := gitserver.GetGIT2Client().RepoByID(ctx, &gitserver.ByIDRequest{ID: id.ID})
	if err != nil {
		return nil, err
	}
	var afid *pb.ArtefactID
	for _, url := range git_repo.URLs {
		u := "https://" + url.Host + "/git/" + url.Path
		fmt.Printf("GitRepo URL: %s\n", u)
		q := db.NewQuery()
		q.AddEqual("url", u)
		afids, err := db.DefaultDBArtefactID().ByDBQuery(ctx, q)
		if err != nil {
			return nil, err
		}
		for _, af := range afids {
			host := "git." + af.Domain
			if host != url.Host {
				continue
			}
			fmt.Printf("Possible Match: #%d %s %s %s\n", af.ID, af.Domain, af.Name, af.URL)
			if afid != nil && afid.ID != af.ID {
				return nil, errors.Errorf("Multiple matches")
			}
			afid = af
		}
	}
	if afid != nil {
		fmt.Printf("Using new style lookup to return artefactid #%d for repo #%d\n", id.ID, afid.ID)
		return &pb.ID{ID: afid.ID}, nil
	}
	return nil, errors.Errorf("cannot resolve id yet")
}
