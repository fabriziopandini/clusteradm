package resources

var defaultRepositories = map[string]string{

	// cluster API default repository
	"CAPI": "https://github.com/kubernetes-sigs/cluster-api/releases/latest/cluster-api-components.yaml",

	// Infrastructure providers default repositories
	"aws": "https://github.com/kubernetes-sigs/cluster-api-provider-aws/releases/latest/infrastructure-components.yaml",

	"vsphere": "https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases/latest/infrastructure-components.yaml",

	// Bootstrap providers default repositories
	"kubeadm": "https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/latest/bootstrap-components.yaml",
}
