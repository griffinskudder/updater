package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
)

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

var adminKey = models.APIKey{
	Key:         "secret",
	Name:        "test",
	Permissions: []string{"admin"},
	Enabled:     true,
}

var cfgWithKey = models.SecurityConfig{APIKeys: []models.APIKey{adminKey}}
var cfgNoKeys = models.SecurityConfig{}

func makeAdminReq(path, cookieVal string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookieVal != "" {
		req.AddCookie(&http.Cookie{Name: "admin_session", Value: cookieVal})
	}
	return req
}

func TestAdminMiddleware_NoCookie_Redirects(t *testing.T) {
	mw := adminSessionMiddleware(cfgWithKey)
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
	mw := adminSessionMiddleware(cfgWithKey)
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
	mw := adminSessionMiddleware(cfgWithKey)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/applications", "secret"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminMiddleware_SkipsLogin(t *testing.T) {
	mw := adminSessionMiddleware(cfgWithKey)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/login", ""))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminMiddleware_SkipsLogout(t *testing.T) {
	mw := adminSessionMiddleware(cfgWithKey)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, makeAdminReq("/admin/logout", ""))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIsValidAdminKey_NoKeysConfigured_AcceptsAny(t *testing.T) {
	assert.True(t, isValidAdminKey("anything", cfgNoKeys))
}

func TestIsValidAdminKey_EmptyKeyNoKeys_Rejects(t *testing.T) {
	assert.False(t, isValidAdminKey("", cfgNoKeys))
}

func TestIsValidAdminKey_ValidAdminKey(t *testing.T) {
	assert.True(t, isValidAdminKey("secret", cfgWithKey))
}

func TestIsValidAdminKey_NonAdminKey_Rejects(t *testing.T) {
	readOnlyCfg := models.SecurityConfig{APIKeys: []models.APIKey{
		{Key: "readkey", Name: "r", Permissions: []string{"read"}, Enabled: true},
	}}
	assert.False(t, isValidAdminKey("readkey", readOnlyCfg))
}

func TestIsValidAdminKey_DisabledKey_Rejects(t *testing.T) {
	disabledCfg := models.SecurityConfig{APIKeys: []models.APIKey{
		{Key: "secret", Name: "d", Permissions: []string{"admin"}, Enabled: false},
	}}
	assert.False(t, isValidAdminKey("secret", disabledCfg))
}
