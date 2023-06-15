package main

import (
	"context"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	br "golang.conradwood.net/apis/buildrepo"
	"strconv"
)

func (e *artefactServer) GetArtefactByID(ctx context.Context, req *pb.ID) (*pb.ArtefactID, error) {
	af, err := idstore.ByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return af, nil
}

func (e *artefactServer) GetArtefactBuilds(ctx context.Context, req *pb.ArtefactID) (*pb.BuildList, error) {
	af, err := idstore.ByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	_, xerr := requestAccess(ctx, af.Name, af.Domain)
	if xerr != nil {
		return nil, xerr
	}
	lvr, err := brepo.ListVersions(ctx, af.Domain, &br.ListVersionsRequest{
		Repository: af.Name,
		Branch:     "master",
	})
	if err != nil {
		return nil, err
	}
	bl := &pb.BuildList{}
	for _, v := range lvr.Entries {
		if v.Name == "latest" {
			continue
		}
		b, err := strconv.ParseUint(v.Name, 10, 64)
		if err != nil {
			return nil, err
		}
		bl.Builds = append(bl.Builds, b)
	}
	return bl, nil
}
func (e *artefactServer) GetDirListing(ctx context.Context, req *pb.DirListRequest) (*pb.DirListing, error) {
	af, err := idstore.ByID(ctx, req.ArtefactID)
	if err != nil {
		return nil, err
	}
	if *debug {
		fmt.Printf("Getting Dir \"%s\" for artefact #%d (%s)\n", req.Dir, req.ArtefactID, af.Name)
	}
	_, xerr := requestAccess(ctx, af.Name, af.Domain)
	if xerr != nil {
		return nil, xerr
	}
	lfr, _, err := brepo.ListFiles(ctx, af.Domain, &br.ListFilesRequest{
		Repository: af.Name,
		Branch:     "master",
		BuildID:    req.Build,
		Recursive:  false,
		Dir:        req.Dir,
	})
	if err != nil {
		return nil, err
	}
	res := &pb.DirListing{
		Path: req.Dir,
		ArtefactInfo: &pb.ArtefactInfo{
			ID:   af.ID,
			Name: af.Name,
		},
	}
	if *debug {
		fmt.Printf("Dir \"%s\" in artefact #%d (%s) got %d entries\n", res.Path, res.ArtefactInfo.ID, res.ArtefactInfo.Name, len(lfr.Entries))
	}
	dir := req.Dir
	for _, e := range lfr.Entries {
		if e.Dir != dir {
			if *debug {
				fmt.Printf("Entry \"%s\" does not match dir \"%s\" (%s)\n", e.Name, dir, e.Dir)
			}
			continue
		}
		if e.Type == 2 {
			res.Dirs = append(res.Dirs, &pb.DirInfo{RelativeDir: e.Dir, Name: e.Name})
		} else if e.Type == 1 {
			res.Files = append(res.Files, &pb.FileInfo{RelativeDir: e.Dir, Name: e.Name})
		} else {
			fmt.Printf("Weird Entry: %#v\n", e)
		}
	}

	return res, nil
}
func (e *artefactServer) GetFileStream(req *pb.FileRequest, srv pb.ArtefactService_GetFileStreamServer) error {
	ctx := srv.Context()
	af, err := idstore.ByID(ctx, req.ArtefactID)
	if err != nil {
		return err
	}
	_, xerr := requestAccess(ctx, af.Name, af.Domain)
	if xerr != nil {
		return xerr
	}
	blvr := &br.GetFileRequest{
		File: &br.File{
			Repository: af.Name,
			Branch:     "master",
			BuildID:    req.Build,
			Filename:   req.Filename,
		},
		Blocksize: 4096,
	}
	err = brepo.GetFile(ctx, af.Domain, blvr, &serverwriter{srv: srv})
	if err != nil {
		return err
	}
	return nil
}
func (e *artefactServer) DoesFileExist(ctx context.Context, req *pb.FileRequest) (*pb.FileExistsInfo, error) {
	af, err := idstore.ByID(ctx, req.ArtefactID)
	if err != nil {
		return nil, err
	}
	_, xerr := requestAccess(ctx, af.Name, af.Domain)
	if xerr != nil {
		return nil, xerr
	}
	blvr := &br.GetFileRequest{
		File: &br.File{
			Repository: af.Name,
			Branch:     "master",
			BuildID:    req.Build,
			Filename:   req.Filename,
		},
		Blocksize: 4096,
	}
	fei, err := brepo.DoesFileExist(ctx, af.Domain, blvr)
	if err != nil {
		return nil, err
	}
	res := &pb.FileExistsInfo{
		Exists: fei.Exists,
		Size:   fei.Size,
	}
	return res, nil
}

type serverwriter struct {
	srv pb.ArtefactService_GetFileStreamServer
}

func (sw *serverwriter) Write(buf []byte) (int, error) {
	fsr := &pb.FileStreamResponse{Payload: buf}
	err := sw.srv.Send(fsr)
	if err != nil {
		return 0, err
	}
	return len(buf), nil
}
