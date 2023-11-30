package versioneer

import (
	"regexp"
	"strings"
)

type TagList []string

// Images returns only tags that represent images, skipping tags representing:
// - sbom
// - att
// - sig
// - -img
func (tl *TagList) Images() *TagList {
	pattern := `.*-(core|standard)-(amd64|arm64)-.*-v.*`
	regexpObject := regexp.MustCompile(pattern)

	result := TagList{}
	for _, t := range *tl {
		// We have to filter "-img" tags outside the regexp because golang regexp
		// doesn't support negative lookaheads.
		if regexpObject.MatchString(t) && !strings.HasSuffix(t, "-img") {
			result = append(result, t)
		}
	}

	return &result
}
