package versioneer_test

import (
	"github.com/kairos-io/kairos-sdk/versioneer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TagList", func() {
	var tagList versioneer.TagList

	BeforeEach(func() {
		tagList = versioneer.TagList{
			Tags:           getFakeTags(),
			RegistryAndOrg: "quay.io/kairos",
		}
	})

	Describe("Images", func() {
		It("returns only tags matching images", func() {
			images := tagList.Images()

			// Sanity check, that we didn't filter everything out
			Expect(len(images.Tags)).To(BeNumerically(">", 4))

			expectOnlyImages(images.Tags)
		})
	})

	Describe("Sorted", func() {
		It("returns tags sorted alphabetically", func() {
			images := tagList.Images()
			sortedImages := images.Sorted()

			// Sanity checks
			Expect(len(images.Tags)).To(BeNumerically(">", 4))
			Expect(len(sortedImages.Tags)).To(Equal(len(images.Tags)))

			Expect(isSorted(images.Tags)).To(BeFalse())
			Expect(isSorted(sortedImages.Tags)).To(BeTrue())
		})
	})

	Describe("RSorted", func() {
		It("returns tags in reverse alphabetical order", func() {
			images := tagList.Images()
			rSortedImages := images.RSorted()

			// Sanity checks
			Expect(len(images.Tags)).To(BeNumerically(">", 4))
			Expect(len(rSortedImages.Tags)).To(Equal(len(images.Tags)))

			Expect(isRSorted(images.Tags)).To(BeFalse())
			Expect(isRSorted(rSortedImages.Tags)).To(BeTrue())
		})
	})

	Describe("OtherVersions", func() {
		BeforeEach(func() {
			tagList.Artifact = &versioneer.Artifact{
				Flavor:                "opensuse",
				FlavorRelease:         "leap-15.5",
				Variant:               "standard",
				Model:                 "generic",
				Arch:                  "amd64",
				Version:               "v2.4.2-rc1",
				SoftwareVersion:       "v1.27.6+k3s1",
				SoftwareVersionPrefix: "k3s",
			}
		})

		It("returns only tags with different version", func() {
			tags := tagList.OtherVersions().Tags

			Expect(tags).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.27.6-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1"))
		})
	})

	Describe("NewerVersions", func() {
		BeforeEach(func() {
			tagList.Artifact = &versioneer.Artifact{
				Flavor:                "opensuse",
				FlavorRelease:         "leap-15.5",
				Variant:               "standard",
				Model:                 "generic",
				Arch:                  "amd64",
				Version:               "v2.4.2-rc2",
				SoftwareVersion:       "v1.27.6+k3s1",
				SoftwareVersionPrefix: "k3s",
			}
		})

		It("returns only tags with newer Version field (the rest similar)", func() {
			tags := tagList.NewerVersions().Tags

			Expect(tags).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1"))
		})
	})

	Describe("OtherSoftwareVersions", func() {
		BeforeEach(func() {
			tagList.Artifact = &versioneer.Artifact{
				Flavor:                "opensuse",
				FlavorRelease:         "leap-15.5",
				Variant:               "standard",
				Model:                 "generic",
				Arch:                  "amd64",
				Version:               "v2.4.2-rc1",
				SoftwareVersion:       "v1.27.6+k3s1",
				SoftwareVersionPrefix: "k3s",
			}
		})

		It("returns only tags with different SoftwareVersion", func() {
			tags := tagList.OtherSoftwareVersions().Tags

			Expect(tags).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.26.9-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.28.2-k3s1"))
		})
	})

	Describe("NewerSofwareVersions", func() {
		BeforeEach(func() {
			tagList.Artifact = &versioneer.Artifact{
				Flavor:                "opensuse",
				FlavorRelease:         "leap-15.5",
				Variant:               "standard",
				Model:                 "generic",
				Arch:                  "amd64",
				Version:               "v2.4.2-rc1",
				SoftwareVersion:       "v1.27.6+k3s1",
				SoftwareVersionPrefix: "k3s",
			}
		})

		It("returns only tags with newer SoftwareVersion", func() {
			tags := tagList.NewerSofwareVersions().Tags

			Expect(tags).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.28.2-k3s1"))
		})
	})

	Describe("OtherAnyVersion", func() {
		BeforeEach(func() {
			tagList.Artifact = &versioneer.Artifact{
				Flavor:                "opensuse",
				FlavorRelease:         "leap-15.5",
				Variant:               "standard",
				Model:                 "generic",
				Arch:                  "amd64",
				Version:               "v2.4.2-rc1",
				SoftwareVersion:       "v1.27.6+k3s1",
				SoftwareVersionPrefix: "k3s",
			}
		})

		It("returns only tags with different Version and/or SoftwareVersion", func() {
			tags := tagList.OtherAnyVersion().Tags

			Expect(tags).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.26.9-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.28.2-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.28.2-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.26.9-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.27.6-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.26.9-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.28.2-k3s1"))
		})

		It("returns a TagList that has the same RegistryAndOrg", func() {
			newTagList := tagList.OtherAnyVersion()

			Expect(newTagList.RegistryAndOrg).To(Equal("quay.io/kairos"))
		})
	})

	Describe("NewerAnyVersion", func() {
		When("artifact has SoftwareVersion", func() {
			BeforeEach(func() {
				tagList.Artifact = &versioneer.Artifact{
					Flavor:                "opensuse",
					FlavorRelease:         "leap-15.5",
					Variant:               "standard",
					Model:                 "generic",
					Arch:                  "amd64",
					Version:               "v2.4.2-rc1",
					SoftwareVersion:       "v1.27.6+k3s1",
					SoftwareVersionPrefix: "k3s",
				}
			})

			It("returns only tags with newer Versions and/or SoftwareVersion", func() {
				tags := tagList.NewerAnyVersion().Tags

				Expect(tags).To(HaveExactElements(
					"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.28.2-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.28.2-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.27.6-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.28.2-k3s1"))
			})

			It("returns a TagList that has the same RegistryAndOrg", func() {
				newTagList := tagList.NewerAnyVersion()

				Expect(newTagList.RegistryAndOrg).To(Equal("quay.io/kairos"))
			})
		})

		When("artifact has no SoftwareVersion", func() {
			BeforeEach(func() {
				tagList.Artifact = &versioneer.Artifact{
					Flavor:                "opensuse",
					FlavorRelease:         "leap-15.5",
					Variant:               "core",
					Model:                 "generic",
					Arch:                  "amd64",
					Version:               "v2.4.2-rc1",
					SoftwareVersion:       "",
					SoftwareVersionPrefix: "k3s",
				}
			})

			It("returns only tags with newer Versions and/or SoftwareVersion", func() {
				tags := tagList.NewerAnyVersion().Tags

				Expect(tags).To(HaveExactElements(
					"leap-15.5-core-amd64-generic-v2.4.2-rc2",
					"leap-15.5-core-amd64-generic-v2.4.2"))
			})
		})
	})

	Describe("NoPrereleases", func() {
		When("Artifact doesn't have a SoftwareVersion", func() {
			BeforeEach(func() {
				tagList.Artifact = &versioneer.Artifact{
					Flavor:          "opensuse",
					FlavorRelease:   "leap-15.5",
					Variant:         "core",
					Model:           "generic",
					Arch:            "amd64",
					Version:         "v2.4.1",
					SoftwareVersion: "",
				}
			})

			It("returns only stable releases for Version", func() {
				tags := tagList.NoPrereleases().Tags

				Expect(tags).To(HaveExactElements("leap-15.5-core-amd64-generic-v2.4.2"))
			})

			It("returns a TagList that has the same RegistryAndOrg", func() {
				newTagList := tagList.NoPrereleases()

				Expect(newTagList.RegistryAndOrg).To(Equal("quay.io/kairos"))
			})
		})

		When("Artifact has a SoftwareVersion", func() {
			BeforeEach(func() {
				tagList.Artifact = &versioneer.Artifact{
					Flavor:                "opensuse",
					FlavorRelease:         "leap-15.5",
					Variant:               "standard",
					Model:                 "generic",
					Arch:                  "amd64",
					Version:               "v2.4.1",
					SoftwareVersion:       "v1.28.3+k3s1",
					SoftwareVersionPrefix: "k3s",
				}
			})

			It("returns only stable releases for Version", func() {
				tags := tagList.NoPrereleases().Tags

				Expect(tags).To(HaveExactElements(
					"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.26.9-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.28.2-k3s1",
				))
			})
		})
	})
})

func expectOnlyImages(images []string) {
	Expect(images).ToNot(ContainElement(ContainSubstring(".att")))
	Expect(images).ToNot(ContainElement(ContainSubstring(".sbom")))
	Expect(images).ToNot(ContainElement(ContainSubstring(".sig")))
	Expect(images).ToNot(ContainElement(ContainSubstring("-img")))

	Expect(images).To(HaveEach(MatchRegexp((".*-(core|standard)-(amd64|arm64)-.*-v.*"))))
}

func isSorted(tl []string) bool {
	for i, tag := range tl {
		if i > 0 {
			previousTag := tl[i-1]
			if previousTag > tag {
				return false
			}
		}
	}

	return true
}

func isRSorted(tl []string) bool {
	for i, tag := range tl {
		if i > 0 {
			previousTag := tl[i-1]
			if previousTag < tag {
				return false
			}
		}
	}

	return true
}
