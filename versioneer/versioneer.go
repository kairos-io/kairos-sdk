package versioneer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kairos-io/kairos-sdk/utils"
)

const (
	EnvVarFlavor          = "FLAVOR"
	EnvVarFlavorRelease   = "FLAVOR_RELEASE"
	EnvVarVariant         = "VARIANT"
	EnvVarModel           = "MODEL"
	EnvVarArch            = "ARCH"
	EnvVarVersion         = "VERSION"
	EnvVarSoftwareVersion = "SOFTWARE_VERSION"
	EnvVarRegistryAndOrg  = "REGISTRY_AND_ORG"
	EnvVarID              = "ID"
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

	if result.Flavor, err = utils.OSRelease(EnvVarFlavor, file...); err != nil {
		return nil, err
	}
	if result.FlavorRelease, err = utils.OSRelease(EnvVarFlavorRelease, file...); err != nil {
		return nil, err
	}
	if result.Variant, err = utils.OSRelease(EnvVarVariant, file...); err != nil {
		return nil, err
	}
	if result.Model, err = utils.OSRelease(EnvVarModel, file...); err != nil {
		return nil, err
	}
	if result.Arch, err = utils.OSRelease(EnvVarArch, file...); err != nil {
		return nil, err
	}
	if result.Version, err = utils.OSRelease(EnvVarVersion, file...); err != nil {
		return nil, err
	}
	if result.SoftwareVersion, err = utils.OSRelease(EnvVarSoftwareVersion, file...); err != nil {
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
	return nil
}

func (a *Artifact) BootableName() (string, error) {
	commonName, err := a.commonVersionedName()
	if err != nil {
		return "", err
	}

	if a.Flavor == "" {
		return "", errors.New("Flavor is empty")
	}

	return fmt.Sprintf("kairos-%s-%s", a.Flavor, commonName), nil
}

func (a *Artifact) ContainerName(registryAndOrg string) (string, error) {
	if a.Flavor == "" {
		return "", errors.New("Flavor is empty")
	}

	tag, err := a.Tag()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s:%s", registryAndOrg, a.Flavor, tag), nil
}

func (a *Artifact) BaseContainerName(registryAndOrg, id string) (string, error) {
	if a.Flavor == "" {
		return "", errors.New("Flavor is empty")
	}

	if id == "" {
		return "", errors.New("no id passed")
	}

	tag, err := a.BaseTag()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s:%s-%s", registryAndOrg, a.Flavor, tag, id), nil
}

func (a *Artifact) BaseTag() (string, error) {
	return a.commonName()
}

func (a *Artifact) Tag() (string, error) {
	commonName, err := a.commonVersionedName()
	if err != nil {
		return commonName, err
	}

	return strings.ReplaceAll(commonName, "+", "-"), nil
}

func (a *Artifact) TagList(registryAndOrg string) (TagList, error) {
	if a.RegistryInspector == nil {
		a.RegistryInspector = &DefaultRegistryInspector{}
	}

	return a.RegistryInspector.TagList(registryAndOrg, a)
}

func (a *Artifact) commonName() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}

	result := fmt.Sprintf("%s-%s-%s-%s",
		a.FlavorRelease, a.Variant, a.Arch, a.Model)

	return result, nil
}

func (a *Artifact) commonVersionedName() (string, error) {
	if a.Version == "" {
		return "", errors.New("Version is empty")
	}

	result, err := a.commonName()
	if err != nil {
		return result, err
	}

	result = fmt.Sprintf("%s-%s", result, a.Version)

	if a.SoftwareVersion != "" {
		result = fmt.Sprintf("%s-%s", result, a.SoftwareVersion)
	}

	return result, nil
}
