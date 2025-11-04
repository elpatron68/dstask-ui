package dstask

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpatron68/dstask-ui/internal/config"
	applog "github.com/elpatron68/dstask-ui/internal/log"
	"gopkg.in/yaml.v3"
)

type Runner struct {
	cfg *config.Config
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
}

type Result struct {
	Stdout   string
	Stderr   string
	Err      error
	ExitCode int
	TimedOut bool
}

// RunWithStdin führt dstask mit gegebenen Argumenten und stdin-Input aus.
func (r *Runner) RunWithStdin(username string, timeout time.Duration, stdin string, args ...string) Result {
	bin := r.cfg.DstaskBin
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)

	// Starte mit vererbter Umgebung, damit PATH/GIT etc. vorhanden sind
	env := os.Environ()
	// HOME/USERPROFILE für den Nutzer ggf. überschreiben
	if home, ok := config.ResolveHomeForUsername(r.cfg, username); ok && home != "" {
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
		cmd.Dir = home
	}
	cmd.Env = env

	// Set stdin
	stdinReader := strings.NewReader(stdin)
	cmd.Stdin = stdinReader

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	applog.Infof("dstask run (stdin): %s %s, stdin length: %d, stdin preview: %q", bin, strings.Join(args, " "), len(stdin), truncate(stdin, 200))
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
	if res.ExitCode != 0 || res.TimedOut {
		applog.Warnf("dstask exit (stdin): code=%d timeout=%v stderr=%q", res.ExitCode, res.TimedOut, truncate(res.Stderr, 300))
	} else {
		applog.Debugf("dstask exit (stdin): code=%d", res.ExitCode)
	}
	return res
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

	applog.Debugf("dstask run: %s %s", bin, strings.Join(args, " "))
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
	if res.ExitCode != 0 || res.TimedOut {
		applog.Warnf("dstask exit: code=%d timeout=%v stderr=%q", res.ExitCode, res.TimedOut, truncate(res.Stderr, 300))
	} else {
		applog.Debugf("dstask exit: code=%d", res.ExitCode)
	}
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
	if len(s) <= max {
		return s
	}
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

// decodeTasksJSONFlexible unterstützt sowohl Arrays als auch einzelne Objekte.
func decodeTasksJSONFlexible(raw string) ([]map[string]any, bool) {
	// 1) Versuche direkt als Array
	var arr []any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&arr); err == nil {
		tasks := make([]map[string]any, 0, len(arr))
		for _, it := range arr {
			if m, ok := it.(map[string]any); ok {
				tasks = append(tasks, m)
			}
		}
		if len(tasks) > 0 {
			return tasks, true
		}
	}
	// 2) Versuche einzelnes Objekt
	var obj map[string]any
	dec2 := json.NewDecoder(strings.NewReader(raw))
	dec2.UseNumber()
	if err := dec2.Decode(&obj); err == nil && len(obj) > 0 {
		return []map[string]any{obj}, true
	}
	// 3) Heuristik: zwischen erstem '{' und letztem '}' ausschneiden und erneut versuchen
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
		dec3 := json.NewDecoder(strings.NewReader(s))
		dec3.UseNumber()
		obj = nil
		if err := dec3.Decode(&obj); err == nil && len(obj) > 0 {
			return []map[string]any{obj}, true
		}
	}
	return nil, false
}

// UpdateTaskNotesDirectly aktualisiert die Notes eines Tasks, indem die YAML-Datei direkt bearbeitet wird.
// Dies umgeht das Problem, dass dstask note einen interaktiven Editor öffnet.
func (r *Runner) UpdateTaskNotesDirectly(username string, taskID string, notes string) error {
	// 1. Hole UUID des Tasks (mit längerem Timeout)
	res := r.Run(username, 10*time.Second, taskID)
	if res.Err != nil {
		applog.Warnf("UpdateTaskNotesDirectly: failed to get task %s: %v (timeout=%v)", taskID, res.Err, res.TimedOut)
		return res.Err
	}
	if res.ExitCode != 0 {
		applog.Warnf("UpdateTaskNotesDirectly: dstask %s returned exit code %d, stderr=%q", taskID, res.ExitCode, truncate(res.Stderr, 200))
		return context.DeadlineExceeded
	}
	if res.TimedOut {
		applog.Warnf("UpdateTaskNotesDirectly: dstask %s timed out", taskID)
		return context.DeadlineExceeded
	}

	applog.Debugf("UpdateTaskNotesDirectly: dstask %s stdout (first 200 chars): %q", taskID, truncate(res.Stdout, 200))
	tasks, ok := decodeTasksJSONFlexible(res.Stdout)
	if !ok || len(tasks) == 0 {
		applog.Warnf("UpdateTaskNotesDirectly: failed to parse JSON from dstask %s output", taskID)
		return context.DeadlineExceeded
	}

	var taskUUID string
	for _, t := range tasks {
		if str(firstOf(t, "id", "ID", "Id")) == taskID {
			taskUUID = str(firstOf(t, "uuid", "UUID"))
			break
		}
	}
	if taskUUID == "" {
		applog.Warnf("UpdateTaskNotesDirectly: task %s UUID not found in response", taskID)
		return context.DeadlineExceeded
	}
	applog.Debugf("UpdateTaskNotesDirectly: found UUID %s for task %s", taskUUID, taskID)

	// 2. Finde YAML-Datei in .dstask Verzeichnis
	home, ok := config.ResolveHomeForUsername(r.cfg, username)
	if !ok || home == "" {
		// Fallback: Versuche, das Home-Verzeichnis aus dem letzten erfolgreichen dstask-Aufruf zu bestimmen
		// Wir können auch versuchen, es aus der Umgebungsvariable zu holen, aber das ist weniger zuverlässig
		applog.Warnf("UpdateTaskNotesDirectly: failed to resolve home for username %s (cfg.Repos=%v), trying fallback", username, r.cfg.Repos)

		// Fallback: Verwende dstask export, um zu sehen, wo dstask die Tasks speichert
		// Oder versuche, das Home-Verzeichnis aus der Umgebungsvariable zu holen
		if homeEnv := os.Getenv("HOME"); homeEnv != "" {
			home = homeEnv
		} else if homeEnv := os.Getenv("USERPROFILE"); homeEnv != "" {
			home = homeEnv
		} else {
			applog.Warnf("UpdateTaskNotesDirectly: no fallback home directory found")
			return context.DeadlineExceeded
		}
		applog.Infof("UpdateTaskNotesDirectly: using fallback home directory %s", home)
	}

	dstaskDir := home
	if !strings.HasSuffix(dstaskDir, ".dstask") {
		dstaskDir = filepath.Join(home, ".dstask")
	}
	applog.Debugf("UpdateTaskNotesDirectly: using dstask directory %s", dstaskDir)

	// Suche in allen Status-Ordnern
	statusDirs := []string{"active", "pending", "paused", "resolved"}
	var yamlPath string
	for _, status := range statusDirs {
		candidate := filepath.Join(dstaskDir, status, taskUUID+".yml")
		if _, err := os.Stat(candidate); err == nil {
			yamlPath = candidate
			applog.Debugf("UpdateTaskNotesDirectly: found YAML file at %s", yamlPath)
			break
		}
	}

	if yamlPath == "" {
		applog.Warnf("UpdateTaskNotesDirectly: YAML file not found for task %s (UUID: %s) in %s", taskID, taskUUID, dstaskDir)
		return context.DeadlineExceeded
	}

	// 3. Lade YAML-Datei
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}

	// 4. Parse YAML
	var taskData map[string]any
	if err := yaml.Unmarshal(data, &taskData); err != nil {
		return err
	}

	// 5. Aktualisiere Notes
	taskData["notes"] = notes

	// 6. Schreibe YAML zurück
	updatedData, err := yaml.Marshal(&taskData)
	if err != nil {
		return err
	}

	if err := os.WriteFile(yamlPath, updatedData, 0644); err != nil {
		return err
	}

	// 7. Führe git add und commit direkt aus (nicht über dstask git)
	// Wir müssen im .dstask Verzeichnis arbeiten
	relPath, err := filepath.Rel(dstaskDir, yamlPath)
	if err != nil {
		relPath = yamlPath // Fallback to absolute path if relative fails
	}
	gitCmd := exec.CommandContext(context.Background(), "git", "-C", dstaskDir, "add", relPath)
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
	gitCmd.Env = env
	if err := gitCmd.Run(); err != nil {
		applog.Warnf("git add failed for notes update: %v", err)
		// Continue anyway - dstask might handle this
	}

	gitCommitCmd := exec.CommandContext(context.Background(), "git", "-C", dstaskDir, "commit", "-m", "Update task notes via web UI")
	gitCommitCmd.Env = env
	if err := gitCommitCmd.Run(); err != nil {
		applog.Warnf("git commit failed for notes update: %v", err)
		// Continue anyway - might be no changes or other git issues
	}

	applog.Infof("UpdateTaskNotesDirectly: successfully updated notes for task %s (UUID: %s)", taskID, taskUUID)
	return nil
}

// Helper functions for parsing
func str(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case json.Number:
		return string(s)
	default:
		return ""
	}
}

func firstOf(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
}
