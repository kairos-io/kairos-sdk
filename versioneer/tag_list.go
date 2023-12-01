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

	return tags.newerSemver(artifact, artifact.Version, "")
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
func (tl TagList) NewerSofwareVersions(artifact Artifact, trimPrefix string) TagList {
	tags := tl.OtherSoftwareVersions(artifact)

	return tags.newerSemver(artifact, artifact.SoftwareVersion, trimPrefix)
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

// stripPrefix is a hack because the k3s version in Kairos artifacts appears as
// "k3sv1.26.9-k3s1" in which the prefix "k3s" makes it an invalid semver.
func (tl TagList) newerSemver(artifact Artifact, field, stripPrefix string) TagList {
	artifactTag, err := artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(field), "(.*)", 1)
	regexpObject := regexp.MustCompile(pattern)

	trimmedField := strings.TrimPrefix(field, stripPrefix)

	result := TagList{}
	for _, t := range tl {
		version := strings.TrimPrefix(regexpObject.FindStringSubmatch(t)[1], stripPrefix)

		if semver.Compare(version, trimmedField) == +1 {
			result = append(result, t)
		}
	}

	return result
}
