package resources

import (
	"net/url"
	"path/filepath"

	"github.com/pkg/errors"
)

// Lookup return resources to be used for installing a cluster API component
// in the management cluster
func Lookup(component string, repositories map[string]string, githubToken string) (*ComponentResources, error) {
	// gets the repository rawUrl for the component, using the user provided repositories first
	// or the default repositories locations as a fallback
	rRawUrl, ok := repositories[component]
	if !ok {
		rRawUrl, ok = defaultRepositories[component]
		if !ok {
			return nil, errors.Errorf("missing repository for %q", component)
		}
	}

	// parse the repository url
	rUrl, err := url.Parse(rRawUrl)
	if err != nil {
		//TODO: provide more detailed help or a link
		return nil, errors.Errorf("repository for %q is not a valid url", component)
	}

	// if the url is a github repository, reads resources from this location
	if rUrl.Scheme == "https" && rUrl.Host == "github.com" {
		resources, err := lookupGitHubRepository(rUrl.Path, githubToken)
		if err != nil {
			return nil, err
		}
		return &ComponentResources{
			resources: resources,
			apply:     rUrl.Fragment,
		}, nil
	}

	// if the url is a http/https repository, reads resources from this location
	if rUrl.Scheme == "http" || rUrl.Scheme == "https" {
		//TODO: support for http/https repositories
		return nil, errors.New("support for http/https repositories is not implemented yet")
	}

	// if the url is a local repository, reads resources from this location
	if rUrl.Scheme == "" || rUrl.Scheme == "file" {
		resources, err := lookupLocalRepository(filepath.Join(rUrl.Host, rUrl.Path))
		if err != nil {
			return nil, err
		}
		return &ComponentResources{
			resources: resources,
			apply:     rUrl.Fragment,
		}, nil
	}

	return nil, errors.New("repository %q not supported")
}
