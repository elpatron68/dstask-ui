package dstask

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/elpatron68/dstask-ui/internal/config"
)

func TestEnsureReady_WithExistingRepoAndValidBin(t *testing.T) {
    t.Parallel()
    tmp := t.TempDir()
    home := filepath.Join(tmp, "home")
    if err := os.MkdirAll(filepath.Join(home, ".dstask"), 0755); err != nil {
        t.Fatal(err)
    }
    cfg := &config.Config{DstaskBin: "/bin/true", Repos: map[string]string{"u": home}}
    // Should succeed without trying to initialize
    if err := EnsureReady(cfg, []string{"u"}); err != nil {
        t.Fatalf("EnsureReady failed: %v", err)
    }
}


