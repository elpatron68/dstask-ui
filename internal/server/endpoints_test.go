package server

import (
	"os"
	"path/filepath"
	"testing"
	"net/http/httptest"
	"strings"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
)

// createDstaskStub erzeugt ein Shell-Script, das je nach Subcommand vordefinierte Ausgaben liefert.
func createDstaskStub(t *testing.T, dir string) string {
	t.Helper()
	stub := filepath.Join(dir, "dstask-stub.sh")
	content := `#!/usr/bin/env bash
subcmd="$1"
case "$subcmd" in
  show-projects)
    cat <<'JSON'
[
  {"name":"alpha","taskCount":3,"resolvedCount":1,"active":1,"priority":"P2"},
  {"name":"beta","taskCount":5,"resolvedCount":0,"active":2,"priority":"P1"}
]
JSON
    ;;
  show-tags)
    echo "+ui"
    echo "+backend"
    ;;
  show-templates)
    cat <<'JSON'
[
  {"id": "1", "summary": "Template A", "project": "alpha", "tags": ["ui"], "due": "2025-12-31"},
  {"id": "2", "summary": "Template B", "project": "beta", "tags": ["backend"], "due": ""}
]
JSON
    ;;
  *)
    # default JSON export minimal
    echo '[]'
    ;;
 esac
`
	if err := os.WriteFile(stub, []byte(content), 0755); err != nil {
		t.Fatalf("write stub failed: %v", err)
	}
	return stub
}

func newTestServerWithStub(t *testing.T, stub string, home string) *Server {
	cfg := config.Default()
	cfg.DstaskBin = stub
	cfg.Repos = map[string]string{"admin": home}
	store := auth.NewInMemoryUserStore()
	if err := store.AddUserPlain("admin", "admin"); err != nil {
		t.Fatal(err)
	}
	return NewServerWithConfig(store, cfg)
}

func TestProjectsEndpoint_RendersTable_FromJSON(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	os.MkdirAll(filepath.Join(home, ".dstask"), 0755)
	stub := createDstaskStub(t, tmp)
	s := newTestServerWithStub(t, stub, home)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/projects", nil)
	req.SetBasicAuth("admin", "admin")
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "Projects") || !strings.Contains(body, "alpha") || !strings.Contains(body, "beta") {
		t.Fatalf("projects page did not render expected names: %s", body)
	}
}

func TestTagsEndpoint_RendersList_FromPlaintext(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	os.MkdirAll(filepath.Join(home, ".dstask"), 0755)
	stub := createDstaskStub(t, tmp)
	s := newTestServerWithStub(t, stub, home)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/tags", nil)
	req.SetBasicAuth("admin", "admin")
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	okUI := strings.Contains(body, "+ui") || strings.Contains(body, "&#43;ui") || strings.Contains(body, ">ui<")
	okBE := strings.Contains(body, "+backend") || strings.Contains(body, "&#43;backend") || strings.Contains(body, ">backend<")
	if !strings.Contains(body, "Tags") || !okUI || !okBE {
		t.Fatalf("tags page did not render expected tags: %s", body)
	}
}

func TestTemplatesEndpoint_RendersTable_FromJSON(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	os.MkdirAll(filepath.Join(home, ".dstask"), 0755)
	stub := createDstaskStub(t, tmp)
	s := newTestServerWithStub(t, stub, home)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/templates", nil)
	req.SetBasicAuth("admin", "admin")
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "Templates") || !strings.Contains(body, "Template A") || !strings.Contains(body, "Template B") {
		t.Fatalf("templates page did not render expected templates: %s", body)
	}
}
