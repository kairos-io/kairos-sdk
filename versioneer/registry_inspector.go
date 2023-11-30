package versioneer

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
)

type RegistryInspector interface {
	TagList(registryAndOrg, repo string) (TagList, error)
}

type DefaultRegistryInspector struct{}

func (i *DefaultRegistryInspector) TagList(registryAndOrg, repo string) (TagList, error) {
	var tags TagList
	var err error

	tags, err = crane.ListTags(fmt.Sprintf("%s/%s", registryAndOrg, repo))
	if err != nil {
		return tags, err
	}

	return tags, nil
}
