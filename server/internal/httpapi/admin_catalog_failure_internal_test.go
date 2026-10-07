package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"strings"
	"testing"
)

func TestWriteAdminFailure_MapsClusterAndInternalFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("list: %w", cluster.ErrUnreachable), http.StatusServiceUnavailable, "cluster_unavailable"},
		{fmt.Errorf("list: %w", cluster.ErrTLSVerify), http.StatusServiceUnavailable, "cluster_unavailable"},
		{errors.New("db down"), http.StatusInternalServerError, codeInternalError},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		writeAdminFailure(rec, "admin list isos failed", tc.err)

		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.code) {
			t.Errorf("%v -> %d %s, want %d %s", tc.err, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
	}
}
