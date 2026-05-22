// Package apiv1 — shared input validators.
//
// All regexes used by request validation live here as package-level vars so
// they compile once at startup, not per-request. Validators return typed
// *errors.ValidationError so handlers can:
//
//   - branch with errors.Is/errors.As against the validation sentinel
//   - rely on writeAppError to surface the message as 400 bad-request
package apiv1

import (
	"regexp"

	pverrors "pvmss/errors"
)

// Shared regexes (compiled once at startup).
var (
	cloudInitIDUnsafeRegex = regexp.MustCompile(`[^a-z0-9-]`)
)

// validateTagName checks that name matches the tag-name format (letters,
// digits, hyphens, underscores; 1..50 chars). Returns a *ValidationError on
// failure.
func validateTagName(name string) error {
	if !tagNameRegex.MatchString(name) {
		return pverrors.ValidationErr(
			"name",
			name,
			"use only letters, digits, hyphens, underscores (max 50 chars)",
		)
	}
	return nil
}

