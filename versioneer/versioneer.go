package versioneer

import "errors"

type Artifact struct {
	Flavor        string
	FlavorRelease string
	Variant       string
	Model         string
	BaseImage     string
	Arch          string
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
	return nil
}

func (a *Artifact) BootableName() {
}
