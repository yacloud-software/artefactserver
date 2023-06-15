package main

import (
	"flag"
	"fmt"
	pb "golang.conradwood.net/apis/artefact"
	"golang.conradwood.net/apis/common"
	oa "golang.conradwood.net/apis/objectauth"
	ar "golang.conradwood.net/go-easyops/authremote"
	"golang.conradwood.net/go-easyops/utils"
	"os"
	//	"strings"
)

var (
	find        = flag.Bool("find", false, "find an artefact by name")
	name        = flag.String("name", "", "artefact name")
	browse      = flag.Bool("browse", false, "if true browse artefact")
	artefactid  = flag.Uint("artefactid", 0, "artefact id")
	repoid      = flag.Uint("repoid", 0, "repository id (resolve to artefact)")
	browsedir   = flag.String("dir", "", "browse this dir")
	browsebuild = flag.Uint("buildid", 0, "build id")
	echoClient  pb.ArtefactServiceClient
)

func main() {
	flag.Parse()
	echoClient = pb.GetArtefactServiceClient()

	if *repoid != 0 {
		ResolveRepoID()
		os.Exit(0)
	}

	// a context with authentication
	ctx := ar.Context()
	if *find {
		dofind()
		os.Exit(0)
	}
	if *browse {
		Browse()
		os.Exit(0)
	}

	response, err := echoClient.List(ctx, &common.Void{})
	utils.Bail("Failed to ping server", err)
	show(response)
}
func show(response *pb.ArtefactList) {
	oac := oa.GetObjectAuthServiceClient()
	fmt.Printf("%d artefacts:\n", len(response.GetArtefacts()))
	t := utils.Table{}
	t.AddHeaders("ArtefactID", "repoid", "version", "name", "domain", "admin", "rights")
	for _, b := range response.GetArtefacts() {
		s := ""
		if b.AdminAccess {
			s = " [admin]"
		}
		bid := uint64(0)
		if b.ArtefactID != nil {
			bid = b.ArtefactID.ID
		}
		t.AddUint64(bid).AddUint64(b.RepositoryID).AddUint64(b.Version).AddString(b.Name)
		t.AddString(b.Domain).AddString(s)
		if bid != 0 {
			ctx := ar.Context()
			ar := &oa.AuthRequest{ObjectType: oa.OBJECTTYPE_Artefact, ObjectID: bid}
			arl, err := oac.GetRights(ctx, ar)
			if err == nil {
				t.AddString(effectiveRights(arl))
			} else {
				t.AddString(fmt.Sprintf("error: %s\n", err))
			}
		}
		t.NewRow()
	}
	fmt.Printf("%s\n", t.ToPrettyString())
	fmt.Printf("Done.\n")
	os.Exit(0)
}
func effectiveRights(a *oa.AccessRightList) string {
	return "rwx"
}
func dofind() {
	ctx := ar.Context()
	l, err := echoClient.Find(ctx, &pb.FindRequest{NameMatch: *name})
	utils.Bail("failed to find", err)
	show(l)
}
func ResolveRepoID() {
	ctx := ar.Context()
	l, err := echoClient.GetArtefactForRepo(ctx, &pb.ID{ID: uint64(*repoid)})
	utils.Bail("failed to get artefact", err)
	fmt.Printf("Artefact ID: %d\n", l.ID)
}
