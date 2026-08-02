package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pvmss/server/internal/httpapi"
)

func TestRouter_SPAFallback(t *testing.T) {
	dir := t.TempDir()
	html := `<!doctype html><html lang="en"><head><title>PVMSS</title></head><body>shell</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("real file"), 0644); err != nil {
		t.Fatalf("write file.txt: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a-dir"), 0755); err != nil {
		t.Fatalf("create a-dir: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	health := httpapi.NewHealth(fakePinger{}, logger)
	mux := httpapi.NewRouter(health, dir, logger)

	cases := []struct {
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{http.MethodGet, "/", http.StatusOK, "shell"},
		{http.MethodGet, "/anything-that-does-not-exist", http.StatusOK, "shell"},
		{http.MethodGet, "/file.txt", http.StatusOK, "real file"},
		{http.MethodGet, "/missing.txt", http.StatusNotFound, "asset not found"},
		{http.MethodGet, "/a-dir", http.StatusOK, "shell"},
		{http.MethodGet, "/api/unknown", http.StatusNotFound, "unknown API path"},
		{http.MethodPost, "/api/unknown", http.StatusNotFound, "unknown API path"},
	}

	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(c.method, c.path, nil)
			mux.ServeHTTP(w, r)

			if w.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, c.wantStatus)
			}
			body, _ := io.ReadAll(w.Result().Body)
			if !strings.Contains(string(body), c.wantBody) {
				t.Fatalf("body = %q, want to contain %q", string(body), c.wantBody)
			}
		})
	}
}

func TestRouter_MissingBuildDir_HealthStillWorks(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	health := httpapi.NewHealth(fakePinger{}, logger)
	mux := httpapi.NewRouter(health, "", logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", w.Code, http.StatusOK)
	}
	var got struct {
		Status    string                         `json:"status"`
		Checks    map[string]httpapi.CheckResult `json:"checks"`
		Timestamp string                         `json:"timestamp"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if got.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", got.Status)
	}
}
