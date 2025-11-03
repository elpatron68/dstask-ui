package main

import (
	stdlog "log"
	"net/http"
	"os"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
	applog "github.com/elpatron68/dstask-ui/internal/log"
	"github.com/elpatron68/dstask-ui/internal/server"
)

func main() {
	username := getenvDefault("DSTWEB_USER", "admin")
	password := getenvDefault("DSTWEB_PASS", "admin")
	listenAddr := getenvDefault("DSTWEB_LISTEN", ":8080")

	// Config laden (optional config.yaml)
	cfg, err := config.Load("")
	if err != nil {
		stdlog.Fatalf("config error: %v", err)
	}

	// Init logging
	applog.InitFromEnvFallback(cfg.Logging.Level)

	userStore := auth.NewInMemoryUserStore()
	// Benutzer aus Datei
	for _, u := range cfg.Users {
		if u.Username == "" || u.PasswordHash == "" {
			continue
		}
		if err := userStore.AddUserHash(u.Username, []byte(u.PasswordHash)); err != nil {
			stdlog.Fatalf("invalid user in config: %v", err)
		}
	}
	// Fallback-User aus ENV, wenn keiner definiert
	if len(cfg.Users) == 0 {
		if err := userStore.AddUserPlain(username, password); err != nil {
			stdlog.Fatalf("failed to add default user: %v", err)
		}
	}

	srv := server.NewServerWithConfig(userStore, cfg)

	stdlog.Printf("dstask web UI listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, srv.Handler()); err != nil {
		stdlog.Fatalf("server error: %v", err)
	}
}

func getenvDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
