package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
)

func TestSyncPost_ShowsCloneForm_IfNoGitRepo(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(filepath.Join(home, ".dstask"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Repos = map[string]string{"admin": home}
	store := auth.NewInMemoryUserStore()
	if err := store.AddUserPlain("admin", "admin"); err != nil {
		t.Fatal(err)
	}
	s := NewServerWithConfig(store, cfg)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/sync", nil)
	req.SetBasicAuth("admin", "admin")

	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(strings.ToLower(body), "clone remote") {
		t.Fatalf("expected clone form when no .git repo exists; body=%s", body)
	}
}
