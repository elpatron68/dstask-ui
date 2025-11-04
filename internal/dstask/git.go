package dstask

import (
    "bytes"
    "errors"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/elpatron68/dstask-ui/internal/config"
    applog "github.com/elpatron68/dstask-ui/internal/log"
)

// RepoDirForUser liefert den Pfad zum .dstask-Verzeichnis des Nutzers.
func (r *Runner) RepoDirForUser(username string) (string, error) {
    home, ok := config.ResolveHomeForUsername(r.cfg, username)
    if !ok || strings.TrimSpace(home) == "" {
        if h, err := os.UserHomeDir(); err == nil && h != "" {
            home = h
        }
    }
    if strings.TrimSpace(home) == "" {
        return "", errors.New("HOME konnte nicht ermittelt werden")
    }
    repo := home
    if filepath.Base(strings.ToLower(repo)) != ".dstask" {
        repo = filepath.Join(home, ".dstask")
    }
    applog.Debugf("RepoDirForUser(%s): %s", username, repo)
    return repo, nil
}

// GitRemoteURL gibt die URL von remote "origin" zurück (leer wenn nicht gesetzt).
func (r *Runner) GitRemoteURL(username string) (string, error) {
    repo, err := r.RepoDirForUser(username)
    if err != nil {
        return "", err
    }
    cmd := exec.Command("git", "-C", repo, "config", "--get", "remote.origin.url")
    // HOME setzen wie in Run()
    env := os.Environ()
    if home, ok := config.ResolveHomeForUsername(r.cfg, username); ok && home != "" {
        replacedHome := false
        for i, e := range env {
            if strings.HasPrefix(e, "HOME=") {
                env[i] = "HOME=" + home
                replacedHome = true
                break
            }
        }
        if !replacedHome {
            env = append(env, "HOME="+home)
        }
    }
    cmd.Env = env
    var out bytes.Buffer
    cmd.Stdout = &out
    if err := cmd.Run(); err != nil {
        applog.Debugf("GitRemoteURL: kein remote.origin.url gefunden in %s (%v)", repo, err)
        return "", nil
    }
    return strings.TrimSpace(out.String()), nil
}

// GitSetRemoteOrigin setzt remote "origin" auf url. Falls vorhanden, wird die URL aktualisiert.
func (r *Runner) GitSetRemoteOrigin(username, url string) error {
    repo, err := r.RepoDirForUser(username)
    if err != nil {
        return err
    }
    // .git muss vorhanden sein – andernfalls soll explizit geklont werden
    if _, statErr := os.Stat(filepath.Join(repo, ".git")); statErr != nil {
        applog.Warnf("GitSetRemoteOrigin: kein Git-Repository in %s – Remote kann nicht gesetzt werden", repo)
        return errors.New("kein Git-Repository vorhanden – bitte Repository klonen")
    }
    env := os.Environ()
    if home, ok := config.ResolveHomeForUsername(r.cfg, username); ok && home != "" {
        replacedHome := false
        for i, e := range env {
            if strings.HasPrefix(e, "HOME=") {
                env[i] = "HOME=" + home
                replacedHome = true
                break
            }
        }
        if !replacedHome {
            env = append(env, "HOME="+home)
        }
    }

    // Prüfe, ob origin existiert
    check := exec.Command("git", "-C", repo, "remote")
    check.Env = env
    var out bytes.Buffer
    check.Stdout = &out
    _ = check.Run()
    remotes := strings.Split(strings.TrimSpace(out.String()), "\n")
    hasOrigin := false
    for _, r := range remotes {
        if strings.TrimSpace(r) == "origin" {
            hasOrigin = true
            break
        }
    }

    var cmd *exec.Cmd
    if hasOrigin {
        cmd = exec.Command("git", "-C", repo, "remote", "set-url", "origin", url)
    } else {
        cmd = exec.Command("git", "-C", repo, "remote", "add", "origin", url)
    }
    cmd.Env = env
    if err := cmd.Run(); err != nil {
        applog.Warnf("GitSetRemoteOrigin: remote set/add failed in %s: %v", repo, err)
        return err
    }
    applog.Infof("GitSetRemoteOrigin: origin=%s konfiguriert in %s", url, repo)
    return nil
}

// GitCloneRemote klont das Remote-Repository in das .dstask-Verzeichnis.
// - Wenn das Verzeichnis nicht existiert: git clone <url> <repoDir>
// - Wenn es existiert und leer ist: in diesem Verzeichnis git clone <url> .
// - Andernfalls Fehler.
func (r *Runner) GitCloneRemote(username, url string) error {
    repo, err := r.RepoDirForUser(username)
    if err != nil {
        return err
    }
    // Stelle sicher, dass Elternverzeichnis existiert
    if err := os.MkdirAll(filepath.Dir(repo), 0755); err != nil {
        return err
    }
    env := os.Environ()
    if home, ok := config.ResolveHomeForUsername(r.cfg, username); ok && home != "" {
        replacedHome := false
        for i, e := range env {
            if strings.HasPrefix(e, "HOME=") {
                env[i] = "HOME=" + home
                replacedHome = true
                break
            }
        }
        if !replacedHome {
            env = append(env, "HOME="+home)
        }
    }

    // Existenz prüfen
    if _, err := os.Stat(repo); errors.Is(err, os.ErrNotExist) {
        applog.Infof("GitCloneRemote: Verzeichnis fehlt, klone nach %s", repo)
        cmd := exec.Command("git", "clone", url, repo)
        cmd.Env = env
        if err := cmd.Run(); err != nil {
            applog.Warnf("GitCloneRemote: clone nach %s fehlgeschlagen: %v", repo, err)
            return err
        }
        return nil
    }

    // Verzeichnis existiert: prüfen, ob leer
    empty, err := isDirEmpty(repo)
    if err != nil {
        return err
    }
    if !empty {
        return errors.New("Zielverzeichnis ist nicht leer – Klonen abgebrochen")
    }
    applog.Infof("GitCloneRemote: Verzeichnis existiert und ist leer, klone in %s (.)", repo)
    cmd := exec.Command("git", "clone", url, ".")
    cmd.Env = env
    cmd.Dir = repo
    if err := cmd.Run(); err != nil {
        applog.Warnf("GitCloneRemote: clone in bestehendes Verzeichnis %s fehlgeschlagen: %v", repo, err)
        return err
    }
    return nil
}

func isDirEmpty(path string) (bool, error) {
    f, err := os.Open(path)
    if err != nil {
        return false, err
    }
    defer f.Close()
    // Eine kleine Anzahl Einträge reicht
    entries, err := f.Readdirnames(1)
    if err != nil {
        // EOF → leer
        if errors.Is(err, os.ErrNotExist) {
            return true, nil
        }
        if err.Error() == "EOF" {
            return true, nil
        }
        return false, err
    }
    return len(entries) == 0, nil
}

// GitSetUpstreamIfMissing setzt den Upstream für den aktuellen Branch auf origin/<branch>, falls nicht gesetzt.
// Gibt den Branch-Namen zurück.
func (r *Runner) GitSetUpstreamIfMissing(username string) (string, error) {
    repo, err := r.RepoDirForUser(username)
    if err != nil {
        return "", err
    }
    env := os.Environ()
    if home, ok := config.ResolveHomeForUsername(r.cfg, username); ok && home != "" {
        replacedHome := false
        for i, e := range env {
            if strings.HasPrefix(e, "HOME=") {
                env[i] = "HOME=" + home
                replacedHome = true
                break
            }
        }
        if !replacedHome {
            env = append(env, "HOME="+home)
        }
    }

    // Ermittle aktuellen Branch
    curr := exec.Command("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD")
    curr.Env = env
    var currOut bytes.Buffer
    curr.Stdout = &currOut
    if err := curr.Run(); err != nil {
        return "", err
    }
    branch := strings.TrimSpace(currOut.String())
    if branch == "" || branch == "HEAD" {
        branch = "master"
    }

    // Prüfe, ob Upstream gesetzt ist
    up := exec.Command("git", "-C", repo, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
    up.Env = env
    if err := up.Run(); err == nil {
        // Upstream existiert bereits
        applog.Debugf("GitSetUpstreamIfMissing: Upstream schon gesetzt in %s", repo)
        return branch, nil
    }

    // Setze Upstream zu origin/<branch>; bei Fehler versuche Remote-HEAD-Branch (z. B. main)
    set := exec.Command("git", "-C", repo, "branch", "--set-upstream-to=origin/"+branch, branch)
    set.Env = env
    if err := set.Run(); err != nil {
        applog.Warnf("GitSetUpstreamIfMissing: Upstream auf origin/%s setzen fehlgeschlagen in %s: %v", branch, repo, err)
        if head, herr := r.GitRemoteHeadBranch(username); herr == nil && head != "" && head != branch {
            applog.Infof("GitSetUpstreamIfMissing: versuche stattdessen origin/%s", head)
            set2 := exec.Command("git", "-C", repo, "branch", "--set-upstream-to=origin/"+head, branch)
            set2.Env = env
            if err2 := set2.Run(); err2 == nil {
                applog.Infof("GitSetUpstreamIfMissing: Upstream gesetzt auf origin/%s (lokal: %s) in %s", head, branch, repo)
                return branch, nil
            }
        }
        return branch, err
    }
    applog.Infof("GitSetUpstreamIfMissing: Upstream gesetzt auf origin/%s in %s", branch, repo)
    return branch, nil
}

// GitRemoteHeadBranch ermittelt den HEAD-Branch des Remotes (z. B. main/master)
func (r *Runner) GitRemoteHeadBranch(username string) (string, error) {
    repo, err := r.RepoDirForUser(username)
    if err != nil {
        return "", err
    }
    env := os.Environ()
    if home, ok := config.ResolveHomeForUsername(r.cfg, username); ok && home != "" {
        replacedHome := false
        for i, e := range env {
            if strings.HasPrefix(e, "HOME=") {
                env[i] = "HOME=" + home
                replacedHome = true
                break
            }
        }
        if !replacedHome {
            env = append(env, "HOME="+home)
        }
    }
    // 1) Schnellweg: symbolic-ref
    cmd := exec.Command("git", "-C", repo, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
    cmd.Env = env
    var out bytes.Buffer
    cmd.Stdout = &out
    if err := cmd.Run(); err == nil {
        s := strings.TrimSpace(out.String()) // origin/main
        if strings.HasPrefix(s, "origin/") {
            return strings.TrimPrefix(s, "origin/"), nil
        }
        return s, nil
    }
    // 2) Fallback: remote show origin parsen
    cmd2 := exec.Command("git", "-C", repo, "remote", "show", "origin")
    cmd2.Env = env
    out.Reset()
    cmd2.Stdout = &out
    if err := cmd2.Run(); err == nil {
        lines := strings.Split(out.String(), "\n")
        for _, ln := range lines {
            ln = strings.TrimSpace(ln)
            if strings.HasPrefix(strings.ToLower(ln), "head branch:") {
                parts := strings.SplitN(ln, ":", 2)
                if len(parts) == 2 {
                    return strings.TrimSpace(parts[1]), nil
                }
            }
        }
    }
    return "", errors.New("Remote-HEAD konnte nicht ermittelt werden")
}


