package main

import (
	"log"
	"net/http"
	"os"

	"github.com/mbusc/dstask-ui/internal/auth"
	"github.com/mbusc/dstask-ui/internal/config"
	"github.com/mbusc/dstask-ui/internal/server"
)

func main() {
	username := getenvDefault("DSTWEB_USER", "admin")
	password := getenvDefault("DSTWEB_PASS", "admin")
	listenAddr := getenvDefault("DSTWEB_LISTEN", ":8080")

	// Config laden (optional config.yaml)
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	userStore := auth.NewInMemoryUserStore()
	// Benutzer aus Datei
	for _, u := range cfg.Users {
		if u.Username == "" || u.PasswordHash == "" {
			continue
		}
		if err := userStore.AddUserHash(u.Username, []byte(u.PasswordHash)); err != nil {
			log.Fatalf("invalid user in config: %v", err)
		}
	}
	// Fallback-User aus ENV, wenn keiner definiert
	if len(cfg.Users) == 0 {
		if err := userStore.AddUserPlain(username, password); err != nil {
			log.Fatalf("failed to add default user: %v", err)
		}
	}

	srv := server.NewServerWithConfig(userStore, cfg)

	log.Printf("dstask web UI listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getenvDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
