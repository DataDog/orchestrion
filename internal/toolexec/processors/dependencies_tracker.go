package processors

type DependenciesTracker struct {
	// PackageMap maps import paths with package archive
	PackageMap map[string]string
}
