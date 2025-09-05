package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

func TestDocsHandler_DocsHandler_UserEn(t *testing.T) {
	handler := NewDocsHandler()

	req := httptest.NewRequest("GET", "/docs/user?lang=en", nil)
	w := httptest.NewRecorder()

	ps := httprouter.Params{
		{Key: "type", Value: "user"},
	}

	handler.DocsHandler(w, req, ps)

	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 for user docs in English")
	assert.Contains(t, w.Body.String(), "user.en.md", "Should contain content from user.en.md")
}

func TestDocsHandler_DocsHandler_UserFr(t *testing.T) {
	handler := NewDocsHandler()

	req := httptest.NewRequest("GET", "/docs/user?lang=fr", nil)
	w := httptest.NewRecorder()

	ps := httprouter.Params{
		{Key: "type", Value: "user"},
	}

	handler.DocsHandler(w, req, ps)

	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 for user docs in French")
	assert.Contains(t, w.Body.String(), "user.fr.md", "Should contain content from user.fr.md")
}

func TestDocsHandler_DocsHandler_Admin(t *testing.T) {
	handler := NewDocsHandler()

	req := httptest.NewRequest("GET", "/docs/admin?lang=en", nil)
	w := httptest.NewRecorder()

	ps := httprouter.Params{
		{Key: "type", Value: "admin"},
	}

	handler.DocsHandler(w, req, ps)

	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 for admin docs")
}
