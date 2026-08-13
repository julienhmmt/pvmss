// Package httpapi provides the HTTP API handlers for the PVMSS server.
//
// Package-wide constants shared by httpapi handlers.
//
// Constants are introduced incrementally: only the handlers explicitly
// migrated reference them. Other handlers keep their inline literals until
// their own migration pass to avoid cross-file merge conflicts.
package httpapi

const (
	// msgInternalServerError is the generic, sanitized message returned to
	// clients when an unexpected server-side error occurs. It never leaks the
	// underlying error (which is logged separately).
	msgInternalServerError = "internal server error"

	// codeInternalError is the stable error code paired with
	// msgInternalServerError in admin error responses.
	codeInternalError = "internal_error"
)
