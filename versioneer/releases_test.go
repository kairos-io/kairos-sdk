package versioneer_test

import (
	"github.com/kairos-io/kairos-sdk/versioneer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type FakeRegistryInspector struct {
	FakeTags versioneer.TagList
}

func (i *FakeRegistryInspector) TagList(registryAndOrg, repo string) (versioneer.TagList, error) {
	return i.FakeTags, nil
}

var _ = Describe("NewerVersions", func() {
	var artifact versioneer.Artifact
	BeforeEach(func() {
		inspector := &FakeRegistryInspector{
			FakeTags: getFakeTags(),
		}

		artifact = versioneer.Artifact{
			Flavor:            "opensuse",
			FlavorRelease:     "leap-15.5",
			Variant:           "standard",
			Model:             "generic",
			Arch:              "amd64",
			Version:           "v2.4.2",
			RegistryInspector: inspector,
		}
	})

	It("returns all tags with a Version higher than the current one", func() {
		versions, err := artifact.NewerVersions("quay.io/kairos")
		Expect(err).ToNot(HaveOccurred(), versions)

		//out, _ := json.Marshal(versions)
		//fmt.Printf("%+v\n", string(out))
		// TODO
	})
})
