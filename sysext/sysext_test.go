package sysext

import (
	"bytes"
	"context"
	"fmt"
	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/kairos-io/kairos-sdk/types"
	"github.com/kairos-io/kairos-sdk/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "sysext Test Suite")
}

var _ = Describe("sysext", Label("sysext"), Ordered, func() {
	var dest string
	var image v1.Image
	var imageTag string
	var buf bytes.Buffer
	var log types.KairosLogger
	var err error

	BeforeEach(func() {
		buf = bytes.Buffer{}
		log = types.NewBufferLogger(&buf)
		dest, err = os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			_, _ = GinkgoWriter.Write(buf.Bytes())
		}
		Expect(os.RemoveAll(dest)).To(Succeed())
	})

	When("Using a normal image", func() {
		BeforeEach(func() {
			imageTag = createTestDockerImage()
			By(fmt.Sprintf("Created image %s", imageTag))
			image, err = utils.GetImage(imageTag, utils.GetCurrentPlatform(), nil, nil)
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			_, _ = cli.ImageRemove(context.Background(), imageTag, dockerImage.RemoveOptions{Force: true})
			By(fmt.Sprintf("Removed image %s", imageTag))
		})
		It("should extract the files into the dir", func() {
			err = ExtractFilesFromLastLayer(image, dest, log, DefaultAllowListRegex)
			Expect(err).ToNot(HaveOccurred())
			_, err := os.Stat(filepath.Join(dest, "usr", "yes"))
			Expect(err).ToNot(HaveOccurred())
			_, err = os.Stat(filepath.Join(dest, "etc", "yes"))
			Expect(err).ToNot(HaveOccurred())
			_, err = os.Stat(filepath.Join(dest, "opt", "nope"))
			Expect(err).To(HaveOccurred())
			_, err = os.Stat(filepath.Join(dest, "var", "nope"))
			Expect(err).To(HaveOccurred())
		})
		It("properly uses the allowList", func() {
			allowList := regexp.MustCompile(`^var|^/var`)
			err = ExtractFilesFromLastLayer(image, dest, log, allowList)
			Expect(err).ToNot(HaveOccurred())
			_, err := os.Stat(filepath.Join(dest, "usr", "yes"))
			Expect(err).To(HaveOccurred())
			_, err = os.Stat(filepath.Join(dest, "etc", "yes"))
			Expect(err).To(HaveOccurred())
			_, err = os.Stat(filepath.Join(dest, "opt", "nope"))
			Expect(err).To(HaveOccurred())
			_, err = os.Stat(filepath.Join(dest, "var", "nope"))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("Using an empty image", func() {
		BeforeEach(func() {
			imageTag = createEmptyDockerImage()
			By(fmt.Sprintf("Created image %s", imageTag))
			image, err = utils.GetImage(imageTag, utils.GetCurrentPlatform(), nil, nil)
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			_, _ = cli.ImageRemove(context.Background(), imageTag, dockerImage.RemoveOptions{Force: true})
			By(fmt.Sprintf("Removed image %s", imageTag))
		})
		It("Fails with no layers image", func() {
			// Cleanup existing image before creating a new one

			err = ExtractFilesFromLastLayer(image, dest, log, DefaultAllowListRegex)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(ErrorImageNoLayers))
		})
	})
})

func createEmptyDockerImage() string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	b := make([]rune, 8)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	img, err := mutate.AppendLayers(empty.Image)
	Expect(err).ToNot(HaveOccurred())
	tag, err := name.NewTag(fmt.Sprintf("kairos-empty-%s:latest", string(b)))
	Expect(err).ToNot(HaveOccurred())
	_, err = daemon.Write(tag, img)
	Expect(err).ToNot(HaveOccurred())

	return tag.String()
}

func createTestDockerImage() string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	b := make([]rune, 8)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	// We don't care about this layer so make it a bit fake
	fistLayer, _ := crane.Layer(map[string][]byte{
		"/etc/one":     []byte("hello"),
		"/etc/another": []byte("world"),
	})

	secondLayer, err := tarball.LayerFromFile("testdata/test.tar")
	Expect(err).ToNot(HaveOccurred())
	img, err := mutate.AppendLayers(empty.Image, fistLayer, secondLayer)
	Expect(err).ToNot(HaveOccurred())
	tag, err := name.NewTag(fmt.Sprintf("kairos-test-%s:latest", string(b)))
	Expect(err).ToNot(HaveOccurred())
	_, err = daemon.Write(tag, img)
	Expect(err).ToNot(HaveOccurred())

	return tag.String()
}
