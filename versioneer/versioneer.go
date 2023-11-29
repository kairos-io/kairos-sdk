package versioneer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

type Artifact struct {
	Flavor          string
	FlavorRelease   string
	Variant         string
	Model           string
	Arch            string
	Version         string // The Kairos version. E.g. "v2.4.2"
	SoftwareVersion string // The k3s version. E.g. "k3sv1.26.9+k3s1"
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
	if len(file) > 1 {
		return nil, errors.New("too many arguments given")
	}

	var filePath string
	if len(file) == 0 {
		filePath = "/etc/os-release"
	} else {
		filePath = file[0]
	}

	out, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	content := string(out)
	result := Artifact{
		Flavor:          findValueInText("KAIROS_FLAVOR", content),
		FlavorRelease:   findValueInText("KAIROS_FLAVOR_RELEASE", content),
		Variant:         findValueInText("KAIROS_VARIANT", content),
		Model:           findValueInText("KAIROS_MODEL", content),
		Arch:            findValueInText("KAIROS_ARCH", content),
		Version:         findValueInText("KAIROS_VERSION", content),
		SoftwareVersion: findValueInText("KAIROS_SOFTWARE_VERSION", content),
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

func findValueInText(key string, content string) string {
	// Define a regular expression pattern
	pattern := fmt.Sprintf(`%s=(.*)\s`, key)
	regexpObject := regexp.MustCompile(pattern)

	// Find all matches in a string
	match := regexpObject.FindStringSubmatch(content)

	if len(match) < 2 {
		return ""
	}

	return match[1]
}
