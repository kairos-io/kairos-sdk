package machine

import (
	"os"
	"strings"

	"github.com/google/shlex"
	"github.com/kairos-io/kairos-sdk/unstructured"
)

const (
	kairosConfigPrefix = "kairos.config="
	cosSetupPrefix     = "cos.setup="
)

// CosSetupURI returns the URI referenced by the legacy "cos.setup=" cmdline
// stanza, or an empty string when it is not set. An empty file argument
// defaults to /proc/cmdline. This stanza points at a yip config source
// (file, directory or URL) that callers should feed straight to yip.Run
// so yip fetches and executes it.
func CosSetupURI(file string) (string, error) {
	if file == "" {
		file = "/proc/cmdline"
	}
	dat, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return CosSetupURIFromString(string(dat)), nil
}

// CosSetupURIFromString is the string-based counterpart of CosSetupURI.
func CosSetupURIFromString(s string) string {
	tokens, _ := shlex.Split(s)
	uri := ""
	for _, t := range tokens {
		if !strings.HasPrefix(t, cosSetupPrefix) {
			continue
		}
		v := strings.TrimPrefix(t, cosSetupPrefix)
		if v == "" {
			continue
		}
		// last occurrence wins, matching how repeated cmdline flags
		// behave in the rest of the codebase.
		uri = strings.Trim(v, `"`)
	}
	return uri
}

func DotToYAML(file string) ([]byte, error) {
	if file == "" {
		file = "/proc/cmdline"
	}
	dat, err := os.ReadFile(file)
	if err != nil {
		return []byte{}, err
	}

	v := stringToMap(string(dat))

	return unstructured.ToYAML(v)
}

// DotStringToYAML turns a whitespace-separated set of KEY=VALUE tokens (with
// KEY supporting dot notation) into a YAML document. It is the string-based
// counterpart of DotToYAML.
func DotStringToYAML(s string) ([]byte, error) {
	return unstructured.ToYAML(stringToMap(s))
}

// KairosConfigStanzas returns the value part of every "kairos.config=" stanza
// found in the given cmdline file. The "kairos.config=" prefix is stripped so
// callers get the raw KEY=VALUE payload. An empty file argument defaults to
// /proc/cmdline. Stanzas whose payload is empty are dropped.
func KairosConfigStanzas(file string) ([]string, error) {
	if file == "" {
		file = "/proc/cmdline"
	}
	dat, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return KairosConfigStanzasFromString(string(dat)), nil
}

// KairosConfigStanzasFromString is the string-based counterpart of
// KairosConfigStanzas.
func KairosConfigStanzasFromString(s string) []string {
	tokens, _ := shlex.Split(s)
	var out []string
	for _, t := range tokens {
		if !strings.HasPrefix(t, kairosConfigPrefix) {
			continue
		}
		v := strings.TrimPrefix(t, kairosConfigPrefix)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

// KairosCmdlineYAML builds a YAML document out of every "kairos.config=KEY=VALUE"
// stanza on the cmdline. KEY supports dot notation for nested maps, so
//
//	kairos.config=install.auto=true kairos.config=hostname=box
//
// yields nested YAML with both install.auto and hostname set. List/array
// indices in the key (e.g. foo.0.bar) are not supported by the underlying
// dot-to-yaml pipeline. Repeated occurrences are merged and the last value
// for a given key wins. Values may contain spaces if the whole stanza is
// quoted on the kernel command line (e.g. kairos.config="hostname=my box").
// Returns (nil, nil) when no stanzas are present so callers can skip cheaply.
func KairosCmdlineYAML(file string) ([]byte, error) {
	stanzas, err := KairosConfigStanzas(file)
	if err != nil {
		return nil, err
	}
	return kairosCmdlineYAMLFromStanzas(stanzas)
}

// KairosCmdlineYAMLFromString is the string-based counterpart of KairosCmdlineYAML.
// It is useful when the cmdline has already been read from a mock filesystem.
func KairosCmdlineYAMLFromString(s string) ([]byte, error) {
	return kairosCmdlineYAMLFromStanzas(KairosConfigStanzasFromString(s))
}

func kairosCmdlineYAMLFromStanzas(stanzas []string) ([]byte, error) {
	if len(stanzas) == 0 {
		return nil, nil
	}
	v := map[string]interface{}{}
	for _, entry := range stanzas {
		parts := strings.SplitN(entry, "=", 2)
		value := "true"
		if len(parts) > 1 {
			value = strings.Trim(parts[1], `"`)
		}
		key := strings.Trim(parts[0], `"`)
		v[key] = value
	}
	return unstructured.ToYAML(v)
}

func stringToMap(s string) map[string]interface{} {
	v := map[string]interface{}{}

	splitted, _ := shlex.Split(s)
	for _, item := range splitted {
		// kairos.config= and cos.setup= have dedicated parsers
		// (KairosCmdlineYAML, CosSetupURI). Skip them here so their
		// KEY=VALUE payload does not leak into the generic dot-nested
		// map as a spurious "kairos.config" / "cos.setup" key.
		if strings.HasPrefix(item, kairosConfigPrefix) || strings.HasPrefix(item, cosSetupPrefix) {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		value := "true"
		if len(parts) > 1 {
			value = strings.Trim(parts[1], `"`)
		}
		key := strings.Trim(parts[0], `"`)
		v[key] = value
	}

	return v
}
