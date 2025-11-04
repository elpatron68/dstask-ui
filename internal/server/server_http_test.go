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

func TestBatchActionWithCSRF(t *testing.T) {
	s := newTestServer(t)

	// First, get a CSRF token by making a GET request to a page that sets it
	req := httptest.NewRequest(http.MethodGet, "/open?html=1", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Extract CSRF token from cookie
	cookies := rr.Result().Cookies()
	var csrfToken string
	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			csrfToken = cookie.Value
			break
		}
	}
	if csrfToken == "" {
		t.Fatalf("expected CSRF token cookie to be set")
	}

	// Now make a batch POST request with the CSRF token
	form := url.Values{}
	form.Set("csrf_token", csrfToken)
	form.Set("action", "start")
	form["ids"] = []string{"1", "2"}

	req2 := httptest.NewRequest(http.MethodPost, "/tasks/batch", strings.NewReader(form.Encode()))
	req2.SetBasicAuth("admin", "admin")
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})

	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req2)

	// Should redirect (even if dstask fails, it should process the request)
	if rr2.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got status %d", rr2.Code)
	}
}

func TestBatchActionWithoutCSRF(t *testing.T) {
	s := newTestServer(t)

	// Make a batch POST request without CSRF token
	form := url.Values{}
	form.Set("action", "start")
	form["ids"] = []string{"1", "2"}

	req := httptest.NewRequest(http.MethodPost, "/tasks/batch", strings.NewReader(form.Encode()))
	req.SetBasicAuth("admin", "admin")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	// Should redirect with error flash message (CSRF validation failed)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got status %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/open?html=1" {
		t.Fatalf("expected redirect to /open?html=1, got %q", location)
	}
}

func TestBatchActionWithInvalidCSRF(t *testing.T) {
	s := newTestServer(t)

	// Make a batch POST request with invalid CSRF token
	form := url.Values{}
	form.Set("csrf_token", "invalid-token")
	form.Set("action", "start")
	form["ids"] = []string{"1", "2"}

	req := httptest.NewRequest(http.MethodPost, "/tasks/batch", strings.NewReader(form.Encode()))
	req.SetBasicAuth("admin", "admin")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "different-token"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	// Should redirect with error flash message (CSRF validation failed)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got status %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/open?html=1" {
		t.Fatalf("expected redirect to /open?html=1, got %q", location)
	}
}
