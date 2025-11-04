package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	us := auth.NewInMemoryUserStore()
	if err := us.AddUserPlain("admin", "admin"); err != nil {
		t.Fatalf("user: %v", err)
	}
	cfg := config.Default()
	s := NewServerWithConfig(us, cfg)
	return s
}

func TestHealthz(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHomeRequiresAuthAndShowsFooter(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Recent dstask commands") {
		t.Fatalf("footer not rendered")
	}
	if !strings.Contains(body, ">Home<") {
		t.Fatalf("navbar Home link missing")
	}
}

func TestUndoHandler(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/undo", nil)
	req.SetBasicAuth("admin", "admin")
	req.Header.Set("Referer", "/open?html=1")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	// Should redirect (even if dstask fails, the handler should redirect)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got status %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if location == "" {
		t.Fatalf("expected Location header in redirect")
	}
}

func TestHelpPage(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/help", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got status %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Help - dstask Web UI") {
		t.Fatalf("help page should contain 'Help - dstask Web UI' title")
	}
	if !strings.Contains(body, "Navigation") {
		t.Fatalf("help page should contain 'Navigation' section")
	}
	if !strings.Contains(body, "Batch Actions") {
		t.Fatalf("help page should contain 'Batch Actions' section")
	}
}

func TestLogoutHandler(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got status %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Signed Out") {
		t.Fatalf("logout page should contain 'Signed Out' title")
	}
	if !strings.Contains(body, "Sign in again") {
		t.Fatalf("logout page should contain 'Sign in again' link")
	}
	// Test that GET method is not allowed
	req2 := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req2.SetBasicAuth("admin", "admin")
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET method, got status %d", rr2.Code)
	}
}

func TestProjectsPage(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got status %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Projects") {
		t.Fatalf("projects page should contain 'Projects' title")
	}
	// Should have links to filter pages
	if !strings.Contains(body, "/open?html=1&q=project:") {
		t.Fatalf("projects page should contain filter links")
	}
}

func TestTagsPage(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/tags", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got status %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Tags") {
		t.Fatalf("tags page should contain 'Tags' title")
	}
	// Should have links to filter pages
	if !strings.Contains(body, "/open?html=1&q=+") {
		t.Fatalf("tags page should contain filter links")
	}
}

func TestTaskEditGET(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/tasks/1/edit", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	// May return 404 or 200 depending on whether task exists
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound && rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 200, 404, or 502, got status %d", rr.Code)
	}
	if rr.Code == http.StatusOK {
		body := rr.Body.String()
		if !strings.Contains(body, "Edit task") {
			t.Fatalf("edit form should contain 'Edit task' title")
		}
	}
}

func TestTaskEditPOST(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{}
	form.Set("summary", "Updated task")
	form.Set("priority", "P1")
	form.Set("project", "test-project")
	req := httptest.NewRequest(http.MethodPost, "/tasks/1/edit", strings.NewReader(form.Encode()))
	req.SetBasicAuth("admin", "admin")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	// May redirect on success or redirect to edit form on error
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got status %d", rr.Code)
	}
}

func TestBatchActionsExtended(t *testing.T) {
	s := newTestServer(t)

	tests := []struct {
		name   string
		action string
		form   map[string]string
	}{
		{"addTag", "addTag", map[string]string{"ids": "1", "action": "addTag", "tag": "testtag"}},
		{"removeTag", "removeTag", map[string]string{"ids": "1", "action": "removeTag", "tag": "testtag"}},
		{"setPriority", "setPriority", map[string]string{"ids": "1", "action": "setPriority", "priority": "P1"}},
		{"setProject", "setProject", map[string]string{"ids": "1", "action": "setProject", "project": "test-project"}},
		{"setDue", "setDue", map[string]string{"ids": "1", "action": "setDue", "due": "2025-12-31"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}
			for k, v := range tt.form {
				form.Set(k, v)
			}
			req := httptest.NewRequest(http.MethodPost, "/tasks/batch", strings.NewReader(form.Encode()))
			req.SetBasicAuth("admin", "admin")
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			s.Handler().ServeHTTP(rr, req)
			// Should redirect on success
			if rr.Code != http.StatusSeeOther {
				t.Fatalf("expected redirect (303), got status %d", rr.Code)
			}
			location := rr.Header().Get("Location")
			if location != "/open?html=1" {
				t.Fatalf("expected redirect to /open?html=1, got %s", location)
			}
		})
	}
}

func TestBatchActionsMissingParams(t *testing.T) {
	s := newTestServer(t)

	// Test missing tag for addTag
	form := url.Values{}
	form.Set("ids", "1")
	form.Set("action", "addTag")
	// tag missing
	req := httptest.NewRequest(http.MethodPost, "/tasks/batch", strings.NewReader(form.Encode()))
	req.SetBasicAuth("admin", "admin")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303) even on skipped, got status %d", rr.Code)
	}
}

func TestActiveFromPathHelp(t *testing.T) {
	if activeFromPath("/help") != "help" {
		t.Fatalf("activeFromPath /help should return 'help'")
	}
}
