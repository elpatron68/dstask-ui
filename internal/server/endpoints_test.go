package server

import (
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "testing"
    "net/http/httptest"
    "strings"

    "github.com/elpatron68/dstask-ui/internal/auth"
    "github.com/elpatron68/dstask-ui/internal/config"
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
