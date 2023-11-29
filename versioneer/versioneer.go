package versioneer

import (
	"errors"
	"fmt"
)

type Artifact struct {
	Flavor          string
	FlavorRelease   string
	Variant         string
	Model           string
	BaseImage       string
	Arch            string
	Version         string // The Kairos version. E.g. "v2.4.2"
	SoftwareVersion string // The k3s version. E.g. "k3sv1.26.9+k3s1"
}

func (a *Artifact) Validate() error {
	if a.Flavor == "" {
		return errors.New("Flavor is empty")
	}
	if a.FlavorRelease == "" {
		return errors.New("FlavorRelease is empty")
	}
	if a.Variant == "" {
		return errors.New("Variant is empty")
	}
	if a.Model == "" {
		return errors.New("Model is empty")
	}
	if a.BaseImage == "" {
		return errors.New("BaseImage is empty")
	}
	if a.Arch == "" {
		return errors.New("Arch is empty")
	}
	if a.Version == "" {
		return errors.New("Version is empty")
	}
	return nil
}

func (a *Artifact) BootableName() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}

	result := fmt.Sprintf("kairos-%s-%s-%s-%s-%s-%s",
		a.Flavor, a.FlavorRelease, a.Variant, a.Arch, a.Model, a.Version)

	if a.SoftwareVersion != "" {
		result = fmt.Sprintf("%s-%s", result, a.SoftwareVersion)
	}

	return result, nil
}
