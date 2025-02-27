package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"golang.conradwood.net/apis/objectauth"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/cache"
	"golang.conradwood.net/go-easyops/errors"
)

var (
	perm_cache        = cache.New("perm_cache", time.Duration(120)*time.Second, 1000)
	always_allow_root = flag.Bool("always_allow_root", true, "if true root gets access to every artefact")
)

type perm_cache_entry struct {
	artefactid uint64
	allowed    bool
}

func requestAccessLinkReference(ctx context.Context, lr *LinkReference) error {
	_, err := requestAccess(ctx, lr.ArtefactName(), lr.Domain())
	return err
}

func is_privileged_service(ctx context.Context) bool {
	svc := auth.GetService(ctx)
	if svc == nil {
		return false
	}
	PRIVILEGED_SERVICES := []string{
		auth.GetServiceIDByName("espota.ESPOtaService"),
	}
	for _, sa := range PRIVILEGED_SERVICES {
		if sa == svc.ID {
			return true
		}
	}
	return false
}

// returns artefactid or error
func requestAccess(ctx context.Context, artefactName string, domain string) (uint64, error) {
	if domain == "" {
		return 0, fmt.Errorf("access to %s without domain denied", artefactName)
	}
	svc := auth.GetService(ctx)
	if svc != nil {
		aar := &objectauth.AllAccessRequest{ObjectType: objectauth.OBJECTTYPE_Artefact, ServiceID: svc.ID}
		ar, err := objectauth.GetObjectAuthClient().AllowAllServiceAccess(ctx, aar)
		if err == nil {
			if ar.ReadAccess {
				rid, err := artefactToID(artefactName, domain)
				return rid, err
			}
		}
	}
	if is_privileged_service(ctx) {
		rid, err := artefactToID(artefactName, domain)
		return rid, err
	}

	u := auth.GetUser(ctx)
	if u == nil {
		fmt.Printf("No user\n")
		return 0, errors.Unauthenticated(ctx, "(3) access to artefact %s denied", artefactName)
	}

	if *debug {
		fmt.Printf("Access by %s [%s] for %s in %s\n", auth.Description(u), u.ID, artefactName, domain)
	}
	rid, err := artefactToID(artefactName, domain)
	if err != nil {
		return 0, err
	}
	if *always_allow_root && auth.IsRoot(ctx) {
		return rid, nil
	}

	if svc != nil && svc.ID == auth.GetServiceIDByName("repobuilder.RepoBuilder") {
		return rid, nil
	}
	if *debug {
		fmt.Printf("getting user access right\n")
	}
	key := fmt.Sprintf("%s_%d", u.ID, rid)
	perm_cache_object := perm_cache.Get(key)
	if perm_cache_object != nil {
		pce := perm_cache_object.(*perm_cache_entry)
		if !pce.allowed {
			return 0, errors.AccessDenied(ctx, "(1) access to artefact #%d (%s) denied", rid, artefactName)
		}
		return rid, nil
	}

	oa := &objectauth.AuthRequest{ObjectType: objectauth.OBJECTTYPE_Artefact, ObjectID: rid}
	ar, err := objectauth.GetObjectAuthServiceClient().AskObjectAccess(ctx, oa)
	if err != nil {
		return 0, err
	}
	if *debug {
		fmt.Printf("user access right, view=%v, read=%v\n", ar.Permissions.View, ar.Permissions.Read)
	}

	if ar.Permissions.View && ar.Permissions.Read {
		perm_cache.Put(key, &perm_cache_entry{artefactid: rid, allowed: true})
		return rid, nil
	}
	if *debug {
		fmt.Printf("Access by %s [%s] for %s in %s DENIED (permissions=%v)\n", auth.Description(u), u.ID, artefactName, domain, ar.Permissions)
	}
	perm_cache.Put(key, &perm_cache_entry{artefactid: rid, allowed: false})
	return 0, errors.AccessDenied(ctx, "(2) access to artefact %s (#%d) denied", artefactName, rid)
}
