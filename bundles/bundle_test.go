package bundles_test

import (
	"io"
	"os"
	"path"
	"path/filepath"

	. "github.com/kairos-io/kairos-sdk/bundles"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bundle", func() {
	Context("install", func() {
		It("installs packages from luet repos", func() {
			dir, err := os.MkdirTemp("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(dir)
			_ = os.MkdirAll(filepath.Join(dir, "var", "tmp", "luet"), os.ModePerm)
			err = RunBundles([]BundleOption{WithDBPath(dir), WithRootFS(dir), WithTarget("package://utils/edgevpn")})
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(dir, "usr", "bin", "edgevpn")).To(BeARegularFile())
		})

		It("installs from container images", func() {
			dir, err := os.MkdirTemp("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(dir)
			err = RunBundles([]BundleOption{WithDBPath(dir), WithRootFS(dir), WithTarget("container://quay.io/mocaccino/extra:edgevpn-utils-0.15.0")})
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(dir, "usr", "bin", "edgevpn")).To(BeARegularFile())
		})

		When("local is true", func() {
			var installer BundleInstaller
			var config *BundleConfig
			var tmpDir, tmpFile string
			var err error

			BeforeEach(func() {
				config = &BundleConfig{
					LocalFile: true,
				}
			})

			AfterEach(func() {
				os.RemoveAll(tmpDir)
			})

			JustBeforeEach(func() {
				installer, err = NewBundleInstaller(*config)
				Expect(err).ToNot(HaveOccurred())
			})

			When("type is container", func() {
				BeforeEach(func() {
					tmpDir, err = os.MkdirTemp("", "test")
					Expect(err).ToNot(HaveOccurred())
					tmpFile = path.Join(tmpDir, "grub-config.tar")
					copyFile("../assets/grub-config.tar", tmpFile)

					config.Target = "container://" + tmpFile
					config.DBPath = "/usr/local/.kairos/db"
					config.RootPath = "/"
				})

				It("installs", func() {
					expectInstalled(installer, config)
				})
			})

			When("type is docker", func() {
				BeforeEach(func() {
					tmpDir, err = os.MkdirTemp("", "test")
					Expect(err).ToNot(HaveOccurred())
					tmpFile = path.Join(tmpDir, "grub-config.tar")
					copyFile("../assets/grub-config.tar", tmpFile)

					config.Target = "docker://" + tmpFile
					config.DBPath = "/usr/local/.kairos/db"
					config.RootPath = "/"
				})

				It("installs", func() {
					expectInstalled(installer, config)
				})
			})

			When("type is run", func() {
				BeforeEach(func() {
					// Ensure no leftovers from previous tests
					// These tests are meant to run in a container (Earthly), so it should
					// be ok to delete files like this.
					os.RemoveAll("/var/lib/rancher/k3s/server/manifests/longhorn.yaml")

					tmpDir, err = os.MkdirTemp("", "test")
					Expect(err).ToNot(HaveOccurred())
					tmpFile = path.Join(tmpDir, "longhorn-bundle.tar")
					copyFile("../assets/longhorn-bundle.tar", tmpFile)
					config.Target = "run://" + tmpFile
				})

				It("installs", func() {
					_, err := os.Stat("/var/lib/rancher/k3s/server/manifests/longhorn.yaml")
					Expect(err).To(HaveOccurred())

					err = installer.Install(config)
					Expect(err).ToNot(HaveOccurred())
					_, err = os.Stat("/var/lib/rancher/k3s/server/manifests/longhorn.yaml")
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		When("local is false", func() {
			var installer BundleInstaller
			var config *BundleConfig
			var err error

			BeforeEach(func() {
				config = &BundleConfig{
					LocalFile: false,
				}
			})

			JustBeforeEach(func() {
				installer, err = NewBundleInstaller(*config)
				Expect(err).ToNot(HaveOccurred())
			})

			When("type is container", func() {
				BeforeEach(func() {
					config.Target = "container://quay.io/kairos/packages:grub-config-static-0.9"
					config.DBPath = "/usr/local/.kairos/db"
					config.RootPath = "/"
				})

				It("installs", func() {
					expectInstalled(installer, config)
				})
			})

			When("type is docker", func() {
				BeforeEach(func() {
					config.Target = "docker://quay.io/kairos/packages:grub-config-static-0.9"
					config.DBPath = "/usr/local/.kairos/db"
					config.RootPath = "/"
				})

				It("installs", func() {
					expectInstalled(installer, config)
				})
			})

			When("type is run", func() {
				BeforeEach(func() {
					os.RemoveAll("/var/lib/rancher/k3s/server/manifests/longhorn.yaml")
					config.Target = "run://quay.io/kairos/community-bundles:longhorn_latest"
				})

				It("installs", func() {
					_, err := os.Stat("/var/lib/rancher/k3s/server/manifests/longhorn.yaml")
					Expect(err).To(HaveOccurred())

					err = installer.Install(config)
					Expect(err).ToNot(HaveOccurred())
					_, err = os.Stat("/var/lib/rancher/k3s/server/manifests/longhorn.yaml")
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})

// Copied from: https://opensource.com/article/18/6/copying-files-go
func copyFile(src, dst string) {
	sourceFileStat, err := os.Stat(src)
	Expect(err).ToNot(HaveOccurred())
	Expect(sourceFileStat.Mode().IsRegular()).To(BeTrue())

	source, err := os.Open(src)
	Expect(err).ToNot(HaveOccurred())
	defer source.Close()

	destination, err := os.Create(dst)
	Expect(err).ToNot(HaveOccurred())
	defer destination.Close()

	_, err = io.Copy(destination, source)
	Expect(err).ToNot(HaveOccurred())
}

func expectInstalled(installer BundleInstaller, config *BundleConfig) {
	// Ensure no leftovers from previous tests
	// These tests are meant to run in a container (Earthly), so it should
	// be ok to delete files like this.
	err := os.RemoveAll("/etc/cos/grub.cfg")
	Expect(err).ToNot(HaveOccurred())
	_, err = os.Stat("/etc/cos/grub.cfg")
	Expect(err).To(HaveOccurred())

	err = installer.Install(config)
	Expect(err).ToNot(HaveOccurred())
	_, err = os.Stat("/etc/cos/grub.cfg")
	Expect(err).ToNot(HaveOccurred())
}
