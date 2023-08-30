package buildrepo

import (
	"context"
	"flag"
	"fmt"
	br "golang.conradwood.net/apis/buildrepo"
	"golang.conradwood.net/go-easyops/client"
	"io"
	"strings"
	"sync"
)

/*
* consolidate multiple repositories
 */
var (
	user_buildrepos    = flag.String("buildrepos", "", "if set, a comma delimited mapping with buildrepos, e.g. domain:foo-host.localdomain,otherdomain:bar-host.localdomain")
	default_buildrepos = map[string]string{
		"conradwood.net": "buildrepo.vpn.conrad.localdomain",
		"singingcat.net": "scbuildrepo.singingcat.localdomain",
	}
	clients = make(map[string]br.BuildRepoManagerClient)
)

func get_build_repo_map() map[string]string {
	if *user_buildrepos == "" {
		return default_buildrepos
	}
	ma := strings.Split(*user_buildrepos, ",")
	res := make(map[string]string)
	for _, m := range ma {
		l := strings.SplitN(m, ":", 2)
		if len(l) != 2 {
			panic(fmt.Sprintf("invalid buildrepo mapping: %s", m))
		}
		res[l[0]] = l[1]
	}
	return nil
}
func CreateBuildrepo() *BuildRepo {
	for _, v := range get_build_repo_map() {
		adr := fmt.Sprintf("%s:5005", v)
		fmt.Printf("Connecting to buildrepo at: %s\n", adr)
		c, err := client.ConnectWithIP(adr)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to buildrepo @ %s: %s", v, err))
		}
		clients[v] = br.NewBuildRepoManagerClient(c)
	}
	res := &BuildRepo{}
	return res
}

type BuildRepo struct {
}
type RepoList struct {
	Entries []*RepoEntry
}
type RepoEntry struct {
	*br.RepoEntry
	Server string
}

// get repos from all build servers
func (b *BuildRepo) ListRepos(ctx context.Context) (*RepoList, error) {
	var wg sync.WaitGroup
	var terr error
	res := &RepoList{}
	var addlock sync.Mutex
	for target, c := range clients {
		wg.Add(1)
		go func(t string, bc br.BuildRepoManagerClient) {
			defer wg.Done()
			lr, err := bc.ListRepos(ctx, &br.ListReposRequest{})
			if err != nil {
				terr = err
				return
			}
			addlock.Lock()
			for _, e := range lr.Entries {
				// compatibility - older buildrepos do not provide domains for entries (yet)
				// if so use a default.
				// TODO: update buildrepo server
				if e.Domain == "" {
					d := GetDefaultDomainForBuildRepo(t)
					fmt.Printf("WARNING: Buildrepo server %s did not provide domain for \"%s\", using \"%s\"\n", t, e.Name, d)
					e.Domain = d
				}
				re := &RepoEntry{e, t}
				res.Entries = append(res.Entries, re)
			}
			addlock.Unlock()
		}(target, c)
	}
	wg.Wait()
	if terr != nil {
		return nil, terr
	}
	return res, nil
}

// returns: files, buildrepo, error
func (b *BuildRepo) ListFiles(ctx context.Context, domain string, blvr *br.ListFilesRequest) (*br.ListFilesResponse, string, error) {
	if domain == "" {
		return nil, "", fmt.Errorf("missing domain for artefact %s", blvr.Repository)
	}
	t := GetDefaultBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, "", fmt.Errorf("no buildrepo server serving %s in domain %s", blvr.Repository, domain)
	}
	lfr, err := c.ListFiles(ctx, blvr)
	return lfr, t, err
}

func (b *BuildRepo) DoesFileExist(ctx context.Context, domain string, blvr *br.GetFileRequest) (*br.FileExistsInfo, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", blvr.File.Repository)
	}
	t := GetDefaultBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("no buildrepo server serving %s in domain %s", blvr.File.Repository, domain)
	}
	res, err := c.DoesFileExist(ctx, blvr)
	return res, err

}

func (b *BuildRepo) GetRepositoryMeta(ctx context.Context, domain string, blvr *br.GetRepoMetaRequest) (*br.RepoMetaInfo, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", blvr.Path)
	}
	t := GetDefaultBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("no buildrepo server serving %s in domain %s", blvr.Path, domain)
	}
	res, err := c.GetRepositoryMeta(ctx, blvr)
	return res, err

}

// write a file to a writer
func (b *BuildRepo) GetFile(ctx context.Context, domain string, blvr *br.GetFileRequest, target io.Writer) error {
	if domain == "" {
		return fmt.Errorf("missing domain for artefact %s", blvr.File.Repository)
	}
	t := GetDefaultBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return fmt.Errorf("no buildrepo server serving %s in domain %s", blvr.File.Repository, domain)
	}
	stream, err := c.GetFileAsStream(ctx, blvr)
	if err != nil {
		return err
	}
	for {
		block, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		_, err = target.Write(block.Data[:block.Size])
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *BuildRepo) GetLatestVersion(ctx context.Context, domain string, glvr *br.GetLatestVersionRequest) (*br.GetLatestVersionResponse, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", glvr.Repository)
	}
	t := GetDefaultBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("no buildrepo server serving %s in domain %s", glvr.Repository, domain)
	}
	v, err := c.GetLatestVersion(ctx, glvr)
	if err != nil {
		return nil, err
	}
	return v, nil
}
func (b *BuildRepo) ListVersions(ctx context.Context, domain string, glvr *br.ListVersionsRequest) (*br.ListVersionsResponse, error) {
	if domain == "" {
		return nil, fmt.Errorf("missing domain for artefact %s", glvr.Repository)
	}
	t := GetDefaultBuildRepoForDomain(domain)
	c := clients[t]
	if c == nil {
		return nil, fmt.Errorf("no buildrepo server serving %s in domain %s", glvr.Repository, domain)
	}
	v, err := c.ListVersions(ctx, glvr)
	if err != nil {
		return nil, err
	}
	return v, nil
}
func GetDefaultBuildRepoForDomain(domain string) string {
	for k, v := range get_build_repo_map() {
		if k == domain {
			return v
		}
	}
	return ""
}

func GetDefaultDomainForBuildRepo(target string) string {
	for k, v := range get_build_repo_map() {
		if v == target {
			return k
		}
	}
	return ""
}

func (b *BuildRepo) GetBuildRepoManagerClient(repo string, domain string) br.BuildRepoManagerClient {
	a := GetDefaultBuildRepoForDomain(domain)
	if a == "" {
		return nil
	}
	return clients[a]
}
