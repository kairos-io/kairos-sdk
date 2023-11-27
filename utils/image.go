package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/archive"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

var defaultRetryBackoff = remote.Backoff{
	Duration: 1.0 * time.Second,
	Factor:   3.0,
	Jitter:   0.1,
	Steps:    3,
}

var defaultRetryPredicate = func(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || strings.Contains(err.Error(), "connection refused") {
		logs.Warn.Printf("retrying %v", err)
		return true
	}
	return false
}

// ExtractOCIImage will extract a given targetImage into a given targetDestination
func ExtractOCIImage(img v1.Image, targetDestination string) error {
	reader := mutate.Extract(img)

	_, err := archive.Apply(context.Background(), targetDestination, reader)

	return err
}

// GetImage if returns the proper image to pull with transport and auth
// tries local daemon first and then fallbacks into remote
func GetImage(targetImage, targetPlatform string) (v1.Image, error) {
	var platform *v1.Platform
	var image v1.Image
	var err error

	if targetPlatform != "" {
		platform, err = v1.ParsePlatform(targetPlatform)
		if err != nil {
			return image, err
		}
	} else {
		platform, err = v1.ParsePlatform(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
		if err != nil {
			return image, err
		}
	}

	ref, err := name.ParseReference(targetImage)
	if err != nil {
		return image, err
	}

	tr := transport.NewRetry(http.DefaultTransport,
		transport.WithRetryBackoff(defaultRetryBackoff),
		transport.WithRetryPredicate(defaultRetryPredicate),
	)

	image, err = daemon.Image(ref)

	if err != nil {
		image, err = remote.Image(ref,
			remote.WithTransport(tr),
			remote.WithPlatform(*platform),
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
		)
	}

	return image, err
}

func GetOCIImageSize(targetImage, targetPlatform string) (int64, error) {
	var size int64
	var img v1.Image
	var err error

	img, err = GetImage(targetImage, targetPlatform)
	if err != nil {
		return size, err
	}
	layers, _ := img.Layers()
	for _, layer := range layers {
		s, _ := layer.Size()
		size += s
	}

	return size, nil
}
