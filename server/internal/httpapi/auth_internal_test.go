package httpapi

import (
	"context"
	"pvmss/server/internal/cluster"
	"slices"
	"testing"
)

func TestWithPoolPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		username string
		want     string
		wantOK   bool
	}{
		{name: "bare pool user", username: "jho@pve", want: "pvmss-jho@pve", wantOK: true},
		{name: "already prefixed", username: "pvmss-jho@pve", want: "", wantOK: false},
		{name: "other realm preserved", username: "jho@pam", want: "pvmss-jho@pam", wantOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := withPoolPrefix(tc.username)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("withPoolPrefix(%q) = (%q, %v), want (%q, %v)", tc.username, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

const (
	typedUser = "jho@pve"
	poolUser  = "pvmss-jho@pve"
)

// authStub accepts one username and records every attempt, in order.
type authStub struct {
	cluster.Client

	valid string
	tried []string
}

func (s *authStub) Authenticate(_ context.Context, username, _ string) (cluster.Identity, error) {
	s.tried = append(s.tried, username)
	if username != s.valid {
		return cluster.Identity{}, cluster.ErrNotFound
	}

	return cluster.Identity{Username: username}, nil
}

func TestAuthenticatePVE_AttemptOrder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		valid     string
		poolFirst bool
		want      []string
	}{
		// A managed pool user must not pay Proxmox's failed-login delay first.
		{name: "managed pool tries prefixed first", valid: poolUser, poolFirst: true, want: []string{poolUser}},
		{name: "managed pool falls back to typed", valid: typedUser, poolFirst: true, want: []string{poolUser, typedUser}},
		{name: "unmanaged tries typed first", valid: typedUser, poolFirst: false, want: []string{typedUser}},
		{name: "unmanaged falls back to prefixed", valid: poolUser, poolFirst: false, want: []string{typedUser, poolUser}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := &authStub{valid: tc.valid}

			got, err := authenticatePVE(context.Background(), stub, typedUser, "pw", tc.poolFirst)
			if err != nil || got.Username != tc.valid {
				t.Fatalf("authenticatePVE = (%+v, %v), want user %q", got, err, tc.valid)
			}

			if !slices.Equal(stub.tried, tc.want) {
				t.Errorf("tried %v, want %v", stub.tried, tc.want)
			}
		})
	}
}
