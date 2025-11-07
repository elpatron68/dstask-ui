package main

import (
	"testing"

	"github.com/elpatron68/dstask-ui/internal/config"
)

func TestResolveListenAddress(t *testing.T) {
	t.Run("defaults to :8080", func(t *testing.T) {
		t.Setenv("DSTWEB_LISTEN", "")
		got := resolveListenAddress(&config.Config{}, "")
		if got != ":8080" {
			t.Fatalf("expected :8080, got %s", got)
		}
	})

	t.Run("uses config value", func(t *testing.T) {
		t.Setenv("DSTWEB_LISTEN", "")
		cfg := &config.Config{Listen: "127.0.0.1:9000"}
		got := resolveListenAddress(cfg, "")
		if got != "127.0.0.1:9000" {
			t.Fatalf("expected config listen, got %s", got)
		}
	})

	t.Run("env overrides config", func(t *testing.T) {
		t.Setenv("DSTWEB_LISTEN", "0.0.0.0:7777")
		cfg := &config.Config{Listen: "127.0.0.1:9000"}
		got := resolveListenAddress(cfg, "")
		if got != "0.0.0.0:7777" {
			t.Fatalf("expected env override, got %s", got)
		}
	})

	t.Run("flag overrides env and config", func(t *testing.T) {
		t.Setenv("DSTWEB_LISTEN", "0.0.0.0:7777")
		cfg := &config.Config{Listen: "127.0.0.1:9000"}
		got := resolveListenAddress(cfg, "[::1]:6060")
		if got != "[::1]:6060" {
			t.Fatalf("expected flag override, got %s", got)
		}
	})
}
