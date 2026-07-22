package machine_test

import (
	"os"

	. "github.com/kairos-io/kairos-sdk/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BootCMDLine", func() {
	Context("kairos.config stanzas", func() {
		writeCmdline := func(contents string) string {
			f, err := os.CreateTemp("", "cmdline")
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = os.Remove(f.Name()) })
			Expect(os.WriteFile(f.Name(), []byte(contents), 0o644)).To(Succeed())
			return f.Name()
		}

		It("returns nothing when no stanzas are present", func() {
			path := writeCmdline("root=LABEL=X rd.immucore.debug\n")
			stanzas, err := KairosConfigStanzas(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(stanzas).To(BeEmpty())

			y, err := KairosCmdlineYAML(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(y).To(BeNil())
		})

		It("extracts a single stanza", func() {
			path := writeCmdline("root=X kairos.config=config_url=https://example.com/a.yaml\n")
			stanzas, err := KairosConfigStanzas(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(stanzas).To(ConsistOf("config_url=https://example.com/a.yaml"))
		})

		It("collects multiple stanzas and builds nested YAML", func() {
			path := writeCmdline(`kairos.config=hostname=box kairos.config=config_url=https://example.com/a.yaml kairos.config=install.auto=true`)
			y, err := KairosCmdlineYAML(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(y)).To(ContainSubstring("hostname: box"))
			Expect(string(y)).To(ContainSubstring("config_url: https://example.com/a.yaml"))
			Expect(string(y)).To(ContainSubstring("install:"))
			Expect(string(y)).To(ContainSubstring("auto: true"))
		})

		It("supports quoted values with spaces", func() {
			path := writeCmdline(`kairos.config=hostname="my box"`)
			y, err := KairosCmdlineYAML(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(y)).To(ContainSubstring("hostname: my box"))
		})

		It("drops empty payloads", func() {
			path := writeCmdline(`kairos.config= kairos.config=hostname=box`)
			stanzas, err := KairosConfigStanzas(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(stanzas).To(ConsistOf("hostname=box"))
		})
	})

	Context("cos.setup stanza", func() {
		writeCmdline := func(contents string) string {
			f, err := os.CreateTemp("", "cmdline")
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = os.Remove(f.Name()) })
			Expect(os.WriteFile(f.Name(), []byte(contents), 0o644)).To(Succeed())
			return f.Name()
		}

		It("returns empty when not set", func() {
			path := writeCmdline("root=LABEL=X rd.immucore.debug\n")
			uri, err := CosSetupURI(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(uri).To(BeEmpty())
		})

		It("extracts a URL value", func() {
			path := writeCmdline("root=X cos.setup=https://example.com/legacy.yaml\n")
			uri, err := CosSetupURI(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(uri).To(Equal("https://example.com/legacy.yaml"))
		})

		It("extracts a file path value", func() {
			uri := CosSetupURIFromString(`cos.setup=/oem/50-extra.yaml`)
			Expect(uri).To(Equal("/oem/50-extra.yaml"))
		})

		It("drops empty payloads and lets the last occurrence win", func() {
			uri := CosSetupURIFromString(`cos.setup= cos.setup=https://example.com/a.yaml cos.setup=https://example.com/b.yaml`)
			Expect(uri).To(Equal("https://example.com/b.yaml"))
		})
	})

	Context("parses data", func() {

		It("returns cmdline if provided", func() {
			f, err := os.CreateTemp("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(f.Name())

			err = os.WriteFile(f.Name(), []byte(`config_url="foo bar" baz.bar=""`), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			b, err := DotToYAML(f.Name())
			Expect(err).ToNot(HaveOccurred())

			Expect(string(b)).To(Equal("baz:\n    bar: \"\"\nconfig_url: foo bar\n"), string(b))
		})
		It("works if cmdline contains a dash or underscore", func() {
			f, err := os.CreateTemp("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(f.Name())

			err = os.WriteFile(f.Name(), []byte(`config-url="foo bar" ba_z.bar=""`), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			_, err = DotToYAML(f.Name())
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
