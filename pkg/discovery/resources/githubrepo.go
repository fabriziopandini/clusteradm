package resources

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog"
)

func lookupGitHubRepository(path, token string) ([]Resource, error) {
	// Checks if the repository path is in the expected format:
	// /{owner}/{path}/{releases|tree}/{latest|version|branch|tag|sha}/{resource-path}[#apply-path]
	t := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(t) < 5 {
		//TODO: provide more detailed help or a link
		return nil, errors.Errorf("repository path %q is not valid. expected 5 parts, found %d", path, len(t))
	}
	if !(t[2] == "releases" || t[2] == "tree") {
		//TODO: provide more detailed help or a link
		return nil, errors.Errorf("repository path %q is not valid", path)
	}

	// Prepare to connect to the github repository
	r := githubRepository{
		owner: t[0],
		repo:  t[1],
	}
	if token != "" {
		r.authenticate(token)
	}

	// handle the case when we are reading resources from the repository release assets
	if t[2] == "releases" {
		// get the selected release
		var release *github.RepositoryRelease
		if t[3] == "latest" {
			release, _ = r.getLatestRelease()
		} else {
			release, _ = r.getReleaseByTag(t[3])
		}
		// download assets from the release
		fmt.Printf("downloading resources from %q release assets in %q github repository...\n", *release.TagName, r.String())
		return r.downloadResourceFromReleaseAssets(release, t[4])
	}

	// otherwise we are reading resources from the repository source tree

	// get the selected sha (if not already specified in the path)
	var sha string
	if len(t[3]) == 40 {
		sha = t[3]
	} else {
		sha, _ = r.getSHA(t[3])
	}
	// download assets from the source tree
	fmt.Printf("downloading resources from tree content in %q github repository (might take few seconds)...\n", r.String())
	return r.downloadResourcesFromTree(sha, t[4])
}

// githubRepository holds references to a github repository where
// resources for deploying cluster API, infrastructure providers
// or bootstrap providers are stored.
type githubRepository struct {
	httpClient *http.Client
	owner      string
	repo       string
}

// String describe a localRepository as a string
func (r *githubRepository) String() string {
	return fmt.Sprintf("%s/%s", r.owner, r.repo)
}

// authenticate sets the auth token for the githubRepository, if defined.
// The auth token is required in order to raise github api rate limit, and this
// might be required when doing multiple discovery request during development iterations
// or when spinning up several management clusters.
func (r *githubRepository) authenticate(token string) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	r.httpClient = oauth2.NewClient(ctx, ts)
}

// Utilities for discovery of resources hosted as a github repository ReleaseAssets

// getLatestRelease return the latest release for a github repository, according to
// semantic version order of the release tag name.
func (r *githubRepository) getLatestRelease() (*github.RepositoryRelease, error) {
	klog.V(3).Infof("Reading latest release")

	ctx := context.Background()
	client := github.NewClient(r.httpClient)

	// get all the releases
	releases, _, err := client.Repositories.ListReleases(ctx, r.owner, r.repo, nil)
	if err != nil {
		return nil, r.handleGithubErr(err, "failed to read releases")
	}

	// search for the latest release according to semantic version order of the release tag name.
	// releases with tag name that are not semantic version number are ignored
	var latestRelease *github.RepositoryRelease
	var latestReleaseVersion *version.Version
	for _, r := range releases {
		r := r // pin

		if r.TagName == nil {
			continue
		}

		sv, err := version.ParseSemantic(fmt.Sprintf("%v", *r.TagName))
		if err != nil {
			continue
		}

		if latestReleaseVersion == nil || latestReleaseVersion.LessThan(sv) {
			latestRelease = r
			latestReleaseVersion = sv
			continue
		}
	}

	if latestRelease == nil {
		return nil, errors.New("failed to get latest release")
	}

	klog.V(3).Infof("> release %q", *latestRelease.TagName)
	return latestRelease, nil
}

// getReleaseByTag return the github repository release with a specific tag name.
func (r *githubRepository) getReleaseByTag(tag string) (*github.RepositoryRelease, error) {
	klog.V(3).Infof("Reading %q release", tag)

	ctx := context.Background()
	client := github.NewClient(r.httpClient)

	release, _, err := client.Repositories.GetReleaseByTag(ctx, r.owner, r.repo, tag)
	if err != nil {
		return nil, r.handleGithubErr(err, "failed to read release %q", tag)
	}

	if release == nil {
		return nil, errors.Errorf("failed to get release %q", tag)
	}

	return release, nil
}

// downloadResourceFromReleaseAssets download a resource from release assets.
func (r *githubRepository) downloadResourceFromReleaseAssets(release *github.RepositoryRelease, assetName string) ([]Resource, error) {
	klog.V(4).Infof("Downloading %q", assetName)

	ctx := context.Background()
	client := github.NewClient(r.httpClient)

	// search for the asset into the release, retriving the asset id
	var assetId *int64
	for _, a := range release.Assets {
		if a.Name != nil && *a.Name == assetName {
			assetId = a.ID
			break
		}
	}
	if assetId == nil {
		return nil, errors.Errorf("the %q release does not contain the asset %q", *release.TagName, assetName)
	}

	// download the asset content
	rc, red, err := client.Repositories.DownloadReleaseAsset(ctx, r.owner, r.repo, *assetId)
	if err != nil {
		return nil, r.handleGithubErr(err, "failed to download asset %q from %q release", *release.TagName, assetName)
	}

	// handle the case when it is returned a ReaderCloser object for a release asset
	if rc != nil {
		defer rc.Close()

		content, err := ioutil.ReadAll(rc)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read downloaded asset %q from %q release", *release.TagName, assetName)
		}

		return []Resource{
			{
				path:    assetName,
				content: content,
			},
		}, nil
	}

	// handle the case when it is returned a redirect link for a release asset
	resp, err := http.Get(red)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download asset %q from %q release from redirect location %q", *release.TagName, assetName, red)
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read downloaded asset %q from %q release", *release.TagName, assetName)
	}
	return []Resource{
		{
			path:    assetName,
			content: content,
		},
	}, nil
}

// Utilities for discovery of resources hosted as a github repository tree files

// getSHA return the SHA1 identifier for a github repository branch or a tag
func (r *githubRepository) getSHA(branchOrTag string) (string, error) {
	klog.V(3).Infof("Reading SHA for %q", branchOrTag)

	ctx := context.Background()
	client := github.NewClient(r.httpClient)

	// Search within branches; if found, return the SHA
	branches, _, err := client.Repositories.ListBranches(ctx, r.owner, r.repo, nil)
	if err != nil {
		return "", r.handleGithubErr(err, "error reading branches")
	}

	for _, b := range branches {
		if b.Name != nil && *b.Name == branchOrTag {
			return *b.Commit.SHA, nil
		}
	}

	// Search within tags; if found, return the SHA
	tags, _, err := client.Repositories.ListTags(ctx, r.owner, r.repo, nil)
	if err != nil {
		return "", r.handleGithubErr(err, "failed to list tags")
	}

	for _, t := range tags {
		if t.Name != nil && *t.Name == branchOrTag {
			return *t.Commit.SHA, nil
		}
	}

	return "", errors.Wrapf(err, "%q does not match any branch or tag", branchOrTag)
}

// downloadResourcesFromTree download resources from a file/folder in a github repository tree.
// If the path is a folder, also sub-folders are read, recursively.
func (r *githubRepository) downloadResourcesFromTree(SHA, path string) ([]Resource, error) {
	klog.V(4).Infof("Downloading %q", path)

	ctx := context.Background()
	client := github.NewClient(r.httpClient)

	// gets the file/folder from the github repository tree
	fileContent, dirContent, _, err := client.Repositories.GetContents(ctx, r.owner, r.repo, path, nil)
	if err != nil {
		return nil, r.handleGithubErr(err, "failed to get content for %q", path)
	}

	// handles the case when the path is a file
	var resources []Resource
	if fileContent != nil {
		if fileContent.Encoding == nil || *fileContent.Encoding != "base64" {
			return nil, errors.Wrapf(err, "invalid encoding for %q", path)
		}

		content, err := b64.StdEncoding.DecodeString(*fileContent.Content)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode %q", path)
		}

		resources = append(resources,
			Resource{
				path:    *fileContent.Path,
				content: content,
			},
		)
		return resources, nil
	}

	// handles the case when the path is a directory reading resources recursively
	for _, item := range dirContent {
		itemResources, err := r.downloadResourcesFromTree(SHA, *item.Path)
		if err != nil {
			return nil, err
		}
		resources = append(resources, itemResources...)
	}

	return resources, nil
}

func (r *githubRepository) handleGithubErr(err error, message string, args ...string) error {
	if _, ok := err.(*github.RateLimitError); ok {
		return errors.New("hitting rate limit for github api. Please get a personal API tokens a assign it to the CLUSTERADM_GITHUB_TOKEN env var")
	}
	return errors.Wrapf(err, message, args)
}
