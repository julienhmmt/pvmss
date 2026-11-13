package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// OpenIDAuthenticator drives a Proxmox OpenID Connect realm on PVMSS's behalf
// (issue #159): Proxmox talks to the IdP and verifies the ID token, PVMSS only
// relays the browser's authorization code. Optional: callers type-assert it.
type OpenIDAuthenticator interface {
	// OpenIDAuthURL returns the IdP authorization URL and the state Proxmox
	// embedded in it, which the IdP echoes back to the callback.
	OpenIDAuthURL(ctx context.Context, realm, redirectURL string) (authURL, state string, err error)
	// OpenIDLogin exchanges the callback's code and state for an identity.
	// A rejected exchange is ErrNotFound, like a wrong password.
	OpenIDLogin(ctx context.Context, code, state, redirectURL string) (Identity, error)
}

// OpenIDAuthURL implements OpenIDAuthenticator (POST /access/openid/auth-url).
func (p Proxmox) OpenIDAuthURL(ctx context.Context, realm, redirectURL string) (string, string, error) {
	var authURL string
	if err := proxmoxPublicPost(ctx, p.rest(), "/access/openid/auth-url", url.Values{"realm": {realm}, "redirect-url": {redirectURL}}, &authURL); err != nil {
		return "", "", err
	}

	parsed, err := url.Parse(authURL)
	if err != nil || parsed.Query().Get("state") == "" {
		return "", "", fmt.Errorf("proxmox openid auth-url: no state in %q", parsed.Redacted())
	}

	return authURL, parsed.Query().Get("state"), nil
}

// OpenIDLogin implements OpenIDAuthenticator (POST /access/openid/login),
// then resolves admin status and pool with the returned ticket exactly as
// Authenticate does.
func (p Proxmox) OpenIDLogin(ctx context.Context, code, state, redirectURL string) (Identity, error) {
	rest := p.rest()

	var login struct {
		Ticket              string `json:"ticket"`
		CSRFPreventionToken string `json:"CSRFPreventionToken"`
		Username            string `json:"username"`
	}
	if err := proxmoxPublicPost(ctx, rest, "/access/openid/login", url.Values{"code": {code}, "state": {state}, "redirect-url": {redirectURL}}, &login); err != nil {
		return Identity{}, err
	}

	if login.Ticket == "" || login.Username == "" {
		return Identity{}, ErrNotFound
	}

	isAdmin, err := proxmoxHasPermission(ctx, rest.withTicket(login.Ticket, login.CSRFPreventionToken), "/", "Permissions.Modify")
	if err != nil {
		return Identity{}, err
	}

	var pool string
	if !isAdmin {
		// A failed lookup fails the login: "no pool" makes the caller create one.
		pool, err = proxmoxACLPool(ctx, rest, login.Username)
		if err != nil {
			return Identity{}, err
		}
	}

	return Identity{Username: login.Username, Pool: pool, IsAdmin: isAdmin}, nil
}

// proxmoxPublicPost sends an unauthenticated form POST (the openid endpoints
// are "user: world" and must not carry the service token) and decodes the
// response's data field into out. 4xx is ErrNotFound: Proxmox rejected the
// code, state or realm, and the caller must not learn which.
func proxmoxPublicPost(ctx context.Context, rest proxmoxRESTClient, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rest.base+path, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := rest.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError {
		return fmt.Errorf("%w: proxmox %s: HTTP %d", ErrNotFound, path, resp.StatusCode)
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("proxmox %s: HTTP %d", path, resp.StatusCode)
	}

	envelope := struct {
		Data any `json:"data"`
	}{Data: out}

	return decodeJSONBody(resp, &envelope)
}
