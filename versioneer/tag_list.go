package versioneer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

type TagList []string

// implements sort.Interface for TagList
func (tl TagList) Len() int      { return len(tl) }
func (tl TagList) Swap(i, j int) { tl[i], tl[j] = tl[j], tl[i] }
func (tl TagList) Less(i, j int) bool {
	return tl[i] < tl[j]
}

// Images returns only tags that represent images, skipping tags representing:
// - sbom
// - att
// - sig
// - -img
func (tl TagList) Images() TagList {
	pattern := `.*-(core|standard)-(amd64|arm64)-.*-v.*`
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{}
	for _, t := range tl {
		// We have to filter "-img" tags outside the regexp because golang regexp doesn't support negative lookaheads.
		if regexpObject.MatchString(t) && !strings.HasSuffix(t, "-img") {
			result = append(result, t)
		}
	}

	return result
}

// OtherVersions returns tags that match all fields of the given Artifact,
// except the Version. Should be used to return other possible versions for the same
// Kairos image (e.g. that one could upgrade to).
// This method returns all versions, not only newer ones. Use NewerVersions to
// fetch only versions, newer than the one of the Artifact.
func (tl TagList) OtherVersions(artifact Artifact) TagList {
	return tl.fieldOtherOptions(artifact, artifact.Version)
}

// NewerVersions returns OtherVersions filtered to only include tags with
// Version higher than the given artifact's.
func (tl TagList) NewerVersions(artifact Artifact) TagList {
	tags := tl.OtherVersions(artifact)

	return tags.newerVersions(artifact)
}

// OtherSoftwareVersions returns tags that match all fields of the given Artifact,
// except the SoftwareVersion. Should be used to return other possible software versions
// for the same Kairos image (e.g. that one could upgrade to).
// This method returns all versions, not only newer ones. Use NewerSofwareVersions to
// fetch only versions, newer than the one of the Artifact.
func (tl TagList) OtherSoftwareVersions(artifact Artifact) TagList {
	return tl.fieldOtherOptions(artifact, artifact.SoftwareVersion)
}

// NewerSofwareVersions returns OtherSoftwareVersions filtered to only include tags with
// SoftwareVersion higher than the given artifact's.
func (tl TagList) NewerSofwareVersions(artifact Artifact, softwarePrefix string) TagList {
	tags := tl.OtherSoftwareVersions(artifact)

	return tags.newerSoftwareVersions(artifact, softwarePrefix)
}

// OtherAnyVersion returns tags that match all fields of the given Artifact,
// except the SoftwareVersion and/or Version.
// Should be used to return tags with newer versions (Kairos or "software")
// that one could upgrade to.
// This method returns all versions, not only newer ones. Use NewerAnyVersion to
// fetch only versions, newer than the one of the Artifact.
func (tl TagList) OtherAnyVersion(artifact Artifact) TagList {
	if artifact.SoftwareVersion != "" {
		return tl.fieldOtherOptions(artifact,
			fmt.Sprintf("%s-%s", artifact.Version, artifact.SoftwareVersion))
	} else {
		return tl.fieldOtherOptions(artifact, artifact.Version)
	}
}

// NewerAnyVersion returns OtherAnyVersion filtered to only include tags with
// Version and SoftwareVersion equal or higher than the given artifact's.
// At least one of the 2 versions will be higher than the current one.
// Splitting the 2 versions is done using the softwarePrefix (first encountered,
// because our tags have a "k3s1" in the end too)
func (tl TagList) NewerAnyVersion(artifact Artifact, softwarePrefix string) TagList {
	tags := tl.OtherAnyVersion(artifact)
	if artifact.SoftwareVersion != "" {
		return tags.newerAllVersions(artifact, softwarePrefix)
	} else {
		return tags.newerVersions(artifact)
	}
}

func (tl TagList) Print() {
	for _, t := range tl {
		fmt.Println(t)
	}
}

// Sorted returns the TagList sorted alphabetically
// This means lower versions come first.
func (tl TagList) Sorted() TagList {
	newTl := make(TagList, len(tl))
	copy(newTl, tl)
	sort.Sort(newTl)

	return newTl
}

// RSorted returns the TagList in the reverse order of Sorted
// This means higher versions come first.
func (tl TagList) RSorted() TagList {
	newTl := make(TagList, len(tl))
	copy(newTl, tl)
	sort.Sort(sort.Reverse(newTl))

	return newTl
}

func (tl TagList) fieldOtherOptions(artifact Artifact, field string) TagList {
	artifactTag, err := artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(field), ".*", 1)
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{}
	for _, t := range tl.Images() {
		if regexpObject.MatchString(t) && t != artifactTag {
			result = append(result, t)
		}
	}

	return result
}

func (tl TagList) newerVersions(artifact Artifact) TagList {
	artifactTag, err := artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(artifact.Version), "(.*)", 1)
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{}
	for _, t := range tl {
		version := regexpObject.FindStringSubmatch(t)[1]

		if semver.Compare(version, artifact.Version) == +1 {
			result = append(result, t)
		}
	}

	return result
}

func (tl TagList) newerSoftwareVersions(artifact Artifact, softwarePrefix string) TagList {
	artifactTag, err := artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(artifact.SoftwareVersion), "(.*)", 1)
	regexpObject := regexp.MustCompile(pattern)

	trimmedVersion := strings.TrimPrefix(artifact.SoftwareVersion, softwarePrefix)

	result := TagList{}
	for _, t := range tl {
		version := strings.TrimPrefix(regexpObject.FindStringSubmatch(t)[1], softwarePrefix)

		if semver.Compare(version, trimmedVersion) == +1 {
			result = append(result, t)
		}
	}

	return result
}

// softwarePrefix is what separates the Version from SoftwareVersion in the tag.
// It has to be removed for the SoftwareVersion to be valid semver.
// E.g. "k3sv1.26.9-k3s1"
func (tl TagList) newerAllVersions(artifact Artifact, softwarePrefix string) TagList {
	artifactTag, err := artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}
	pattern := regexp.QuoteMeta(artifactTag)

	// Example result:
	// leap-15\.5-standard-amd64-generic-(.*?)-k3sv1.27.6-k3s1
	pattern = strings.Replace(pattern, regexp.QuoteMeta(artifact.Version), "(.*?)", 1)

	// Example result:
	// leap-15\.5-standard-amd64-generic-(.*?)-k3s(.*)
	pattern = strings.Replace(pattern,
		regexp.QuoteMeta(strings.TrimPrefix(artifact.SoftwareVersion, softwarePrefix)),
		"(.*)", 1)

	regexpObject := regexp.MustCompile(pattern)

	trimmedSVersion := strings.TrimPrefix(artifact.SoftwareVersion, softwarePrefix)

	result := TagList{}
	for _, t := range tl {
		matches := regexpObject.FindStringSubmatch(t)
		version := matches[1]
		softwareVersion := matches[2]

		versionResult := semver.Compare(version, artifact.Version)
		sVersionResult := semver.Compare(softwareVersion, trimmedSVersion)

		// If version is not lower than the current
		// and softwareVersion is not lower than the current
		// and at least one of the 2 is higher than the current
		if versionResult >= 0 && sVersionResult >= 0 && versionResult+sVersionResult > 0 {
			result = append(result, t)
		}
	}

	return result
}
