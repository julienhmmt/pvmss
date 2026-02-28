package apiv1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginAdmin_WrongPassword(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD_HASH", "$2a$04$5oKeAhlTIjDWUd0y0/4n.OGhDyxUpExuFd.icOxoAMuDORhdebu3m")
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	h := NewAuthHandler(sm)

	body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "wrongpassword", Admin: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLoginAdmin_CorrectPassword(t *testing.T) {
	// Hash of "testpass123" generated with bcrypt MinCost for fast tests
	t.Setenv("ADMIN_PASSWORD_HASH", "$2a$04$5oKeAhlTIjDWUd0y0/4n.OGhDyxUpExuFd.icOxoAMuDORhdebu3m")
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	h := NewAuthHandler(sm)

	body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "testpass123", Admin: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	// Verify access_token cookie is set
	var found bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == accessTokenCookie {
			found = true
		}
	}
	if !found {
		t.Error("expected access_token cookie to be set")
	}
}

func TestLoginAdmin_MissingSecret(t *testing.T) {
	sm := newTestSM("") // no jwt_secret in settings
	h := NewAuthHandler(sm)

	body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "1234", Admin: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}
}

func TestLogout_ClearsCookies(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	h := NewAuthHandler(sm)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var foundAccess, foundRefresh bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == accessTokenCookie && c.MaxAge < 0 {
			foundAccess = true
		}
		if c.Name == refreshTokenCookie && c.MaxAge < 0 {
			foundRefresh = true
		}
	}
	if !foundAccess || !foundRefresh {
		t.Error("expected both token cookies to be cleared")
	}
}

func TestMe_ReturnsUsername(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newTestSM(secret)
	h := NewAuthHandler(sm)

	// Build a request with a valid JWT in context (simulating JWTMiddleware)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	signed := signToken(t, secret, "jdoe", true, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})

	// Run through JWTMiddleware first, then Me
	rr := httptest.NewRecorder()
	JWTMiddleware(sm, http.HandlerFunc(h.Me)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp MeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Username != "jdoe" || !resp.IsAdmin {
		t.Errorf("unexpected me response: %+v", resp)
	}
}
