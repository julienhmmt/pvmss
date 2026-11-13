package cluster

import (
	"context"
	"net/url"
)

// FakeOpenIDCode is the only authorization code the fake's OpenID realm
// accepts; it signs in as FakeUserAlice. Demo and tests only.
const FakeOpenIDCode = "fake-oidc-code"

const fakeOpenIDState = "fake-oidc-state"

// OpenIDAuthURL implements OpenIDAuthenticator with a deterministic URL that
// points straight back at the callback, so the demo needs no IdP.
func (fake Fake) OpenIDAuthURL(_ context.Context, _, redirectURL string) (string, string, error) {
	if fake.unavailable() {
		return "", "", ErrUnreachable
	}

	query := url.Values{"code": {FakeOpenIDCode}, "state": {fakeOpenIDState}}

	return redirectURL + "?" + query.Encode(), fakeOpenIDState, nil
}

// OpenIDLogin implements OpenIDAuthenticator against the demo identities.
func (fake Fake) OpenIDLogin(_ context.Context, code, state, _ string) (Identity, error) {
	if fake.unavailable() {
		return Identity{}, ErrUnreachable
	}

	if code != FakeOpenIDCode || state != fakeOpenIDState {
		return Identity{}, ErrNotFound
	}

	st := fake.stateOrDefault()

	st.identMu.RLock()
	defer st.identMu.RUnlock()

	identity := st.identities[FakeUserAlice]

	return Identity{Username: FakeUserAlice, Pool: identity.pool, IsAdmin: identity.isAdmin}, nil
}
