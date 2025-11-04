package dstask

import (
    "errors"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "time"

    "github.com/elpatron68/dstask-ui/internal/config"
    applog "github.com/elpatron68/dstask-ui/internal/log"
)

// EnsureReady prüft beim Start:
// 1) Ist das dstask-Binary auffindbar/ausführbar?
// 2) Existiert pro Nutzer das .dstask-Repository, ansonsten wird es initialisiert.
// usernames: Liste der App-Nutzer (für HOME-Auflösung per cfg.Repos oder Prozess-HOME als Fallback)
func EnsureReady(cfg *config.Config, usernames []string) error {
    // 1) Binary validieren/finden
    if err := ensureDstaskBinary(cfg); err != nil {
        return err
    }

    // 2) Repos sicherstellen
    r := NewRunner(cfg)
    for _, user := range usernames {
        if strings.TrimSpace(user) == "" {
            continue
        }
        if err := ensureRepoForUser(r, user); err != nil {
            return err
        }
    }
    return nil
}

func ensureDstaskBinary(cfg *config.Config) error {
    bin := strings.TrimSpace(cfg.DstaskBin)

    // Falls Windows-Default in nicht-Windows-Umgebung gesetzt ist, versuche PATH-Fallback
    if bin == "" || (runtime.GOOS != "windows" && (bin == `C:\\tools\\dstask.exe`)) {
        if p, err := exec.LookPath("dstask"); err == nil {
            cfg.DstaskBin = p
            applog.Infof("dstask Binary aus PATH verwendet: %s", p)
            return nil
        }
    }

    // Existiert das angegebene Binary direkt oder im PATH?
    if p, err := exec.LookPath(bin); err == nil {
        cfg.DstaskBin = p
        applog.Infof("dstask Binary gefunden: %s", p)
        return nil
    }

    // Letzter Versuch: plain "dstask" im PATH
    if p, err := exec.LookPath("dstask"); err == nil {
        cfg.DstaskBin = p
        applog.Infof("dstask Binary (Fallback) aus PATH: %s", p)
        return nil
    }

    return errors.New("dstask Binary wurde nicht gefunden – bitte DSTWEB_DSTASK_BIN setzen oder dstask installieren")
}

func ensureRepoForUser(r *Runner, username string) error {
    // Bestimme HOME/Repo-Pfad
    home, ok := config.ResolveHomeForUsername(r.cfg, username)
    if !ok || strings.TrimSpace(home) == "" {
        // Fallback: Prozess-HOME
        if h, err := os.UserHomeDir(); err == nil && h != "" {
            home = h
        }
    }
    if strings.TrimSpace(home) == "" {
        // Ohne HOME kein Repo-Setup möglich
        applog.Warnf("Kein HOME für Nutzer %q ermittelbar – Repo-Check übersprungen", username)
        return nil
    }

    repoDir := home
    if !strings.HasSuffix(strings.ToLower(repoDir), string(filepath.Separator)+".dstask") && filepath.Base(repoDir) != ".dstask" {
        repoDir = filepath.Join(home, ".dstask")
    }

    // Wenn Repo existiert, nichts tun
    if st, err := os.Stat(repoDir); err == nil && st.IsDir() {
        // Optional: schneller Sanity-Check mit "dstask version"
        res := r.Run(username, 3*time.Second, "version")
        if res.Err == nil && res.ExitCode == 0 && !res.TimedOut {
            return nil
        }
        // Auch wenn version fehlschlägt, Repo existiert – nicht interaktiv löschen/neu anlegen
        return nil
    }

    // Repo existiert nicht – nicht-interaktive Initialisierung mit stdin="y\n"
    applog.Infof("dstask-Repository fehlt für %q unter %s – wird angelegt", username, repoDir)
    res := r.RunWithStdin(username, 10*time.Second, "y\n")
    if res.TimedOut {
        return errors.New("Initialisierung von dstask-Repository ist in Timeout gelaufen")
    }
    if res.Err != nil || res.ExitCode != 0 {
        // Häufiger Fall: dstask hat erneut nachgefragt oder stderr liefert Hinweis
        applog.Warnf("dstask Init stderr: %q", truncate(res.Stderr, 300))
        return errors.New("dstask-Repository konnte nicht initialisiert werden")
    }

    // Nachkontrolle: Verzeichnis sollte jetzt existieren
    if st, err := os.Stat(repoDir); err != nil || !st.IsDir() {
        return errors.New("dstask-Repository wurde nach Initialisierung nicht gefunden")
    }
    applog.Infof("dstask-Repository angelegt unter %s", repoDir)
    return nil
}


