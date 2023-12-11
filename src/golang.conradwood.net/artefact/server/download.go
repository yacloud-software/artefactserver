package main

import (
    "golang.conradwood.net/go-easyops/authremote"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	h2g "golang.conradwood.net/apis/h2gproxy"
	"strings"
	//	"golang.conradwood.net/go-easyops/utils"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/errors"
	"golang.conradwood.net/go-easyops/rpc"
	"golang.conradwood.net/go-easyops/utils"
	"io"
)

func (e *artefactServer) StreamHTTP(req *h2g.StreamRequest, srv pb.ArtefactService_StreamHTTPServer) error {
	ctx := srv.Context()
	if auth.GetUser(ctx) == nil {
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
	ctx = authremote.Context()
	r := ""
	for _, x := range req.Parameters {
		if x.Name == "ref" {
			r = x.Value
			break
		}
	}
	if r == "" {
		return e.download_v2(req, srv)
	}
	fmt.Printf("Downloading. Parsing reference \"%s\"...\n", r)
	ref, err := parseReference(ctx, r)
	if err != nil {
		fmt.Printf("Unable to parse download reference: %s\n", utils.ErrorString(err))
		return err
	}
	fmt.Printf("Downloading: %s\n", ref.String())
	ctx = srv.Context()
	_, err = requestAccess(ctx, ref.Repository(), ref.domain)
	if err != nil {
		fmt.Printf("Access error: %s\n", utils.ErrorString(err))
		return err
	}

	ctx = authremote.Context() // stream stuff still doesn't work quite right

	fname := fmt.Sprintf("%s/%s", ref.path, ref.name)
	fmt.Printf("Downloading (%s:%s) from \"%s\"...\n", ref.Repository(), fname, ref.buildrepo)
	b := brepo.GetBuildRepoManagerClient(ref.Repository(), ref.domain)
	file := &br.File{
		Repository: ref.Repository(),
		Branch:     ref.Branch(),
		BuildID:    ref.Version(),
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

func (e *artefactServer) GetFile(req *pb.Reference, srv pb.ArtefactService_GetFileServer) error {
	ctx := srv.Context()
	if auth.GetUser(ctx) == nil {
		return errors.Unauthenticated(ctx, "access denied to streamhttp/download build repo file")
	}
	r := req.Reference
	if r == "" {
		return errors.InvalidArgs(ctx, "missing reference", "no reference to download")
	}
	fmt.Printf("Downloading. Parsing reference \"%s\"...\n", r)
	ref, err := parseReference(ctx, r)
	if err != nil {
		fmt.Printf("Unable to parse download reference: %s\n", utils.ErrorString(err))
		return err
	}

	ctx = srv.Context()
	_, err = requestAccess(ctx, ref.Repository(), ref.domain)
	if err != nil {
		fmt.Printf("Access error: %s\n", utils.ErrorString(err))
		return err
	}

	ctx = authremote.Context() // stream stuff still doesn't work quite right

	fname := fmt.Sprintf("%s/%s", ref.path, ref.name)
	fmt.Printf("Downloading (%s:%s)...\n", ref.Repository(), fname)
	b := brepo.GetBuildRepoManagerClient(ref.Repository(), ref.domain)
	if b == nil {
		return errors.NotFound(ctx, "no buildrepo for domain \"%s\"", ref.domain)
	}
	file := &br.File{
		Repository: ref.Repository(),
		Branch:     ref.Branch(),
		BuildID:    ref.Version(),
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
		MimeType: "binary/octet-stream",
	}})
	if err != nil {
		return err
	}
	bc, err := b.GetFileAsStream(ctx, &br.GetFileRequest{File: file, Blocksize: 8192})
	if err != nil {
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
func getmimetype(filename string) string {
	if strings.HasSuffix(filename, ".html") {
		return "text/html"
	}
	if strings.HasSuffix(filename, ".css") {
		return "text/css"
	}
	if strings.HasSuffix(filename, ".js") {
		return "application/javascript"
	}
	return "binary/octet-stream"
}


