// Package proxmox — error translation helpers.
//
// All HTTP-level and transport-level errors raised by RestyClient are
// translated into PVMSS domain errors here. Two goals:
//
//   - Callers in api/v1 / handlers can branch on `errors.Is(err, errors.ErrNotFound)`
//     (and friends) without knowing they are talking to Proxmox.
//   - The wrapping rule (Preslav #7) is respected: third-party resty errors
//     are folded in via `%v` (no API promise), domain sentinels via `%w`.
package proxmox

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	pverrors "pvmss/errors"
)

// translateStatusErr converts a non-2xx Proxmox response into a domain
// AppError whose chain contains the appropriate sentinel and whose message
// carries the (trimmed) response body so handlers can surface it to clients.
// Returns nil for status < 400.
func translateStatusErr(method, path string, status int, body string) error {
	if status < 400 {
		return nil
	}
	code, sentinel := codeForStatus(status)
	msg := strings.TrimSpace(body)
	if msg == "" {
		msg = fmt.Sprintf("proxmox %s %s returned %d", method, path, status)
	}
	appErr := pverrors.AppErr(code, msg)
	if sentinel != nil {
		appErr.Err = sentinel
	}
	return appErr
}

// translateTransportErr wraps a resty/network-level error. The foreign error
// is folded in via %v (per Preslav #7) and a domain sentinel attached via %w
// so callers can branch with errors.Is.
func translateTransportErr(method, path string, err error) error {
	if err == nil {
		return nil
	}
	sentinel := pverrors.ErrUnavailable
	if stderrors.Is(err, context.DeadlineExceeded) || stderrors.Is(err, context.Canceled) {
		sentinel = pverrors.ErrTimeout
	}
	return fmt.Errorf("calling proxmox %s %s: %v: %w", method, path, err, sentinel)
}

// codeForStatus maps an HTTP status to a PVMSS error code + matching sentinel.
func codeForStatus(status int) (pverrors.ErrorCode, error) {
	switch status {
	case http.StatusNotFound:
		return pverrors.CodeNotFound, pverrors.ErrNotFound
	case http.StatusConflict:
		return pverrors.CodeConflict, pverrors.ErrConflict
	case http.StatusUnauthorized, http.StatusForbidden:
		return pverrors.CodeUnauthorized, pverrors.ErrUnauthorized
	case http.StatusTooManyRequests:
		return pverrors.CodeRateLimited, pverrors.ErrRateLimited
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return pverrors.CodeTimeout, pverrors.ErrTimeout
	case http.StatusServiceUnavailable:
		return pverrors.CodeUnavailable, pverrors.ErrUnavailable
	}
	if status >= 400 && status < 500 {
		return pverrors.CodeValidation, pverrors.ErrValidation
	}
	return pverrors.CodeProxmox, nil
}
