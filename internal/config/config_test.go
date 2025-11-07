package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultHasReasonableValues(t *testing.T) {
	cfg := Default()
	// dstaskBin is optional now; default may be empty and PATH autodetection is used at runtime
	if cfg.DstaskBin != "" {
		t.Fatalf("expected empty default dstask bin, got %q", cfg.DstaskBin)
	}
	if cfg.Listen != ":8080" {
		t.Fatalf("expected default listen :8080, got %q", cfg.Listen)
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("expected default log level info, got %q", cfg.Logging.Level)
	}
	if !cfg.UI.ShowCommandLog {
		t.Fatalf("expected ShowCommandLog default true")
	}
	if cfg.UI.CommandLogMax <= 0 {
		t.Fatalf("expected positive CommandLogMax")
	}
	if cfg.GitAutoSync {
		t.Fatalf("expected GitAutoSync default false")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("DSTWEB_DSTASK_BIN", "C:/x/dstask.exe")
	t.Setenv("DSTWEB_LISTEN", ":3000")
	t.Setenv("DSTWEB_LOG_LEVEL", "debug")
	t.Setenv("DSTWEB_UI_SHOW_CMDLOG", "0")
	t.Setenv("DSTWEB_CMDLOG_MAX", "50")
	t.Setenv("DSTWEB_GIT_AUTOSYNC", "true")

	cfg, err := Load("__does_not_exist.yaml")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if cfg.DstaskBin != "C:/x/dstask.exe" {
		t.Fatalf("dstaskBin env override failed: %q", cfg.DstaskBin)
	}
	if cfg.Listen != ":3000" {
		t.Fatalf("listen env override failed: %q", cfg.Listen)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("log level env override failed: %q", cfg.Logging.Level)
	}
	if cfg.UI.ShowCommandLog {
		t.Fatalf("UI.ShowCommandLog expected false via env")
	}
	if cfg.UI.CommandLogMax != 50 {
		t.Fatalf("UI.CommandLogMax expected 50 via env, got %d", cfg.UI.CommandLogMax)
	}
	if !cfg.GitAutoSync {
		t.Fatalf("GitAutoSync expected true via env override")
	}
}

func TestLoadFromFileAndResolveHome(t *testing.T) {
	// Save and clear environment variables that might override file values
	// We need to unset them, not just set them to empty strings
	envVars := []string{"DSTWEB_DSTASK_BIN", "DSTWEB_LISTEN", "DSTWEB_LOG_LEVEL", "DSTWEB_UI_SHOW_CMDLOG", "DSTWEB_CMDLOG_MAX"}
	saved := make(map[string]string)
	for _, key := range envVars {
		if val, ok := os.LookupEnv(key); ok {
			saved[key] = val
		}
		os.Unsetenv(key)
	}
	// Restore environment variables after test
	defer func() {
		for _, key := range envVars {
			if val, ok := saved[key]; ok {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	dir := t.TempDir()
	yaml := []byte("dstaskBin: 'D:/dstask.exe'\nlisten: '127.0.0.1:9000'\ngitAutoSync: true\nrepos:\n  alice: 'C:/Users/alice/.dstask'\nlogging:\n  level: 'warn'\nui:\n  showCommandLog: false\n  commandLogMax: 12\n")
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, yaml, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DstaskBin != "D:/dstask.exe" {
		t.Fatalf("file load failed for dstaskBin: %q", cfg.DstaskBin)
	}
	if cfg.Listen != "127.0.0.1:9000" {
		t.Fatalf("file load failed for listen: %q", cfg.Listen)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("file load failed for logging.level: %q", cfg.Logging.Level)
	}
	if cfg.UI.ShowCommandLog {
		t.Fatalf("file load failed for ui.showCommandLog")
	}
	if cfg.UI.CommandLogMax != 12 {
		t.Fatalf("file load failed for ui.commandLogMax: %d", cfg.UI.CommandLogMax)
	}
	if !cfg.GitAutoSync {
		t.Fatalf("file load failed for gitAutoSync")
	}
	home, ok := ResolveHomeForUsername(cfg, "alice")
	if !ok {
		t.Fatalf("expected repo mapping for alice")
	}
	want := filepath.FromSlash("C:/Users/alice")
	if home != want {
		t.Fatalf("expected %q, got %q", want, home)
	}
}
