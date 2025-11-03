package server

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/mbusc/dstask-ui/internal/auth"
    "github.com/mbusc/dstask-ui/internal/config"
)

func newTestServer(t *testing.T) *Server {
    t.Helper()
    us := auth.NewInMemoryUserStore()
    if err := us.AddUserPlain("admin", "admin"); err != nil { t.Fatalf("user: %v", err) }
    cfg := config.Default()
    s := NewServerWithConfig(us, cfg)
    return s
}

func TestHealthz(t *testing.T) {
    s := newTestServer(t)
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rr := httptest.NewRecorder()
    s.Handler().ServeHTTP(rr, req)
    if rr.Code != http.StatusOK { t.Fatalf("status %d", rr.Code) }
    if rr.Body.String() != "ok" { t.Fatalf("unexpected body: %q", rr.Body.String()) }
}

func TestHomeRequiresAuthAndShowsFooter(t *testing.T) {
    s := newTestServer(t)
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    req.SetBasicAuth("admin", "admin")
    rr := httptest.NewRecorder()
    s.Handler().ServeHTTP(rr, req)
    if rr.Code != http.StatusOK { t.Fatalf("status %d", rr.Code) }
    body := rr.Body.String()
    if !strings.Contains(body, "Recent dstask commands") {
        t.Fatalf("footer not rendered")
    }
    if !strings.Contains(body, ">Home<") {
        t.Fatalf("navbar Home link missing")
    }
}



