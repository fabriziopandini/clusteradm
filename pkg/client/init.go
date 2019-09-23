package client

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/timothysc/clusteradm/pkg/discovery/resources"
	"k8s.io/klog"
)

func (c *ClusteradmClient) Init(cfg ClusteradmCfg) error {
	fmt.Println("performing init...")

	var res []*resources.ComponentResources

	// get resouces for CAPI itself
	klog.V(1).Infof("Getting resources for %q", "CAPI")
	rCAPI, err := resources.Lookup("CAPI", cfg.Repositories, cfg.GitHubToken)
	if err != nil {
		return errors.Wrapf(err, "failed to get resources for %q", "CAPI")
	}
	res = append(res, rCAPI)

	// get resouces for the bootstrap provider
	if cfg.Bootstrap != "" {
		klog.V(1).Infof("Getting resources for %q bootstrap provider", cfg.Bootstrap)
		rBootstrap, err := resources.Lookup(cfg.Bootstrap, cfg.Repositories, cfg.GitHubToken)
		if err != nil {
			return errors.Wrapf(err, "failed to get resources for %q", cfg.Bootstrap)
		}
		res = append(res, rBootstrap)
	}

	// get resources for the infrastructure provider
	for _, p := range cfg.Providers {
		klog.V(1).Infof("Getting resources for %q infrastructure provider", p)
		rInfra, err := resources.Lookup(p, cfg.Repositories, cfg.GitHubToken)
		if err != nil {
			return errors.Wrapf(err, "failed to get resources for %q", p)
		}
		res = append(res, rInfra)
	}

	// apply resources to the management cluster
	fmt.Printf("applying %d component resources to the management cluster...\n", len(res))

	return nil
}
