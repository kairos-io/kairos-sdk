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
			images := *tagList.Images()

			Expect(len(images)).To(BeNumerically(">", 10))
			Expect(images).ToNot(ContainElement(ContainSubstring(".att")))
			Expect(images).ToNot(ContainElement(ContainSubstring(".sbom")))
			Expect(images).ToNot(ContainElement(ContainSubstring(".sig")))
			Expect(images).ToNot(ContainElement(ContainSubstring("-img")))

			Expect(images).To(HaveEach(MatchRegexp((".*-(core|standard)-(amd64|arm64)-.*-v.*"))))
		})
	})
})
