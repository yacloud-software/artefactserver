package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "golang.conradwood.net/apis/artefact"
	"golang.conradwood.net/artefact/db"
	"golang.conradwood.net/go-easyops/errors"
)

func (a *artefactServer) CreateArtefactIfRequired(ctx context.Context, req *pb.CreateArtefactRequest) (*pb.CreateArtefactResponse, error) {
	if req.ArtefactName == "" {
		return nil, errors.InvalidArgs(ctx, "artefactname required", "artefactname required")
	}
	if req.BuildRepoDomain == "" {
		return nil, errors.InvalidArgs(ctx, "buildrepodomain required", "buildrepodomain required")
	}
	if req.OrganisationID == "" {
		return nil, errors.InvalidArgs(ctx, "organisationid required", "organisationid required")
	}
	prefix := fmt.Sprintf("[\"%s\", url=\"%s\", domain=\"%s\"] ", req.ArtefactName, req.GitURL, req.BuildRepoDomain)
	fmt.Print(prefix + "Request to create (if required)\n")
	if strings.Contains(req.GitURL, "git.singingcat.net") && strings.Contains(req.BuildRepoDomain, "conradwood") {
		fmt.Printf("******** THIS LOOKS WRONG. DOMAIN CONRADWOOD.NET FOR SINGINGCAT.NET?? DENIED (artefactserver).\n")
		return nil, errors.InvalidArgs(ctx, "invalid domain/url combo", "invalid domain/url combo")
	}
	afs, err := db.DefaultDBArtefactID().ByName(ctx, req.ArtefactName)
	if err != nil {
		return nil, err
	}
	var myaf *pb.ArtefactID
	for _, af := range afs {
		if af.Domain != req.BuildRepoDomain {
			continue
		}
		myaf = af
		break
	}

	if myaf != nil {
		fmt.Printf(prefix+" exists already (id=%d)\n", myaf.ID)
		// if new create request has a new url, update it
		if req.GitURL != "" && myaf.URL != req.GitURL {
			myaf.URL = req.GitURL
			err = db.DefaultDBArtefactID().Update(ctx, myaf)
			if err != nil {
				return nil, err
			}
		}
		am, err := create_artefact_meta(ctx, myaf)
		if err != nil {
			return nil, err
		}
		res := &pb.CreateArtefactResponse{
			Created: false,
			Meta:    am,
		}
		return res, nil
	}
	myaf = &pb.ArtefactID{
		Domain:  req.BuildRepoDomain,
		Name:    req.ArtefactName,
		URL:     req.GitURL,
		Created: uint32(time.Now().Unix()),
	}
	_, err = db.DefaultDBArtefactID().Save(ctx, myaf)
	if err != nil {
		return nil, err
	}
	am, err := create_artefact_meta(ctx, myaf)
	if err != nil {
		return nil, err
	}
	fmt.Printf(prefix+" created (id=%d)\n", am.ID)
	res := &pb.CreateArtefactResponse{
		Created: true,
		Meta:    am,
	}
	return res, nil
}
func create_artefact_meta(ctx context.Context, af *pb.ArtefactID) (*pb.ArtefactMeta, error) {
	var lb *pb.LatestBuild
	afs := &artefactServer{}
	repoid := uint64(0)
	ridp, err := afs.GetRepoForArtefact(ctx, &pb.ID{ID: af.ID})
	if err != nil {
		fmt.Printf("Got no repo for artefact id #%d\n", af.ID)
		ridp = &pb.ID{}
	} else {
		repoid = ridp.ID
		lb, err = get_latest_build(ctx, repoid)
		if err != nil {
			fmt.Printf("Got no latest build for artefact: %s\n", errors.ErrorString(err))
		}
	}
	am := &pb.ArtefactMeta{
		ID:           af.ID,
		RepositoryID: ridp.ID,
		LatestBuild:  lb,
	}
	return am, nil
}
