package apiv1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "pvmss/api/v1"
	"pvmss/database"
	"pvmss/state"
)

// newTagStateDB builds a StateManager backed by a real in-memory DB (HasDB
// true) so CreateTag/DeleteTag exercise the SetTags audit path.
func newTagStateDB(t *testing.T) state.StateManager {
	t.Helper()
	db, err := database.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.CompleteBootstrap("test"))
	sm := state.MakeAppStateWithDB(db)
	require.NoError(t, sm.LoadSettingsFromDB())
	return sm
}

// newTagStateNoDB builds a StateManager with no DB (HasDB false) so
// CreateTag/DeleteTag exercise the in-memory SetSettings path.
func newTagStateNoDB(t *testing.T) state.StateManager {
	t.Helper()
	return state.MakeAppState()
}

// nameParamRequest injects the httprouter `:name` param the way the router does
// at runtime, so DeleteTag resolves it from context.
func nameParamRequest(method, target, name string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	ctx := context.WithValue(r.Context(), httprouter.ParamsKey, httprouter.Params{
		{Key: "name", Value: name},
	})
	return r.WithContext(ctx)
}

func createTagBody(name string) *bytes.Buffer {
	b, _ := json.Marshal(apiv1.CreateTagRequest{Name: name})
	return bytes.NewBuffer(b)
}

// ── CreateTag ───────────────────────────────────────────────────────────────

func TestCreateTag_DB_PersistsAndReturns201(t *testing.T) {
	sm := newTagStateDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", createTagBody("web"))
	h.CreateTag(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, sm.GetTags(), "web", "DB path must persist the new tag")
}

func TestCreateTag_NoDB_PersistsAndReturns201(t *testing.T) {
	sm := newTagStateNoDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", createTagBody("web"))
	h.CreateTag(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, sm.GetTags(), "web", "in-memory path must persist the new tag")
}

func TestCreateTag_Duplicate_ReturnsBadRequest(t *testing.T) {
	sm := newTagStateDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	first := httptest.NewRecorder()
	h.CreateTag(first, httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", createTagBody("dup")))
	require.Equal(t, http.StatusCreated, first.Code)

	w := httptest.NewRecorder()
	h.CreateTag(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", createTagBody("dup")))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "already exists")
}

func TestCreateTag_InvalidName_ReturnsBadRequest(t *testing.T) {
	sm := newTagStateDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	w := httptest.NewRecorder()
	h.CreateTag(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", createTagBody("bad tag!")))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── DeleteTag ───────────────────────────────────────────────────────────────

func TestDeleteTag_DB_RemovesAndReturns204(t *testing.T) {
	sm := newTagStateDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	create := httptest.NewRecorder()
	h.CreateTag(create, httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", createTagBody("web")))
	require.Equal(t, http.StatusCreated, create.Code)

	w := httptest.NewRecorder()
	h.DeleteTag(w, nameParamRequest(http.MethodDelete, "/api/v1/admin/tags/web", "web"))
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.NotContains(t, sm.GetTags(), "web", "DB path must remove the tag")
}

func TestDeleteTag_NoDB_RemovesAndReturns204(t *testing.T) {
	sm := newTagStateNoDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	create := httptest.NewRecorder()
	h.CreateTag(create, httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", createTagBody("web")))
	require.Equal(t, http.StatusCreated, create.Code)

	w := httptest.NewRecorder()
	h.DeleteTag(w, nameParamRequest(http.MethodDelete, "/api/v1/admin/tags/web", "web"))
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.NotContains(t, sm.GetTags(), "web", "in-memory path must remove the tag")
}

func TestDeleteTag_RequiredPvmssTag_ReturnsBadRequest(t *testing.T) {
	sm := newTagStateDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	w := httptest.NewRecorder()
	h.DeleteTag(w, nameParamRequest(http.MethodDelete, "/api/v1/admin/tags/pvmss", "pvmss"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "pvmss")
}

func TestDeleteTag_NotFound_ReturnsBadRequest(t *testing.T) {
	sm := newTagStateDB(t)
	h := apiv1.MakeAdminMutationsHandler(sm)

	w := httptest.NewRecorder()
	h.DeleteTag(w, nameParamRequest(http.MethodDelete, "/api/v1/admin/tags/ghost", "ghost"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}
