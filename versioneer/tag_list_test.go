package versioneer_test

import (
	"github.com/kairos-io/kairos-sdk/versioneer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TagList", func() {
	var tagList versioneer.TagList

	BeforeEach(func() {
		tagList = getFakeTags()
	})

	Describe("Images", func() {
		It("returns only tags matching images", func() {
			images := tagList.Images()

			// Sanity check, that we didn't filter everything out
			Expect(len(images)).To(BeNumerically(">", 4))

			expectOnlyImages(images)
		})
	})

	Describe("Sorted", func() {
		It("returns tags sorted alphabetically", func() {
			images := tagList.Images()
			sortedImages := images.Sorted()

			// Sanity checks
			Expect(len(images)).To(BeNumerically(">", 4))
			Expect(len(sortedImages)).To(Equal(len(images)))

			Expect(isSorted(images)).To(BeFalse())
			Expect(isSorted(sortedImages)).To(BeTrue())
		})
	})

	Describe("RSorted", func() {
		It("returns tags in reverse alphabetical order", func() {
			images := tagList.Images()
			rSortedImages := images.RSorted()

			// Sanity checks
			Expect(len(images)).To(BeNumerically(">", 4))
			Expect(len(rSortedImages)).To(Equal(len(images)))

			Expect(isRSorted(images)).To(BeFalse())
			Expect(isRSorted(rSortedImages)).To(BeTrue())
		})
	})

	Describe("OtherVersions", func() {
		var artifact versioneer.Artifact
		BeforeEach(func() {
			artifact = versioneer.Artifact{
				Flavor:          "opensuse",
				FlavorRelease:   "leap-15.5",
				Variant:         "standard",
				Model:           "generic",
				Arch:            "amd64",
				Version:         "v2.4.2-rc1",
				SoftwareVersion: "k3sv1.27.6-k3s1",
			}
		})

		It("returns only tags with different version", func() {
			otherVersions := tagList.OtherVersions(artifact)

			Expect(otherVersions).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.27.6-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1"))
		})
	})

	Describe("NewerVersions", func() {
		var artifact versioneer.Artifact
		BeforeEach(func() {
			artifact = versioneer.Artifact{
				Flavor:          "opensuse",
				FlavorRelease:   "leap-15.5",
				Variant:         "standard",
				Model:           "generic",
				Arch:            "amd64",
				Version:         "v2.4.2-rc2",
				SoftwareVersion: "k3sv1.27.6-k3s1",
			}
		})

		It("returns only tags with newer Version field (the rest similar)", func() {
			versions := tagList.NewerVersions(artifact)

			Expect(versions).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1"))
		})
	})

	Describe("OtherSoftwareVersions", func() {
		var artifact versioneer.Artifact
		BeforeEach(func() {
			artifact = versioneer.Artifact{
				Flavor:          "opensuse",
				FlavorRelease:   "leap-15.5",
				Variant:         "standard",
				Model:           "generic",
				Arch:            "amd64",
				Version:         "v2.4.2-rc1",
				SoftwareVersion: "k3sv1.27.6-k3s1",
			}
		})

		It("returns only tags with different SoftwareVersion", func() {
			tags := tagList.OtherSoftwareVersions(artifact)

			Expect(tags).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.26.9-k3s1",
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.28.2-k3s1"))
		})
	})

	Describe("NewerSofwareVersions", func() {
		var artifact versioneer.Artifact
		BeforeEach(func() {
			artifact = versioneer.Artifact{
				Flavor:          "opensuse",
				FlavorRelease:   "leap-15.5",
				Variant:         "standard",
				Model:           "generic",
				Arch:            "amd64",
				Version:         "v2.4.2-rc1",
				SoftwareVersion: "k3sv1.27.6-k3s1",
			}
		})

		It("returns only tags with newer SoftwareVersion", func() {
			tags := tagList.NewerSofwareVersions(artifact, "k3s")

			Expect(tags).To(HaveExactElements(
				"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.28.2-k3s1"))
		})
	})

	Describe("OtherAnyVersion", func() {
		var artifact versioneer.Artifact
		BeforeEach(func() {
			artifact = versioneer.Artifact{
				Flavor:          "opensuse",
				FlavorRelease:   "leap-15.5",
				Variant:         "standard",
				Model:           "generic",
				Arch:            "amd64",
				Version:         "v2.4.2-rc1",
				SoftwareVersion: "k3sv1.27.6-k3s1",
			}
		})

		It("returns only tags with different Version and/or SoftwareVersion", func() {
			tags := tagList.OtherAnyVersion(artifact)

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
	})

	Describe("NewerAnyVersion", func() {
		var artifact versioneer.Artifact
		When("artifact has SoftwareVersion", func() {

			BeforeEach(func() {
				artifact = versioneer.Artifact{
					Flavor:          "opensuse",
					FlavorRelease:   "leap-15.5",
					Variant:         "standard",
					Model:           "generic",
					Arch:            "amd64",
					Version:         "v2.4.2-rc1",
					SoftwareVersion: "k3sv1.27.6-k3s1",
				}
			})

			It("returns only tags with newer Versions and/or SoftwareVersion", func() {
				tags := tagList.NewerAnyVersion(artifact, "k3s")

				Expect(tags).To(HaveExactElements(
					"leap-15.5-standard-amd64-generic-v2.4.2-rc1-k3sv1.28.2-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.28.2-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-rc2-k3sv1.27.6-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.27.6-k3s1",
					"leap-15.5-standard-amd64-generic-v2.4.2-k3sv1.28.2-k3s1"))
			})
		})

		When("artifact has no SoftwareVersion", func() {
			BeforeEach(func() {
				artifact = versioneer.Artifact{
					Flavor:          "opensuse",
					FlavorRelease:   "leap-15.5",
					Variant:         "core",
					Model:           "generic",
					Arch:            "amd64",
					Version:         "v2.4.2-rc1",
					SoftwareVersion: "",
				}
			})

			It("returns only tags with newer Versions and/or SoftwareVersion", func() {
				tags := tagList.NewerAnyVersion(artifact, "k3s")

				Expect(tags).To(HaveExactElements(
					"leap-15.5-core-amd64-generic-v2.4.2-rc2",
					"leap-15.5-core-amd64-generic-v2.4.2"))
			})
		})
	})
})

func expectOnlyImages(images versioneer.TagList) {
	Expect(images).ToNot(ContainElement(ContainSubstring(".att")))
	Expect(images).ToNot(ContainElement(ContainSubstring(".sbom")))
	Expect(images).ToNot(ContainElement(ContainSubstring(".sig")))
	Expect(images).ToNot(ContainElement(ContainSubstring("-img")))

	Expect(images).To(HaveEach(MatchRegexp((".*-(core|standard)-(amd64|arm64)-.*-v.*"))))
}

func isSorted(tl versioneer.TagList) bool {
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

func isRSorted(tl versioneer.TagList) bool {
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
