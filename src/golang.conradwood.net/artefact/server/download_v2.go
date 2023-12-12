package main

import (
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	h2g "golang.conradwood.net/apis/h2gproxy"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/errors"
	"golang.conradwood.net/go-easyops/rpc"
	//	"golang.conradwood.net/go-easyops/tokens"
	"golang.conradwood.net/go-easyops/utils"
	"io"
)

func (e *artefactServer) download_v2(req *h2g.StreamRequest, srv pb.ArtefactService_StreamHTTPServer) error {
	fmt.Printf("Downloading V2 style:\"%s\"\n", req.Path)
	ctx := srv.Context()
	user := auth.GetUser(ctx)
	if user == nil {
		cs := rpc.CallStateFromContext(ctx)
		if cs == nil {
			fmt.Printf("No callstate\n")
		} else {
			cs.Debug = true
			cs.PrintContext()
		}
		fmt.Printf("Streamhttp called without user\n")
		return errors.Unauthenticated(ctx, "access denied to streamhttp/download build repo file")
	}
	lr, err := ParseLinkReference(ctx, req.Path)
	if err != nil {
		fmt.Printf("invalid link reference: %s\n", err)
		return err
	}
	err = requestAccessLinkReference(ctx, lr)
	if err != nil {
		fmt.Printf("User #%s (%s) does not have access to artefact %s\n", user.ID, user.Email, lr.String())
		return err
	}
	fmt.Printf("Downloading: %s\n", lr.String())
	fname := fmt.Sprintf("%s", lr.Path())
	fmt.Printf("Downloading (%s:%s) from \"%s\"...\n", lr.ArtefactName(), fname, lr.Domain())
	b := brepo.GetBuildRepoManagerClient(lr.ArtefactName(), lr.Domain())
	file := &br.File{
		Repository: lr.ArtefactName(),
		Branch:     lr.Branch(),
		BuildID:    lr.ResolvedVersion(ctx),
		Filename:   fname,
	}

	glv, err := b.GetFileMetaData(ctx, &br.GetMetaRequest{File: file})
	if err != nil {
		fmt.Printf("Unable to get size of file: %s\n", utils.ErrorString(err))
		return err
	}
	fsize := glv.Size
	fmt.Printf("Filesize: %d\n", fsize)

	err = srv.Send(&h2g.StreamDataResponse{Response: &h2g.StreamResponse{
		Filename: fname,
		Size:     fsize,
		MimeType: getmimetype(fname),
	}})
	if err != nil {
		return err
	}
	bc, err := b.GetFileAsStream(ctx, &br.GetFileRequest{File: file, Blocksize: 8192})
	if err != nil {
		fmt.Printf("no file as stream: %s\n", err)
		return err
	}
	for {
		fb, err := bc.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		err = srv.Send(&h2g.StreamDataResponse{Data: fb.Data[:fb.Size]})
		if err != nil {
			return err
		}
	}

	return nil
}



