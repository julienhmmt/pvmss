package cloudinit

import (
	"regexp"
	"strings"
)

// slugRe strips non-alphanumeric characters for slug derivation.
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a label to a lowercase hyphenated slug (e.g. "Web server"
// → "web-server"), falling back when nothing alphanumeric remains. Shared by
// catalog id derivation and user cloud-init file ids - it lives here, not in
// catalog, because catalog already imports cloudinit (cloudinit → catalog
// would be a cycle).
func Slugify(label, fallback string) string {
	slug := slugRe.ReplaceAllString(strings.ToLower(label), "-")

	slug = strings.Trim(slug, "-")
	if slug == "" {
		return fallback
	}

	return slug
}
