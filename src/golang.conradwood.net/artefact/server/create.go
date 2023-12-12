package main

import (
	"context"
	"fmt"
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
	fmt.Printf("Request to create artefact \"%s\" on domain \"%s\"\n", req.ArtefactName, req.BuildRepoDomain)
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
		Domain: req.BuildRepoDomain,
		Name:   req.ArtefactName,
		URL:    req.GitURL,
	}
	_, err = db.DefaultDBArtefactID().Save(ctx, myaf)
	if err != nil {
		return nil, err
	}
	am, err := create_artefact_meta(ctx, myaf)
	if err != nil {
		return nil, err
	}
	res := &pb.CreateArtefactResponse{
		Created: true,
		Meta:    am,
	}
	return res, nil
}
func create_artefact_meta(ctx context.Context, af *pb.ArtefactID) (*pb.ArtefactMeta, error) {
	am := &pb.ArtefactMeta{ID: af.ID}
	return am, nil
}



