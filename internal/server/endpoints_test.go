package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
	"github.com/elpatron68/dstask-ui/internal/music"
)

// createDstaskStub erzeugt ein Shell-Script, das je nach Subcommand vordefinierte Ausgaben liefert.
func createDstaskStub(t *testing.T, dir string) string {
	t.Helper()
	// Build a small Go binary as cross-platform stub
	src := filepath.Join(dir, "dstask_stub_main.go")
	bin := filepath.Join(dir, "dstask-stub")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	code := `package main
import (
  "encoding/json"
  "fmt"
  "os"
)
func main(){
  if len(os.Args) < 2 { fmt.Println("[]"); return }
  switch os.Args[1] {
  case "show-projects":
    fmt.Println("[{\"name\":\"alpha\",\"taskCount\":3,\"resolvedCount\":1,\"active\":1,\"priority\":\"P2\"},{\"name\":\"beta\",\"taskCount\":5,\"resolvedCount\":0,\"active\":2,\"priority\":\"P1\"}]")
  case "show-tags":
    fmt.Println("+ui")
    fmt.Println("+backend")
  case "show-templates":
    type T struct{ ID string ` + "`json:\"id\"`" + `; Summary string ` + "`json:\"summary\"`" + `; Project string ` + "`json:\"project\"`" + `; Tags []string ` + "`json:\"tags\"`" + `; Due string ` + "`json:\"due\"`" + ` }
    out, _ := json.Marshal([]T{{ID:"1",Summary:"Template A",Project:"alpha",Tags:[]string{"ui"},Due:"2025-12-31"},{ID:"2",Summary:"Template B",Project:"beta",Tags:[]string{"backend"}}})
    fmt.Println(string(out))
  default:
    fmt.Println("[]")
  }
}
`
	if err := os.WriteFile(src, []byte(code), 0644); err != nil {
		t.Fatalf("write stub source failed: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build stub failed: %v, out=%s", err, string(out))
	}
	return bin
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

func TestAutoSyncTriggeredAfterTaskAdd(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	os.MkdirAll(filepath.Join(home, ".dstask"), 0755)
	stub := createDstaskStub(t, tmp)
	s := newTestServerWithStub(t, stub, home)
	s.cfg.GitAutoSync = true

	form := url.Values{}
	form.Set("summary", "Test task via UI")
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(form.Encode()))
	req.SetBasicAuth("admin", "admin")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect after task creation, got %d", rr.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	found := false
	for !found && time.Now().Before(deadline) {
		entries := s.cmdStore.List("admin", 5)
		for _, e := range entries {
			if e.Context == "Auto sync" {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !found {
		t.Fatalf("expected auto sync command log entry")
	}
}

func TestTasksNewContainsMusicInputs(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	os.MkdirAll(filepath.Join(home, ".dstask"), 0755)
	stub := createDstaskStub(t, tmp)
	s := newTestServerWithStub(t, stub, home)

	req := httptest.NewRequest(http.MethodGet, "/tasks/new", nil)
	req.SetBasicAuth("admin", "admin")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
	body := rr.Body.String()
	checks := []string{
		"Music (radio station)",
		"name=\"music_type\"",
		"id=\"new_music_name_search\"",
		"name=\"music_url\"",
	}
	for _, substr := range checks {
		if !strings.Contains(body, substr) {
			t.Fatalf("/tasks/new missing %q in response: %s", substr, body)
		}
	}
}

func TestMusicTasksHandlersPersistAndReturnState(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	os.MkdirAll(filepath.Join(home, ".dstask"), 0755)
	stub := createDstaskStub(t, tmp)
	s := newTestServerWithStub(t, stub, home)

	// Seed existing entry
	m := music.Map{Version: 1, Tasks: map[string]music.TaskMusic{
		"123": {Type: "radio", Name: "Init", URL: "https://stream", Volume: 0.42, Muted: true},
	}}
	if _, err := music.SaveForUser(s.cfg, "admin", &m); err != nil {
		t.Fatalf("seed map failed: %v", err)
	}

	// GET should return persisted values
	reqGet := httptest.NewRequest(http.MethodGet, "/music/tasks/123", nil)
	reqGet.SetBasicAuth("admin", "admin")
	rrGet := httptest.NewRecorder()
	s.Handler().ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rrGet.Code)
	}
	if !strings.Contains(rrGet.Body.String(), "\"volume\":0.42") {
		t.Fatalf("GET response missing volume: %s", rrGet.Body.String())
	}

	// PUT new state
	payload := map[string]any{
		"type":   "radio",
		"name":   "Updated",
		"url":    "https://updated",
		"volume": 0.65,
		"muted":  false,
	}
	buf, _ := json.Marshal(payload)
	reqPut := httptest.NewRequest(http.MethodPut, "/music/tasks/123", bytes.NewReader(buf))
	reqPut.SetBasicAuth("admin", "admin")
	reqPut.Header.Set("Content-Type", "application/json")
	rrPut := httptest.NewRecorder()
	s.Handler().ServeHTTP(rrPut, reqPut)
	if rrPut.Code != http.StatusOK {
		t.Fatalf("PUT status = %d", rrPut.Code)
	}

	// Ensure persisted map reflects update
	loaded, _, err := music.LoadForUser(s.cfg, "admin")
	if err != nil {
		t.Fatalf("load map failed: %v", err)
	}
	entry, ok := loaded.Tasks["123"]
	if !ok {
		t.Fatalf("task 123 missing after PUT")
	}
	if entry.Volume != 0.65 || entry.Muted {
		t.Fatalf("unexpected entry after PUT: %+v", entry)
	}
}
