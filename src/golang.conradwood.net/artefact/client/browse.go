package main

import (
	"fmt"
	"golang.conradwood.net/apis/artefact"
	"golang.conradwood.net/go-easyops/authremote"
	"golang.conradwood.net/go-easyops/utils"
	"os"
)

func Browse() {
	if *artefactid == 0 {
		fmt.Printf("please specify artefactid\n")
		os.Exit(10)
	}
	if *browsebuild == 0 {
		browseBuilds()
		return
	}
	ar := artefact.GetArtefactServiceClient()
	ctx := authremote.Context()
	dl, err := ar.GetDirListing(ctx, &artefact.DirListRequest{
		Build:      uint64(*browsebuild),
		Dir:        *browsedir,
		ArtefactID: uint64(*artefactid),
	})
	utils.Bail("failed to get dir listing", err)
	for _, f := range dl.Files {
		fmt.Printf("[FILE] %s\n", f.Name)
	}
	for _, f := range dl.Dirs {
		fmt.Printf("[DIR ] %s\n", f.Name)
	}
}

func browseBuilds() {
	ar := artefact.GetArtefactServiceClient()
	ctx := authremote.Context()
	bl, err := ar.GetArtefactBuilds(ctx, &artefact.ArtefactID{ID: uint64(*artefactid)})
	utils.Bail("failed to get builds", err)
	for _, b := range bl.Builds {
		fmt.Printf("Build #%d\n", b)
	}

}
