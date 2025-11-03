package dstask

import (
    "bytes"
    "context"
    "encoding/json"
    "os/exec"
    "os"
    "strings"
    "time"
    "log"

    "github.com/mbusc/dstask-ui/internal/config"
)

type Runner struct {
    cfg *config.Config
}

func NewRunner(cfg *config.Config) *Runner {
    return &Runner{cfg: cfg}
}

type Result struct {
    Stdout string
    Stderr string
    Err    error
    ExitCode int
    TimedOut bool
}

// Run führt dstask mit gegebenen Argumenten für einen Benutzer aus.
// timeout bestimmt die maximale Laufzeit.
func (r *Runner) Run(username string, timeout time.Duration, args ...string) Result {
    bin := r.cfg.DstaskBin
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, bin, args...)

    // Starte mit vererbter Umgebung, damit PATH/GIT etc. vorhanden sind
    env := os.Environ()
    // HOME/USERPROFILE für den Nutzer ggf. überschreiben
    if home, ok := config.ResolveHomeForUsername(r.cfg, username); ok && home != "" {
        // ersetzen oder anhängen
        replacedHome := false
        replacedUser := false
        for i, e := range env {
            if strings.HasPrefix(e, "HOME=") {
                env[i] = "HOME=" + home
                replacedHome = true
                continue
            }
            if strings.HasPrefix(e, "USERPROFILE=") {
                env[i] = "USERPROFILE=" + home
                replacedUser = true
            }
        }
        if !replacedHome {
            env = append(env, "HOME="+home)
        }
        if !replacedUser {
            env = append(env, "USERPROFILE="+home)
        }
        // Arbeitsverzeichnis optional auf HOME setzen
        cmd.Dir = home
    }
    cmd.Env = env

    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf

    log.Printf("dstask run: %s %s", bin, strings.Join(args, " "))
    runErr := cmd.Run()
    res := Result{
        Stdout: normalizeNewlines(outBuf.String()),
        Stderr: normalizeNewlines(errBuf.String()),
        Err:    runErr,
    }
    if ctx.Err() == context.DeadlineExceeded {
        res.TimedOut = true
    }
    if exitErr, ok := runErr.(*exec.ExitError); ok {
        res.ExitCode = exitErr.ExitCode()
    } else if runErr == nil {
        res.ExitCode = 0
    } else {
        res.ExitCode = -1
    }
    log.Printf("dstask exit: code=%d timeout=%v stderr=%q", res.ExitCode, res.TimedOut, truncate(res.Stderr, 300))
    return res
}

func normalizeNewlines(s string) string {
    // Vereinheitliche Zeilenenden auf \n
    s = strings.ReplaceAll(s, "\r\n", "\n")
    s = strings.ReplaceAll(s, "\r", "\n")
    return s
}

// Helper zum sicheren Zusammenbauen einer Summary-Zeile (Whitelist wird später ergänzt)
func JoinArgs(parts ...string) string {
    cleaned := make([]string, 0, len(parts))
    for _, p := range parts {
        if p == "" {
            continue
        }
        cleaned = append(cleaned, p)
    }
    return strings.Join(cleaned, " ")
}

func truncate(s string, max int) string {
    if len(s) <= max { return s }
    return s[:max] + "..."
}

// Export ruft `dstask export` auf und liefert die rohe JSON-Struktur zurück.
func (r *Runner) Export(username string, timeout time.Duration) ([]Task, error) {
    res := r.Run(username, timeout, "export")
    if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
        if res.Err != nil {
            return nil, res.Err
        }
        return nil, context.DeadlineExceeded
    }
    // Zuerst versuchen als Array zu decodieren
    var tasks []Task
    dec := json.NewDecoder(strings.NewReader(res.Stdout))
    dec.UseNumber()
    if err := dec.Decode(&tasks); err == nil {
        return tasks, nil
    }
    // Fallback: evtl. Wrapper-Objekt mit tasks-Array
    var root map[string]any
    dec2 := json.NewDecoder(strings.NewReader(res.Stdout))
    dec2.UseNumber()
    if err := dec2.Decode(&root); err != nil {
        return nil, err
    }
    if v, ok := root["tasks"]; ok {
        if arr, ok := v.([]any); ok {
            tasks = make([]Task, 0, len(arr))
            for _, it := range arr {
                if m, ok := it.(map[string]any); ok {
                    tasks = append(tasks, m)
                }
            }
            return tasks, nil
        }
    }
    return nil, nil
}


