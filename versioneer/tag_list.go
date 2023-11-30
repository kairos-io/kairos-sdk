package versioneer

import (
	"fmt"
	"regexp"
	"strings"
)

type TagList []string

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
		// We have to filter "-img" tags outside the regexp because golang regexp
		// doesn't support negative lookaheads.
		if regexpObject.MatchString(t) && !strings.HasSuffix(t, "-img") {
			result = append(result, t)
		}
	}

	return result
}

// OtherVersions returns tags that all fields of the given Artifact, except the
// Version. Should be used to return other possible versions for the same
// Kairos image (e.g. that one could upgrade to).
// NOTE: Returns all versions, not only newer ones.
func (tl TagList) OtherVersions(artifact Artifact) TagList {
	artifactTag, err := artifact.Tag()
	if err != nil {
		panic(fmt.Errorf("invalid artifact passed: %w", err))
	}

	pattern := regexp.QuoteMeta(artifactTag)
	pattern = strings.Replace(pattern, regexp.QuoteMeta(artifact.Version), ".*", 1)
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{}
	for _, t := range tl.Images() {
		if regexpObject.MatchString(t) && t != artifactTag {
			result = append(result, t)
		}
	}

	return result
}

// NewerVersions returns OtherVersions filtered to only include tags with
// Version higher than the given artifact's.
func (tl TagList) NewerVersions(artifact Artifact) TagList {
	// TODO:
	return tl
}

// SemverSorted returns the TagList sorted:
// - alphabetically for all non-version fields
// - Sorted by highest Version first and then by highest SoftwareVersion first
func (tl TagList) SemverSorted(artifact Artifact) TagList {
	// TODO:
	// - Implement this method
	// - Use it in all methods above to return sorted results
	// - Adapt test to check that the results are always sorted
	return tl
}
