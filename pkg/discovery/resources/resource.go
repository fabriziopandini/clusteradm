package resources

// Resource define a yaml resource to be used for deploying cluster API,
// infrastructure providers or bootstrap providers.
type Resource struct {
	path    string
	content []byte
}

// ComponentResources defines a group of yaml resource to be used for deploying cluster API,
// infrastructure providers or bootstrap providers.
// In case of multiple resources, it could be defined also which resource should be used as
// entry point for kubectl apply
type ComponentResources struct {
	resources []Resource
	apply     string
}
