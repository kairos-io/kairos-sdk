package versioneer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

type TagList struct {
	Tags     []string
	Artifact *Artifact
}

// implements sort.Interface for TagList
func (tl TagList) Len() int      { return len(tl.Tags) }
func (tl TagList) Swap(i, j int) { tl.Tags[i], tl.Tags[j] = tl.Tags[j], tl.Tags[i] }
func (tl TagList) Less(i, j int) bool {
	return tl.Tags[i] < tl.Tags[j]
}

// Images returns only tags that represent images, skipping tags representing:
// - sbom
// - att
// - sig
// - -img
func (tl TagList) Images() TagList {
	pattern := `.*-(core|standard)-(amd64|arm64)-.*-v.*`
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{Artifact: tl.Artifact}
	for _, t := range tl.Tags {
		// We have to filter "-img" tags outside the regexp because golang regexp doesn't support negative lookaheads.
		if regexpObject.MatchString(t) && !strings.HasSuffix(t, "-img") {
			result.Tags = append(result.Tags, t)
		}
	}

	return result
}

// OtherVersions returns tags that match all fields of the given Artifact,
// except the Version. Should be used to return other possible versions for the same
// Kairos image (e.g. that one could upgrade to).
// This method returns all versions, not only newer ones. Use NewerVersions to
// fetch only versions, newer than the one of the Artifact.
func (tl TagList) OtherVersions() TagList {
	return tl.fieldOtherOptions(tl.Artifact.Version)
}

// NewerVersions returns OtherVersions filtered to only include tags with
// Version higher than the given artifact's.
func (tl TagList) NewerVersions() TagList {
	tags := tl.OtherVersions()

	return tags.newerVersions()
}

// OtherSoftwareVersions returns tags that match all fields of the given Artifact,
// except the SoftwareVersion. Should be used to return other possible software versions
// for the same Kairos image (e.g. that one could upgrade to).
// This method returns all versions, not only newer ones. Use NewerSofwareVersions to
// fetch only versions, newer than the one of the Artifact.
func (tl TagList) OtherSoftwareVersions() TagList {
	return tl.fieldOtherOptions(tl.Artifact.SoftwareVersion)
}

// NewerSofwareVersions returns OtherSoftwareVersions filtered to only include tags with
// SoftwareVersion higher than the given artifact's.
func (tl TagList) NewerSofwareVersions(softwarePrefix string) TagList {
	tags := tl.OtherSoftwareVersions()

	return tags.newerSoftwareVersions(softwarePrefix)
}

// OtherAnyVersion returns tags that match all fields of the given Artifact,
// except the SoftwareVersion and/or Version.
// Should be used to return tags with newer versions (Kairos or "software")
// that one could upgrade to.
// This method returns all versions, not only newer ones. Use NewerAnyVersion to
// fetch only versions, newer than the one of the Artifact.
func (tl TagList) OtherAnyVersion() TagList {
	if tl.Artifact.SoftwareVersion != "" {
		return tl.fieldOtherOptions(
			fmt.Sprintf("%s-%s", tl.Artifact.Version, tl.Artifact.SoftwareVersion))
	} else {
		return tl.fieldOtherOptions(tl.Artifact.Version)
	}
}

// NewerAnyVersion returns OtherAnyVersion filtered to only include tags with
// Version and SoftwareVersion equal or higher than the given artifact's.
// At least one of the 2 versions will be higher than the current one.
// Splitting the 2 versions is done using the softwarePrefix (first encountered,
// because our tags have a "k3s1" in the end too)
func (tl TagList) NewerAnyVersion(softwarePrefix string) TagList {
	tags := tl.OtherAnyVersion()
	if tl.Artifact.SoftwareVersion != "" {
		return tags.newerAllVersions(softwarePrefix)
	} else {
		return tags.newerVersions()
	}
}

func (tl TagList) Print() {
	for _, t := range tl.Tags {
		fmt.Println(t)
	}
}

// Sorted returns the TagList sorted alphabetically
// This means lower versions come first.
func (tl TagList) Sorted() TagList {
	newTags := make([]string, len(tl.Tags))
	copy(newTags, tl.Tags)
	sort.Strings(newTags)

	return TagList{Artifact: tl.Artifact, Tags: newTags}
}

// RSorted returns the TagList in the reverse order of Sorted
// This means higher versions come first.
func (tl TagList) RSorted() TagList {
	newTags := make([]string, len(tl.Tags))
	copy(newTags, tl.Tags)
	sort.Sort(sort.Reverse(sort.StringSlice(newTags)))

	return TagList{Artifact: tl.Artifact, Tags: newTags}
}

func (tl TagList) fieldOtherOptions(field string) TagList {
	artifactTag, err := tl.Artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(field), ".*", 1)
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{Artifact: tl.Artifact}
	for _, t := range tl.Images().Tags {
		if regexpObject.MatchString(t) && t != artifactTag {
			result.Tags = append(result.Tags, t)
		}
	}

	return result
}

func (tl TagList) newerVersions() TagList {
	artifactTag, err := tl.Artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(tl.Artifact.Version), "(.*)", 1)
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{Artifact: tl.Artifact}
	for _, t := range tl.Tags {
		version := regexpObject.FindStringSubmatch(t)[1]

		if semver.Compare(version, tl.Artifact.Version) == +1 {
			result.Tags = append(result.Tags, t)
		}
	}

	return result
}

func (tl TagList) newerSoftwareVersions(softwarePrefix string) TagList {
	artifactTag, err := tl.Artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(tl.Artifact.SoftwareVersion), "(.*)", 1)
	regexpObject := regexp.MustCompile(pattern)

	trimmedVersion := strings.TrimPrefix(tl.Artifact.SoftwareVersion, softwarePrefix)

	result := TagList{Artifact: tl.Artifact}
	for _, t := range tl.Tags {
		version := strings.TrimPrefix(regexpObject.FindStringSubmatch(t)[1], softwarePrefix)

		if semver.Compare(version, trimmedVersion) == +1 {
			result.Tags = append(result.Tags, t)
		}
	}

	return result
}

// softwarePrefix is what separates the Version from SoftwareVersion in the tag.
// It has to be removed for the SoftwareVersion to be valid semver.
// E.g. "k3sv1.26.9-k3s1"
func (tl TagList) newerAllVersions(softwarePrefix string) TagList {
	artifactTag, err := tl.Artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}
	pattern := regexp.QuoteMeta(artifactTag)

	// Example result:
	// leap-15\.5-standard-amd64-generic-(.*?)-k3sv1.27.6-k3s1
	pattern = strings.Replace(pattern, regexp.QuoteMeta(tl.Artifact.Version), "(.*?)", 1)

	// Example result:
	// leap-15\.5-standard-amd64-generic-(.*?)-k3s(.*)
	pattern = strings.Replace(pattern,
		regexp.QuoteMeta(strings.TrimPrefix(tl.Artifact.SoftwareVersion, softwarePrefix)),
		"(.*)", 1)

	regexpObject := regexp.MustCompile(pattern)

	trimmedSVersion := strings.TrimPrefix(tl.Artifact.SoftwareVersion, softwarePrefix)

	result := TagList{Artifact: tl.Artifact}
	for _, t := range tl.Tags {
		matches := regexpObject.FindStringSubmatch(t)
		version := matches[1]
		softwareVersion := matches[2]

		versionResult := semver.Compare(version, tl.Artifact.Version)
		sVersionResult := semver.Compare(softwareVersion, trimmedSVersion)

		// If version is not lower than the current
		// and softwareVersion is not lower than the current
		// and at least one of the 2 is higher than the current
		if versionResult >= 0 && sVersionResult >= 0 && versionResult+sVersionResult > 0 {
			result.Tags = append(result.Tags, t)
		}
	}

	return result
}
