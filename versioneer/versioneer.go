package versioneer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kairos-io/kairos-sdk/utils"
)

type Artifact struct {
	Flavor            string
	FlavorRelease     string
	Variant           string
	Model             string
	Arch              string
	Version           string // The Kairos version. E.g. "v2.4.2"
	SoftwareVersion   string // The k3s version. E.g. "k3sv1.26.9+k3s1"
	RegistryInspector RegistryInspector
}

func NewArtifactFromJSON(jsonStr string) (*Artifact, error) {
	result := &Artifact{}
	err := json.Unmarshal([]byte(jsonStr), result)

	return result, err
}

// NewArtifactFromOSRelease generates an artifact by inpecting the variables
// in the /etc/os-release file of a Kairos image. The variable should be
// prefixed with "KAIROS_". E.g. KAIROS_VARIANT would be used to set the Variant
// field. The function optionally takes an argument to specify a different file
// path (for testing reasons).
func NewArtifactFromOSRelease(file ...string) (*Artifact, error) {
	var err error
	result := Artifact{}

	if result.Flavor, err = utils.OSRelease("FLAVOR", file...); err != nil {
		return nil, err
	}
	if result.FlavorRelease, err = utils.OSRelease("FLAVOR_RELEASE", file...); err != nil {
		return nil, err
	}
	if result.Variant, err = utils.OSRelease("VARIANT", file...); err != nil {
		return nil, err
	}
	if result.Model, err = utils.OSRelease("MODEL", file...); err != nil {
		return nil, err
	}
	if result.Arch, err = utils.OSRelease("ARCH", file...); err != nil {
		return nil, err
	}
	if result.Version, err = utils.OSRelease("VERSION", file...); err != nil {
		return nil, err
	}
	if result.SoftwareVersion, err = utils.OSRelease("SOFTWARE_VERSION", file...); err != nil {
		return nil, err
	}

	return &result, nil
}

func (a *Artifact) Validate() error {
	if a.FlavorRelease == "" {
		return errors.New("FlavorRelease is empty")
	}
	if a.Variant == "" {
		return errors.New("Variant is empty")
	}
	if a.Model == "" {
		return errors.New("Model is empty")
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
	commonName, err := a.commonName()
	if err != nil {
		return "", err
	}

	if a.Flavor == "" {
		return "", errors.New("Flavor is empty")
	}

	return fmt.Sprintf("kairos-%s-%s", a.Flavor, commonName), nil
}

func (a *Artifact) ContainerName(registryAndOrg string) (string, error) {
	commonName, err := a.commonName()
	if err != nil {
		return "", err
	}

	if a.Flavor == "" {
		return "", errors.New("Flavor is empty")
	}

	commonName = strings.ReplaceAll(commonName, "+", "-")

	return fmt.Sprintf("%s/%s:%s", registryAndOrg, a.Flavor, commonName), nil
}

func (a *Artifact) commonName() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}

	result := fmt.Sprintf("%s-%s-%s-%s-%s",
		a.FlavorRelease, a.Variant, a.Arch, a.Model, a.Version)

	if a.SoftwareVersion != "" {
		result = fmt.Sprintf("%s-%s", result, a.SoftwareVersion)
	}

	return result, nil
}
