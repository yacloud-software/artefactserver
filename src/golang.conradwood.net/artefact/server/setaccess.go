package main

import (
	"context"
	pb "golang.conradwood.net/apis/artefact"
	"golang.conradwood.net/apis/common"
)

func (e *artefactServer) SetAccess(ctx context.Context, req *pb.SetAccessRequest) (*common.Void, error) {
	return &common.Void{}, nil
}
