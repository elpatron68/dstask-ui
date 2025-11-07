package main

import (
	"flag"
	stdlog "log"
	"net/http"
	"os"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
	"github.com/elpatron68/dstask-ui/internal/dstask"
	applog "github.com/elpatron68/dstask-ui/internal/log"
	"github.com/elpatron68/dstask-ui/internal/server"
)

func main() {
	listenFlag := flag.String("listen", "", "override listen address (e.g. :8080 or 127.0.0.1:8080)")
	flag.Parse()

	username := getenvDefault("DSTWEB_USER", "admin")
	password := getenvDefault("DSTWEB_PASS", "admin")

	// Config laden aus <HOME>/.dstask-ui/config.yaml; wenn fehlend, wird sie mit Defaults erzeugt
	cfg, err := config.Load("")
	if err != nil {
		stdlog.Fatalf("config error: %v", err)
	}

	listenAddr := resolveListenAddress(cfg, *listenFlag)

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

	// Startup-Checks: dstask-Binary + Repo(s)
	usernames := make([]string, 0, len(cfg.Repos))
	for uname := range cfg.Repos {
		usernames = append(usernames, uname)
	}
	// Wenn keine Repos konfiguriert, verwende den konfigurierten/an Umgebungsvariablen hängenden Login-Nutzer,
	// damit zumindest das Repo im Prozess-HOME initialisiert wird.
	if len(usernames) == 0 {
		usernames = append(usernames, username)
	}
	if err := dstask.EnsureReady(cfg, usernames); err != nil {
		stdlog.Fatalf("startup check failed: %v", err)
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

func resolveListenAddress(cfg *config.Config, cli string) string {
	if cli != "" {
		return cli
	}
	if env := os.Getenv("DSTWEB_LISTEN"); env != "" {
		return env
	}
	if cfg != nil && cfg.Listen != "" {
		return cfg.Listen
	}
	return ":8080"
}
