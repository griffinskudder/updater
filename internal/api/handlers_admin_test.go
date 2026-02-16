package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"updater/internal/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAdminTemplates_ParseWithoutError(t *testing.T) {
	tmpl, err := ParseAdminTemplates()
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	expected := []string{
		"login", "health", "applications", "new-application-form",
		"application", "edit-application-form", "new-release-form",
		"flash", "app-row", "release-row",
	}
	for _, name := range expected {
		require.NotNil(t, tmpl.Lookup(name), "template %q not found", name)
	}
}

func newAdminHandlers(t *testing.T) *Handlers {
	t.Helper()
	tmpl, err := ParseAdminTemplates()
	require.NoError(t, err)
	cfg := models.SecurityConfig{
		APIKeys: []models.APIKey{
			{Key: "admin-key", Name: "test", Permissions: []string{"admin"}, Enabled: true},
		},
	}
	return NewHandlers(&MockUpdateService{},
		WithAdminTemplates(tmpl),
		WithSecurityConfig(cfg),
	)
}

func serveWithVars(h http.HandlerFunc, vars map[string]string, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req = mux.SetURLVars(req, vars)
	h(rec, req)
	return rec
}

// Login / logout

func TestAdminLogin_GET_ShowsForm(t *testing.T) {
	h := newAdminHandlers(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	h.AdminLogin(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "Admin Login")
}

func TestAdminLogin_POST_ValidKey_Redirects(t *testing.T) {
	h := newAdminHandlers(t)
	form := url.Values{"api_key": {"admin-key"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.AdminLogin(rec, req)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/applications", rec.Header().Get("Location"))
	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "admin_session" {
			assert.Equal(t, "admin-key", c.Value)
			assert.True(t, c.HttpOnly)
			found = true
		}
	}
	assert.True(t, found, "admin_session cookie not set")
}

func TestAdminLogin_POST_InvalidKey_ShowsError(t *testing.T) {
	h := newAdminHandlers(t)
	form := url.Values{"api_key": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.AdminLogin(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid")
}

func TestAdminLogout_ClearsCookieAndRedirects(t *testing.T) {
	h := newAdminHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: "admin-key"})
	rec := httptest.NewRecorder()
	h.AdminLogout(rec, req)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/login", rec.Header().Get("Location"))
	for _, c := range rec.Result().Cookies() {
		if c.Name == "admin_session" {
			assert.Equal(t, -1, c.MaxAge)
		}
	}
}

// Health

func TestAdminHealth_ReturnsPage(t *testing.T) {
	h := newAdminHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/health", nil)
	rec := httptest.NewRecorder()
	h.AdminHealth(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Service Health")
}

// Applications list and create

func TestAdminListApplications_RendersPage(t *testing.T) {
	svc := &MockUpdateService{}
	svc.On("ListApplications", mock.Anything, 50, 0).Return(&models.ListApplicationsResponse{
		Applications: []models.ApplicationSummary{},
		TotalCount:   0,
	}, nil)

	tmpl, _ := ParseAdminTemplates()
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))

	req := httptest.NewRequest(http.MethodGet, "/admin/applications", nil)
	rec := httptest.NewRecorder()
	h.AdminListApplications(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Applications")
	svc.AssertExpectations(t)
}

func TestAdminNewApplicationForm_RendersForm(t *testing.T) {
	h := newAdminHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/applications/new", nil)
	rec := httptest.NewRecorder()
	h.AdminNewApplicationForm(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "New Application")
}

func TestAdminCreateApplication_ValidInput_Redirects(t *testing.T) {
	svc := &MockUpdateService{}
	svc.On("CreateApplication", mock.Anything, mock.AnythingOfType("*models.CreateApplicationRequest")).
		Return(&models.CreateApplicationResponse{ID: "my-app", Message: "created"}, nil)

	tmpl, _ := ParseAdminTemplates()
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))

	form := url.Values{
		"id": {"my-app"}, "name": {"My App"}, "platforms": {"windows", "linux"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/applications",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.AdminCreateApplication(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/admin/applications/my-app")
	svc.AssertExpectations(t)
}

func TestAdminCreateApplication_EmptyID_ShowsError(t *testing.T) {
	h := newAdminHandlers(t)
	form := url.Values{"id": {""}, "name": {"App"}, "platforms": {"windows"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/applications",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.AdminCreateApplication(rec, req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

// Application detail, edit, delete

func TestAdminGetApplication_RendersPage(t *testing.T) {
	svc := &MockUpdateService{}
	svc.On("GetApplication", mock.Anything, "my-app").Return(&models.ApplicationInfoResponse{
		ID: "my-app", Name: "My App", Platforms: []string{"windows"},
	}, nil)
	svc.On("ListReleases", mock.Anything, mock.AnythingOfType("*models.ListReleasesRequest")).
		Return(&models.ListReleasesResponse{}, nil)

	tmpl, _ := ParseAdminTemplates()
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))

	req := httptest.NewRequest(http.MethodGet, "/admin/applications/my-app", nil)
	rec := serveWithVars(h.AdminGetApplication, map[string]string{"app_id": "my-app"}, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "My App")
	svc.AssertExpectations(t)
}

func TestAdminDeleteApplication_Success_ReturnsOK(t *testing.T) {
	svc := &MockUpdateService{}
	svc.On("DeleteApplication", mock.Anything, "my-app").Return(nil)

	tmpl, _ := ParseAdminTemplates()
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))

	req := httptest.NewRequest(http.MethodDelete, "/admin/applications/my-app", nil)
	rec := serveWithVars(h.AdminDeleteApplication, map[string]string{"app_id": "my-app"}, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestAdminUpdateApplication_ValidForm_Redirects(t *testing.T) {
	svc := &MockUpdateService{}
	svc.On("UpdateApplication", mock.Anything, "my-app",
		mock.AnythingOfType("*models.UpdateApplicationRequest")).
		Return(&models.UpdateApplicationResponse{ID: "my-app", Message: "updated"}, nil)

	tmpl, _ := ParseAdminTemplates()
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))

	form := url.Values{"name": {"New Name"}, "platforms": {"windows"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/applications/my-app/edit",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveWithVars(h.AdminUpdateApplication, map[string]string{"app_id": "my-app"}, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	svc.AssertExpectations(t)
}

// Release management

func TestAdminNewReleaseForm_RendersForm(t *testing.T) {
	h := newAdminHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/applications/my-app/releases/new", nil)
	rec := serveWithVars(h.AdminNewReleaseForm, map[string]string{"app_id": "my-app"}, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Register Release")
	assert.Contains(t, rec.Body.String(), "my-app")
}

func TestAdminCreateRelease_ValidInput_Redirects(t *testing.T) {
	svc := &MockUpdateService{}
	svc.On("RegisterRelease", mock.Anything, mock.AnythingOfType("*models.RegisterReleaseRequest")).
		Return(&models.RegisterReleaseResponse{ID: "rel-1", Message: "created"}, nil)

	tmpl, _ := ParseAdminTemplates()
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))

	form := url.Values{
		"version": {"1.0.0"}, "platform": {"windows"}, "architecture": {"amd64"},
		"download_url": {"https://example.com/app.exe"},
		"checksum":     {"abc"}, "checksum_type": {"sha256"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/applications/my-app/releases",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveWithVars(h.AdminCreateRelease, map[string]string{"app_id": "my-app"}, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/admin/applications/my-app")
	svc.AssertExpectations(t)
}

func TestAdminDeleteRelease_Success_ReturnsOK(t *testing.T) {
	svc := &MockUpdateService{}
	svc.On("DeleteRelease", mock.Anything, "my-app", "1.0.0", "windows", "amd64").
		Return(&models.DeleteReleaseResponse{ID: "rel-1", Message: "deleted"}, nil)

	tmpl, _ := ParseAdminTemplates()
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))

	req := httptest.NewRequest(http.MethodDelete,
		"/admin/applications/my-app/releases/1.0.0/windows/amd64", nil)
	vars := map[string]string{
		"app_id": "my-app", "version": "1.0.0", "platform": "windows", "arch": "amd64",
	}
	rec := serveWithVars(h.AdminDeleteRelease, vars, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

// Routes

func TestAdminRoutes_LoginPublic(t *testing.T) {
	tmpl, _ := ParseAdminTemplates()
	svc := &MockUpdateService{}
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))
	router := SetupRoutes(h, &models.Config{})
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/login")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdminRoutes_ProtectedRedirectsWithoutCookie(t *testing.T) {
	tmpl, _ := ParseAdminTemplates()
	svc := &MockUpdateService{}
	h := NewHandlers(svc, WithAdminTemplates(tmpl), WithSecurityConfig(models.SecurityConfig{}))
	router := SetupRoutes(h, &models.Config{})
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(server.URL + "/admin/applications")
	require.NoError(t, err)
	resp.Body.Close()
	// No cookie → redirect to login (even in dev mode with no configured keys).
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
}
