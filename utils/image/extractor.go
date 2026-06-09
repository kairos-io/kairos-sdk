package image

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	imagetypes "github.com/kairos-io/kairos-sdk/types/images"
	"github.com/kairos-io/kairos-sdk/utils"
)

// OCIImageExtractor is the default implementation of imagetypes.ImageExtractor:
// it pulls an OCI image with GetImage and unpacks it with ExtractOCIImage.
//
// Set Insecure to allow pulling from registries served over plain HTTP or
// presenting an untrusted/self-signed TLS certificate (see WithInsecureRegistry).
type OCIImageExtractor struct {
	Insecure bool
}

var _ imagetypes.ImageExtractor = OCIImageExtractor{}

// pullOptions translates the extractor's configuration into GetImage options.
func (e OCIImageExtractor) pullOptions() []GetOption {
	if e.Insecure {
		return []GetOption{WithInsecureRegistry()}
	}
	return nil
}

// resolvePlatform keeps a valid explicit platform, otherwise falls back to the
// current host platform.
func resolvePlatform(platformRef string) string {
	if platformRef != "" {
		if _, err := v1.ParsePlatform(platformRef); err == nil {
			return platformRef
		}
	}
	return utils.GetCurrentPlatform()
}

func (e OCIImageExtractor) ExtractImage(imageRef, destination, platformRef string, excludes ...string) error {
	img, err := GetImage(imageRef, resolvePlatform(platformRef), nil, nil, e.pullOptions()...)
	if err != nil {
		return err
	}
	return ExtractOCIImage(img, destination, excludes...)
}

func (e OCIImageExtractor) GetOCIImageSize(imageRef, platformRef string) (int64, error) {
	return GetOCIImageSize(imageRef, resolvePlatform(platformRef), nil, nil, e.pullOptions()...)
}
