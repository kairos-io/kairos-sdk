package versioneer

// NewerVersions returns all the images in the given container registry
// that have a higher Version number than the current Artifact.
// All other fields still be the same (e.g. SoftwareVersion).
// This function can be used if one wants to upgrade to a newer
// Kairos image without changing anything else (not even the k3s version).
// Given we usually bump k3s versions with every release and that we only
// maintain the latest 3 minor releases (highest patch release for each),
// this function might not return any results.
func (a *Artifact) NewerVersions(registryAndOrg string) ([]string, error) {
	var err error
	var result []string

	result, err = a.tagList(registryAndOrg)
	if err != nil {
		return result, err
	}

	// TODO: Filter

	return result, nil
}

// NewerSofwareVersions returns all the images in the given container registry
// that have a higher SoftwareVersion number than the current Artifact.
// All other fields still be the same (e.g. Version).
// This function can be used if one wants to upgrade to a newer k3s version
// without changing anything else (not even the Kairos version).
// We create artifacts for the latest k3s minor versions for each flavor so
// unless the user installed the latest k3s initially, this function may return
// up to 2 more results.
func (a *Artifact) NewerSofwareVersions(registryAndOrg string) {
}

// NewerAllVersions returns all the images in the given container registry
// that have a higher Version or SoftwareVersion number than the current Artifact.
// All other fields still be the same.
// This function can be used if one wants to upgrade to some later Kairos version
// and if needed upgrade k3s as well. If a newer version of Kairos has been
// produced, this function should return some results.
func (a *Artifact) NewerAllVersions(registryAndOrg string) {
}

func (a *Artifact) tagList(registryAndOrg string) (TagList, error) {
	if a.RegistryInspector == nil {
		a.RegistryInspector = &DefaultRegistryInspector{}
	}

	return a.RegistryInspector.TagList(registryAndOrg, a.Flavor)
}
