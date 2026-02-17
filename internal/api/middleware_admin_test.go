package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

// newTestStore creates a MemoryStorage seeded with a single admin API key.
// The raw key value "secret" is stored as its SHA-256 hash.
func newTestStore(t *testing.T) (storage.Storage, string) {
	t.Helper()
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	rawKey := "secret"
	ak := models.NewAPIKey(models.NewKeyID(), "test", rawKey, []string{"admin"})
	err = store.CreateAPIKey(context.Background(), ak)
	require.NoError(t, err)
	return store, rawKey
}

func makeAdminReq(path, cookieVal string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookieVal != "" {
		req.AddCookie(&http.Cookie{Name: "admin_session", Value: cookieVal})
	}
	return req
}

func TestAdminMiddleware_NoCookie_Redirects(t *testing.T) {
	store, _ := newTestStore(t)
	mw := adminSessionMiddleware(store, true)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/applications", ""))
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/login", rec.Header().Get("Location"))
	for _, c := range rec.Result().Cookies() {
		if c.Name == "admin_session" {
			assert.Equal(t, -1, c.MaxAge)
			assert.True(t, c.HttpOnly, "clearing cookie must be HttpOnly")
			assert.True(t, c.Secure, "clearing cookie must carry Secure flag")
			assert.Equal(t, http.SameSiteStrictMode, c.SameSite, "clearing cookie must be SameSite=Strict")
		}
	}
}

func TestAdminMiddleware_InvalidKey_Redirects(t *testing.T) {
	store, _ := newTestStore(t)
	mw := adminSessionMiddleware(store, true)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/applications", "wrong"))
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "admin_session" {
			assert.Equal(t, -1, c.MaxAge)
			assert.True(t, c.HttpOnly, "clearing cookie must be HttpOnly")
			assert.True(t, c.Secure, "clearing cookie must carry Secure flag")
			assert.Equal(t, http.SameSiteStrictMode, c.SameSite, "clearing cookie must be SameSite=Strict")
		}
	}
}

func TestAdminMiddleware_ValidKey_Passes(t *testing.T) {
	store, rawKey := newTestStore(t)
	mw := adminSessionMiddleware(store, true)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/applications", rawKey))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminMiddleware_SkipsLogin(t *testing.T) {
	store, _ := newTestStore(t)
	mw := adminSessionMiddleware(store, true)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/login", ""))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminMiddleware_SkipsLogout(t *testing.T) {
	store, _ := newTestStore(t)
	mw := adminSessionMiddleware(store, true)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/logout", ""))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminMiddleware_DevMode_AnyKeyPasses(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	// enableAuth=false => dev mode: any non-empty key is accepted.
	mw := adminSessionMiddleware(store, false)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/applications", "any-key"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIsValidAdminKey_DevMode_AcceptsAny(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	assert.True(t, isValidAdminKey(context.Background(), "anything", store, false))
}

func TestIsValidAdminKey_EmptyKey_Rejects(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	assert.False(t, isValidAdminKey(context.Background(), "", store, false))
}

func TestIsValidAdminKey_ValidAdminKey(t *testing.T) {
	store, rawKey := newTestStore(t)
	assert.True(t, isValidAdminKey(context.Background(), rawKey, store, true))
}

func TestIsValidAdminKey_NonAdminKey_Rejects(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	rawKey := "readkey"
	ak := models.NewAPIKey(models.NewKeyID(), "r", rawKey, []string{"read"})
	err = store.CreateAPIKey(context.Background(), ak)
	require.NoError(t, err)
	assert.False(t, isValidAdminKey(context.Background(), rawKey, store, true))
}

func TestIsValidAdminKey_DisabledKey_Rejects(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	rawKey := "disabledkey"
	ak := models.NewAPIKey(models.NewKeyID(), "d", rawKey, []string{"admin"})
	ak.Enabled = false
	ak.UpdatedAt = time.Now()
	err = store.CreateAPIKey(context.Background(), ak)
	require.NoError(t, err)
	assert.False(t, isValidAdminKey(context.Background(), rawKey, store, true))
}
