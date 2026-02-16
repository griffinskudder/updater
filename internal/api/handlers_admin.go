package api

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"updater/internal/models"

	"github.com/gorilla/mux"
)

//go:embed admin/templates
var adminTemplateFS embed.FS

// ParseAdminTemplates parses all admin templates from the embedded FS.
func ParseAdminTemplates() (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"hasPlatform": func(platforms []string, p string) bool {
			for _, pl := range platforms {
				if pl == p {
					return true
				}
			}
			return false
		},
	}).ParseFS(adminTemplateFS,
		"admin/templates/*.html",
		"admin/templates/partials/*.html",
	)
}

// adminBaseData is embedded in every page data struct.
type adminBaseData struct {
	Flash *adminFlashData
}

type adminFlashData struct {
	Type    string // "success" or "error"
	Message string
}

type adminLoginData struct {
	Error string
}

type adminApplicationsData struct {
	adminBaseData
	Applications *models.ListApplicationsResponse
	Platforms    []string
}

type adminNewAppData struct {
	adminBaseData
	Error     string
	Form      models.CreateApplicationRequest
	Platforms []string
}

type adminApplicationData struct {
	adminBaseData
	Application *models.ApplicationInfoResponse
	Releases    *models.ListReleasesResponse
	Platforms   []string
	Error       string
}

type adminEditAppData struct {
	adminBaseData
	Application *models.ApplicationInfoResponse
	Platforms   []string
	Error       string
}

type adminReleaseFormData struct {
	adminBaseData
	AppID         string
	Platforms     []string
	Architectures []string
	Error         string
}

type adminHealthData struct {
	adminBaseData
	Health *models.HealthCheckResponse
}

var allPlatforms = []string{"windows", "linux", "darwin", "android", "ios"}
var allArchitectures = []string{"amd64", "arm64", "386", "arm"}

// renderAdmin renders a named template, writing 500 on error.
func (h *Handlers) renderAdmin(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.adminTmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("admin template error", "template", name, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// flashFromQuery reads a ?flash=<msg>&flash_type=<type> query pair into adminFlashData.
func flashFromQuery(r *http.Request) *adminFlashData {
	msg := r.URL.Query().Get("flash")
	if msg == "" {
		return nil
	}
	ft := r.URL.Query().Get("flash_type")
	if ft == "" {
		ft = "success"
	}
	return &adminFlashData{Type: ft, Message: msg}
}

// addFlash appends flash query params to a redirect URL.
func addFlash(base, msg, flashType string) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "flash=" + msg + "&flash_type=" + flashType
}

// adminPathVar extracts a named path variable from the request using gorilla/mux.
func adminPathVar(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}

// AdminLogin handles GET (show form) and POST (submit key) for /admin/login.
func (h *Handlers) AdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.renderAdmin(w, "login", adminLoginData{})
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderAdmin(w, "login", adminLoginData{Error: "Invalid form submission"})
		return
	}
	key := r.FormValue("api_key")
	if !isValidAdminKey(key, h.securityConfig) {
		w.WriteHeader(http.StatusUnauthorized)
		h.renderAdmin(w, "login", adminLoginData{Error: "Invalid API key or insufficient permissions"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    key,
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/admin/applications", http.StatusSeeOther)
}

// AdminLogout clears the session cookie and redirects to login.
func (h *Handlers) AdminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "admin_session",
		Value:  "",
		Path:   "/admin",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// AdminHealth renders the health dashboard.
func (h *Handlers) AdminHealth(w http.ResponseWriter, r *http.Request) {
	resp := models.NewHealthCheckResponse(models.StatusHealthy)
	resp.Version = "1.0.0"

	storageStatus := models.StatusHealthy
	storageMsg := "Storage is operational"
	if h.storage != nil {
		if err := h.storage.Ping(r.Context()); err != nil {
			storageStatus = models.StatusUnhealthy
			storageMsg = err.Error()
			resp.Status = models.StatusDegraded
		}
	}
	resp.AddComponent("storage", storageStatus, storageMsg)
	resp.AddComponent("api", models.StatusHealthy, "API is operational")

	h.renderAdmin(w, "health", adminHealthData{Health: resp})
}

// AdminListApplications renders the applications list page.
func (h *Handlers) AdminListApplications(w http.ResponseWriter, r *http.Request) {
	resp, err := h.updateService.ListApplications(r.Context(), 50, 0)
	if err != nil {
		h.renderAdmin(w, "applications", adminApplicationsData{
			adminBaseData: adminBaseData{Flash: &adminFlashData{Type: "error", Message: err.Error()}},
			Platforms:     allPlatforms,
		})
		return
	}
	h.renderAdmin(w, "applications", adminApplicationsData{
		adminBaseData: adminBaseData{Flash: flashFromQuery(r)},
		Applications:  resp,
		Platforms:     allPlatforms,
	})
}

// AdminNewApplicationForm renders the create-application form.
func (h *Handlers) AdminNewApplicationForm(w http.ResponseWriter, r *http.Request) {
	h.renderAdmin(w, "new-application-form", adminNewAppData{Platforms: allPlatforms})
}

// AdminCreateApplication processes the create-application form.
func (h *Handlers) AdminCreateApplication(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		h.renderAdmin(w, "new-application-form", adminNewAppData{Error: "Invalid form", Platforms: allPlatforms})
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	name := strings.TrimSpace(r.FormValue("name"))
	if id == "" || name == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderAdmin(w, "new-application-form", adminNewAppData{
			Error:     "ID and Name are required",
			Form:      models.CreateApplicationRequest{ID: id, Name: name},
			Platforms: allPlatforms,
		})
		return
	}

	platforms := r.Form["platforms"]
	if len(platforms) == 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderAdmin(w, "new-application-form", adminNewAppData{
			Error:     "At least one platform is required",
			Form:      models.CreateApplicationRequest{ID: id, Name: name},
			Platforms: allPlatforms,
		})
		return
	}

	req := &models.CreateApplicationRequest{
		ID:          id,
		Name:        name,
		Description: r.FormValue("description"),
		Platforms:   platforms,
	}
	resp, err := h.updateService.CreateApplication(r.Context(), req)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderAdmin(w, "new-application-form", adminNewAppData{
			Error: err.Error(), Form: *req, Platforms: allPlatforms,
		})
		return
	}

	http.Redirect(w, r, addFlash("/admin/applications/"+resp.ID, "Application created", "success"),
		http.StatusSeeOther)
}

// AdminGetApplication renders the application detail page with its releases.
func (h *Handlers) AdminGetApplication(w http.ResponseWriter, r *http.Request) {
	appID := adminPathVar(r, "app_id")
	app, err := h.updateService.GetApplication(r.Context(), appID)
	if err != nil {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	releases, err := h.updateService.ListReleases(r.Context(), &models.ListReleasesRequest{
		ApplicationID: appID, Limit: 100,
	})
	if err != nil {
		releases = &models.ListReleasesResponse{}
	}
	h.renderAdmin(w, "application", adminApplicationData{
		adminBaseData: adminBaseData{Flash: flashFromQuery(r)},
		Application:   app,
		Releases:      releases,
		Platforms:     allPlatforms,
	})
}

// AdminEditApplicationForm renders the edit form pre-filled with current values.
func (h *Handlers) AdminEditApplicationForm(w http.ResponseWriter, r *http.Request) {
	appID := adminPathVar(r, "app_id")
	app, err := h.updateService.GetApplication(r.Context(), appID)
	if err != nil {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	h.renderAdmin(w, "edit-application-form", adminEditAppData{
		Application: app, Platforms: allPlatforms,
	})
}

// AdminUpdateApplication processes the edit form (POST /admin/applications/{id}/edit).
func (h *Handlers) AdminUpdateApplication(w http.ResponseWriter, r *http.Request) {
	appID := adminPathVar(r, "app_id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	platforms := r.Form["platforms"]
	if name == "" || len(platforms) == 0 {
		app, _ := h.updateService.GetApplication(r.Context(), appID)
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderAdmin(w, "edit-application-form", adminEditAppData{
			Application: app, Platforms: allPlatforms,
			Error: "Name and at least one platform are required",
		})
		return
	}
	desc := r.FormValue("description")
	req := &models.UpdateApplicationRequest{
		Name:        &name,
		Description: &desc,
		Platforms:   platforms,
	}
	if _, err := h.updateService.UpdateApplication(r.Context(), appID, req); err != nil {
		app, _ := h.updateService.GetApplication(r.Context(), appID)
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderAdmin(w, "edit-application-form", adminEditAppData{
			Application: app, Platforms: allPlatforms, Error: err.Error(),
		})
		return
	}
	http.Redirect(w, r, addFlash("/admin/applications/"+appID, "Application updated", "success"),
		http.StatusSeeOther)
}

// AdminDeleteApplication deletes the application and returns 200 OK (HTMX removes the row).
func (h *Handlers) AdminDeleteApplication(w http.ResponseWriter, r *http.Request) {
	appID := adminPathVar(r, "app_id")
	if err := h.updateService.DeleteApplication(r.Context(), appID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// AdminNewReleaseForm renders the release registration form.
func (h *Handlers) AdminNewReleaseForm(w http.ResponseWriter, r *http.Request) {
	h.renderAdmin(w, "new-release-form", adminReleaseFormData{
		AppID: adminPathVar(r, "app_id"), Platforms: allPlatforms, Architectures: allArchitectures,
	})
}

// AdminCreateRelease processes the release registration form.
func (h *Handlers) AdminCreateRelease(w http.ResponseWriter, r *http.Request) {
	appID := adminPathVar(r, "app_id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	req := &models.RegisterReleaseRequest{
		ApplicationID:  appID,
		Version:        strings.TrimSpace(r.FormValue("version")),
		Platform:       r.FormValue("platform"),
		Architecture:   r.FormValue("architecture"),
		DownloadURL:    strings.TrimSpace(r.FormValue("download_url")),
		Checksum:       strings.TrimSpace(r.FormValue("checksum")),
		ChecksumType:   r.FormValue("checksum_type"),
		ReleaseNotes:   r.FormValue("release_notes"),
		MinimumVersion: r.FormValue("minimum_version"),
		Required:       r.FormValue("required") == "on",
	}
	if req.Version == "" || req.Platform == "" || req.Architecture == "" ||
		req.DownloadURL == "" || req.Checksum == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderAdmin(w, "new-release-form", adminReleaseFormData{
			AppID: appID, Platforms: allPlatforms, Architectures: allArchitectures,
			Error: "Version, Platform, Architecture, Download URL, and Checksum are required",
		})
		return
	}
	if _, err := h.updateService.RegisterRelease(r.Context(), req); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderAdmin(w, "new-release-form", adminReleaseFormData{
			AppID: appID, Platforms: allPlatforms, Architectures: allArchitectures,
			Error: err.Error(),
		})
		return
	}
	http.Redirect(w, r, addFlash("/admin/applications/"+appID, "Release registered", "success"),
		http.StatusSeeOther)
}

// AdminDeleteRelease deletes a release and returns 200 OK (HTMX removes the row).
func (h *Handlers) AdminDeleteRelease(w http.ResponseWriter, r *http.Request) {
	if _, err := h.updateService.DeleteRelease(r.Context(),
		adminPathVar(r, "app_id"),
		adminPathVar(r, "version"),
		adminPathVar(r, "platform"),
		adminPathVar(r, "arch"),
	); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
