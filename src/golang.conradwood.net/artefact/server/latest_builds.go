package main

import (
	"context"

	pb "golang.conradwood.net/apis/artefact"
	"golang.conradwood.net/apis/common"
	"golang.conradwood.net/apis/gitserver"
	"golang.conradwood.net/go-easyops/authremote"
)

func (a *artefactServer) LatestBuildForGoEasyops(ctx context.Context, req *common.Void) (*pb.LatestBuild, error) {
	return get_latest_build(ctx, 59) // go-easyops is maintained in git and there it is ID 59
}
func get_latest_build(ctx context.Context, repoid uint64) (*pb.LatestBuild, error) {
	ctx = authremote.Context()
	gr := &gitserver.ByIDRequest{ID: repoid}
	lb, err := gitserver.GetGIT2Client().GetLatestSuccessfulBuild(ctx, gr)
	if err != nil {
		return nil, err
	}
	res := &pb.LatestBuild{BuildID: lb.ID, UnixTimestamp: lb.Timestamp}
	return res, nil
}
