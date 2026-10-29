package httpapi

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/coder/websocket"
)

func TestIsNormalClose(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, true},
		{"eof", fmt.Errorf("read: %w", io.EOF), true},
		{"normal closure", websocket.CloseError{Code: websocket.StatusNormalClosure}, true},
		{"going away", websocket.CloseError{Code: websocket.StatusGoingAway}, true},
		{"abnormal closure", websocket.CloseError{Code: websocket.StatusAbnormalClosure}, false},
		{"policy violation", websocket.CloseError{Code: websocket.StatusPolicyViolation}, false},
		{"internal error", websocket.CloseError{Code: websocket.StatusInternalError}, false},
		{"unrelated error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isNormalClose(tc.err); got != tc.want {
				t.Fatalf("isNormalClose(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
