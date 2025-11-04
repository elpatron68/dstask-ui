package main

import (
	stdlog "log"
	"net/http"
	"os"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
	applog "github.com/elpatron68/dstask-ui/internal/log"
    "github.com/elpatron68/dstask-ui/internal/dstask"
	"github.com/elpatron68/dstask-ui/internal/server"
)

func main() {
	username := getenvDefault("DSTWEB_USER", "admin")
	password := getenvDefault("DSTWEB_PASS", "admin")

	// Config laden (optional config.yaml)
	// Versuche zuerst im aktuellen Verzeichnis, dann im Root-Verzeichnis (zwei Ebenen nach oben)
	configPath := ""
	if _, err := os.Stat("config.yaml"); err == nil {
		configPath = "config.yaml"
	} else if _, err := os.Stat("../../config.yaml"); err == nil {
		configPath = "../../config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		stdlog.Fatalf("config error: %v", err)
	}

	// Listen address: config.yaml > ENV > default
	listenAddr := cfg.Listen
	if listenAddr == "" {
		listenAddr = getenvDefault("DSTWEB_LISTEN", ":8080")
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
