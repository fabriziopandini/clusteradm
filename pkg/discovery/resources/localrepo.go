package resources

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog"
)

func lookupLocalRepository(path string) ([]Resource, error) {
	r := localRepository{
		path: path,
	}

	fmt.Printf("reading resources from %q local repository...\n", r.String())
	return r.getResources()
}

// localRepository holds references to a local folder/file where
// resources for deploying cluster API, infrastructure providers
// or bootstrap providers are stored.
type localRepository struct {
	path string
}

// String describe a localRepository as a string
func (r *localRepository) String() string {
	return r.path
}

// getResources return the resources available in the local folder/file.
// If the localRepository is a folder, also sub-folders are read, recursively.
func (r *localRepository) getResources() ([]Resource, error) {
	// explore the local folder/dir collecting relative path for each resource path
	var resourcePaths []string
	err := filepath.Walk(r.path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if !(filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
			return nil
		}

		resourcePaths = append(resourcePaths, path)
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "error reading local repository")
	}

	// read content for each resource path and pack path and content into a resource object
	var resources []Resource
	for _, path := range resourcePaths {
		klog.V(3).Infof("Reading %q", path)
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading resource content for %q", path)
		}

		relativePath := strings.TrimPrefix(path, filepath.Dir(r.path))
		resources = append(resources, Resource{
			path:    relativePath,
			content: content,
		})
	}

	return resources, nil
}
