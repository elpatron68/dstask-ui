package server

import (
    "fmt"
    "html/template"
    "net/http"
    "strings"
    "time"

    "github.com/mbusc/dstask-ui/internal/auth"
    "github.com/mbusc/dstask-ui/internal/config"
    "github.com/mbusc/dstask-ui/internal/dstask"
)

type Server struct {
    userStore auth.UserStore
    mux       *http.ServeMux
    layoutTpl *template.Template
    cfg       *config.Config
    runner    *dstask.Runner
}

func NewServer(userStore auth.UserStore) *Server {
    return NewServerWithConfig(userStore, config.Default())
}

func NewServerWithConfig(userStore auth.UserStore, cfg *config.Config) *Server {
    s := &Server{userStore: userStore, cfg: cfg}
    s.runner = dstask.NewRunner(cfg)
    s.mux = http.NewServeMux()

    // Templates: minimal inline for MVP skeleton; real templates folgen in späteren To-dos
    s.layoutTpl = template.Must(template.New("layout").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>dstask</title>
<style>
body{font-family:system-ui,-apple-system,Segoe UI,Roboto,Ubuntu,Helvetica,Arial,sans-serif;margin:16px}
nav a{padding:6px 10px; text-decoration:none; color:#0366d6; border-radius:4px}
nav a.active{background:#0366d6;color:#fff}
nav{margin-bottom:12px}
</style>
</head><body>
<nav>
  <a href="/next?html=1" class="{{if eq .Active "next"}}active{{end}}">Next</a>
  <a href="/open?html=1" class="{{if eq .Active "open"}}active{{end}}">Open</a>
  <a href="/active?html=1" class="{{if eq .Active "active"}}active{{end}}">Active</a>
  <a href="/paused?html=1" class="{{if eq .Active "paused"}}active{{end}}">Paused</a>
  <a href="/resolved?html=1" class="{{if eq .Active "resolved"}}active{{end}}">Resolved</a>
  <a href="/tags" class="{{if eq .Active "tags"}}active{{end}}">Tags</a>
  <a href="/projects" class="{{if eq .Active "projects"}}active{{end}}">Projects</a>
  <a href="/context" class="{{if eq .Active "context"}}active{{end}}">Context</a>
  <a href="/tasks/new" class="{{if eq .Active "new"}}active{{end}}">New task</a>
  <a href="/tasks/action" class="{{if eq .Active "action"}}active{{end}}">Actions</a>
  <a href="/version" class="{{if eq .Active "version"}}active{{end}}">Version</a>
</nav>
{{ template "content" . }}
</body></html>`))

    s.routes()
    return s
}

func (s *Server) routes() {
    s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        _, _ = w.Write([]byte("ok"))
    })

    s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        t := template.Must(s.layoutTpl.Clone())
        _, _ = t.New("content").Parse(`<h1>dstask Web UI</h1><p>Signed in as: {{.User}}</p><div><a href="/next?html=1">Next</a> | <a href="/open?html=1">Open</a> | <a href="/active?html=1">Active</a> | <a href="/paused?html=1">Paused</a> | <a href="/resolved?html=1">Resolved</a> | <a href="/tags">Tags</a> | <a href="/projects">Projects</a> | <a href="/context">Context</a> | <a href="/tasks/new">New task</a> | <a href="/tasks/action">Actions</a> | <a href="/version">Version</a></div>
<form method="post" action="/sync" style="margin-top:8px"><button type="submit">Sync</button></form>`) // placeholder
        username, _ := auth.UsernameFromRequest(r)
        _ = t.Execute(w, map[string]any{"User": username, "Active": activeFromPath(r.URL.Path)})
    })

    s.mux.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        if r.URL.Query().Get("html") == "1" {
            exp := s.runner.Run(username, 5_000_000_000, "export")
            if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
                if tasks, ok := decodeTasksJSONFlexible(exp.Stdout); ok && len(tasks) > 0 {
                    rows := buildRowsFromTasks(tasks, "")
                    rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                    if len(rows) > 0 {
                        w.Header().Set("Content-Type", "text/html; charset=utf-8")
                        s.renderExportTable(w, r, "Next", rows)
                        return
                    }
                }
            }
            res := s.runner.Run(username, 5_000_000_000, "next")
            if res.Err != nil && !res.TimedOut {
                http.Error(w, res.Stderr, http.StatusBadGateway)
                return
            }
            // Versuch: JSON direkt aus next-Stdout extrahieren
            if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
                rows := buildRowsFromTasks(tasks2, "")
                rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                if len(rows) > 0 {
                    w.Header().Set("Content-Type", "text/html; charset=utf-8")
                    s.renderExportTable(w, r, "Next", rows)
                    return
                }
            }
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            s.renderListHTML(w, r, "Next", res.Stdout)
            return
        }
        res := s.runner.Run(username, 5_000_000_000, "next")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        _, _ = w.Write([]byte(res.Stdout))
    })

    s.mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        if r.URL.Query().Get("html") == "1" {
            // Primär: export rohen JSON-Text holen und parsen (robuster, da wir Json sehen)
            exp := s.runner.Run(username, 5_000_000_000, "export")
            if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
                if tasks, ok := decodeTasksJSON(exp.Stdout); ok && len(tasks) > 0 {
                    rows := make([]map[string]string, 0, len(tasks))
                    for _, t := range tasks {
                        // Zeige alle offenen und aktiven; resolved werden unten ggf. herausgefiltert
                        id := str(firstOf(t, "id", "ID", "Id", "uuid", "UUID"))
                        if id == "" { continue }
                        rows = append(rows, map[string]string{
                            "id":       id,
                            "status":   str(firstOf(t, "status", "state")),
                            "summary":  trimQuotes(str(firstOf(t, "summary", "Summary", "description", "Description"))),
                            "project":  trimQuotes(str(firstOf(t, "project", "Project"))),
                            "priority": str(firstOf(t, "priority", "Priority")),
                            "due":      trimQuotes(str(firstOf(t, "due", "Due", "dueDate", "DueDate"))),
                            "tags":     joinTags(firstOf(t, "tags", "Tags")),
                        })
                    }
                    if len(rows) > 0 {
                w.Header().Set("Content-Type", "text/html; charset=utf-8")
                s.renderExportTable(w, r, "Open", rows)
                        return
                    }
                } else {
                    // Loose Parser über den Rohtext
                    rows := parseTasksLooseFromJSONText(exp.Stdout)
                    if len(rows) > 0 {
                        w.Header().Set("Content-Type", "text/html; charset=utf-8")
                        s.renderExportTable(w, r, "Open", rows)
                        return
                    }
                }
            }
            // Fallback: Plaintext parsen und als Tabelle rendern
            res := s.runner.Run(username, 5_000_000_000, "show-open")
            if res.Err != nil && !res.TimedOut {
                http.Error(w, res.Stderr, http.StatusBadGateway)
                return
            }
            // Versuche zuerst JSON aus show-open zu extrahieren (manche Builds geben JSON aus)
            if tasks2, ok := decodeTasksJSON(res.Stdout); ok && len(tasks2) > 0 {
                rows := make([]map[string]string, 0, len(tasks2))
                for _, t := range tasks2 {
                    id := str(firstOf(t, "id", "ID", "uuid"))
                    if id == "" { continue }
                    rows = append(rows, map[string]string{
                        "id":       id,
                        "status":   str(firstOf(t, "status", "state")),
                        "summary":  trimQuotes(str(firstOf(t, "summary", "description"))),
                        "project":  trimQuotes(str(firstOf(t, "project"))),
                        "priority": str(firstOf(t, "priority")),
                        "due":      trimQuotes(str(firstOf(t, "due"))),
                        "tags":     joinTags(firstOf(t, "tags")),
                    })
                }
                if len(rows) > 0 {
                    w.Header().Set("Content-Type", "text/html; charset=utf-8")
                    s.renderExportTable(w, r, "Open", rows)
                    return
                }
            }
            rows := parseOpenPlain(res.Stdout)
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            if len(rows) > 0 {
                s.renderExportTable(w, r, "Open", rows)
            } else {
                ok := r.URL.Query().Get("ok") != ""
                // reuse list renderer with Ok flag
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            t := template.Must(s.layoutTpl.Clone())
            _, _ = t.New("content").Parse(`<h2>Open</h2>{{if .Ok}}<div style="background:#d4edda;border:1px solid #c3e6cb;color:#155724;padding:8px;margin-bottom:8px;">Action successful</div>{{end}}<pre style="white-space: pre-wrap;">{{.Body}}</pre>`)
            _ = t.Execute(w, map[string]any{"Ok": ok, "Body": res.Stdout, "Active": activeFromPath(r.URL.Path)})
            }
            return
        }
        // Plaintext
        res := s.runner.Run(username, 5_000_000_000, "show-open")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        _, _ = w.Write([]byte(res.Stdout))
    })

    s.mux.HandleFunc("/active", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        if r.URL.Query().Get("html") == "1" {
            exp := s.runner.Run(username, 5_000_000_000, "export")
            if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
                if tasks, ok := decodeTasksJSONFlexible(exp.Stdout); ok && len(tasks) > 0 {
                    rows := buildRowsFromTasks(tasks, "active")
                    rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                    if len(rows) > 0 {
                        w.Header().Set("Content-Type", "text/html; charset=utf-8")
                        s.renderExportTable(w, r, "Active", rows)
                        return
                    }
                }
            }
            res := s.runner.Run(username, 5_000_000_000, "show-active")
            if res.Err != nil && !res.TimedOut {
                http.Error(w, res.Stderr, http.StatusBadGateway)
                return
            }
            // Versuch: JSON direkt aus show-active-Stdout extrahieren
            if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
                rows := buildRowsFromTasks(tasks2, "active")
                rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                if len(rows) > 0 {
                    w.Header().Set("Content-Type", "text/html; charset=utf-8")
                    s.renderExportTable(w, r, "Active", rows)
                    return
                }
            }
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            s.renderListHTML(w, r, "Active", res.Stdout)
            return
        }
        res := s.runner.Run(username, 5_000_000_000, "show-active")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        _, _ = w.Write([]byte(res.Stdout))
    })

    s.mux.HandleFunc("/paused", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        if r.URL.Query().Get("html") == "1" {
            exp := s.runner.Run(username, 5_000_000_000, "export")
            if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
                if tasks, ok := decodeTasksJSON(exp.Stdout); ok && len(tasks) > 0 {
                    rows := buildRowsFromTasks(tasks, "paused")
                    rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                    if len(rows) > 0 {
                        w.Header().Set("Content-Type", "text/html; charset=utf-8")
                        s.renderExportTable(w, r, "Paused", rows)
                        return
                    }
                }
            }
            res := s.runner.Run(username, 5_000_000_000, "show-paused")
            if res.Err != nil && !res.TimedOut {
                http.Error(w, res.Stderr, http.StatusBadGateway)
                return
            }
            // Versuch: JSON direkt aus show-paused-Stdout extrahieren
            if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
                rows := buildRowsFromTasks(tasks2, "paused")
                rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                if len(rows) > 0 {
                    w.Header().Set("Content-Type", "text/html; charset=utf-8")
                    s.renderExportTable(w, r, "Paused", rows)
                    return
                }
            }
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            s.renderListHTML(w, r, "Paused", res.Stdout)
            return
        }
        res := s.runner.Run(username, 5_000_000_000, "show-paused")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        _, _ = w.Write([]byte(res.Stdout))
    })

    s.mux.HandleFunc("/resolved", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        if r.URL.Query().Get("html") == "1" {
            exp := s.runner.Run(username, 5_000_000_000, "export")
            if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
                if tasks, ok := decodeTasksJSON(exp.Stdout); ok && len(tasks) > 0 {
                    rows := buildRowsFromTasks(tasks, "resolved")
                    rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                    if len(rows) > 0 {
                        w.Header().Set("Content-Type", "text/html; charset=utf-8")
                        s.renderExportTable(w, r, "Resolved", rows)
                        return
                    }
                }
            }
            res := s.runner.Run(username, 5_000_000_000, "show-resolved")
            if res.Err != nil && !res.TimedOut {
                http.Error(w, res.Stderr, http.StatusBadGateway)
                return
            }
            // Versuch: JSON direkt aus show-resolved-Stdout extrahieren
            if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
                rows := buildRowsFromTasks(tasks2, "resolved")
                rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
                if len(rows) > 0 {
                    w.Header().Set("Content-Type", "text/html; charset=utf-8")
                    s.renderExportTable(w, r, "Resolved", rows)
                    return
                }
            }
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            s.renderListHTML(w, r, "Resolved", res.Stdout)
            return
        }
        res := s.runner.Run(username, 5_000_000_000, "show-resolved")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        _, _ = w.Write([]byte(res.Stdout))
    })

    s.mux.HandleFunc("/tags", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        res := s.runner.Run(username, 5_000_000_000, "show-tags")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        if r.URL.Query().Get("raw") != "1" {
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            t := template.Must(s.layoutTpl.Clone())
            _, _ = t.New("content").Parse(`<h2>Tags</h2>
<pre style="white-space:pre-wrap;">{{.Out}}</pre>`) 
            _ = t.Execute(w, map[string]any{"Out": strings.TrimSpace(res.Stdout), "Active": activeFromPath(r.URL.Path)})
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        out := strings.TrimSpace(res.Stdout)
        if out == "" { out = "Keine Tags vorhanden" }
        _, _ = w.Write([]byte(out))
    })

    s.mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        res := s.runner.Run(username, 5_000_000_000, "show-projects")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        if r.URL.Query().Get("raw") != "1" {
            // Versuche JSON zu erkennen und als Tabelle zu rendern
            if arr, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(arr) > 0 {
                rows := make([]map[string]string, 0, len(arr))
                for _, m := range arr {
                    name := trimQuotes(str(firstOf(m, "name", "project")))
                    if name == "" { continue }
                    rows = append(rows, map[string]string{
                        "name":          name,
                        "taskCount":     str(firstOf(m, "taskCount")),
                        "resolvedCount": str(firstOf(m, "resolvedCount")),
                        "active":        str(firstOf(m, "active")),
                        "priority":      str(firstOf(m, "priority")),
                    })
                }
                if len(rows) > 0 {
                    w.Header().Set("Content-Type", "text/html; charset=utf-8")
                    s.renderProjectsTable(w, r, "Projects", rows)
                    return
                }
            }
            // Fallback: HTML mit Pre
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            t := template.Must(s.layoutTpl.Clone())
            _, _ = t.New("content").Parse(`<h2>Projects</h2>
<pre style="white-space:pre-wrap;">{{.Out}}</pre>`) 
            _ = t.Execute(w, map[string]any{"Out": strings.TrimSpace(res.Stdout), "Active": activeFromPath(r.URL.Path)})
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        out := strings.TrimSpace(res.Stdout)
        if out == "" { out = "Keine Projekte vorhanden" }
        _, _ = w.Write([]byte(out))
    })

    // Context anzeigen/setzen
    s.mux.HandleFunc("/context", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            username, _ := auth.UsernameFromRequest(r)
            res := s.runner.Run(username, 5_000_000_000, "context")
            if res.Err != nil && !res.TimedOut {
                http.Error(w, res.Stderr, http.StatusBadGateway)
                return
            }
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            t := template.Must(s.layoutTpl.Clone())
            _, _ = t.New("content").Parse(`<h2>Context</h2>
<pre>{{.Out}}</pre>
<form method="post" action="/context">
  <div><label>New context (e.g. +work project:dstask): <input name="value"></label></div>
  <div>
    <button type="submit">Apply</button>
    <button type="submit" name="clear" value="1">Clear</button>
  </div>
</form>`)
            _ = t.Execute(w, map[string]any{"Out": strings.TrimSpace(res.Stdout), "Active": activeFromPath(r.URL.Path)})
        case http.MethodPost:
            if err := r.ParseForm(); err != nil {
                http.Error(w, "invalid form", http.StatusBadRequest)
                return
            }
            username, _ := auth.UsernameFromRequest(r)
            if r.FormValue("clear") == "1" {
                res := s.runner.Run(username, 5_000_000_000, "context", "none")
                if res.Err != nil && !res.TimedOut {
                    http.Error(w, res.Stderr, http.StatusBadRequest)
                    return
                }
                http.Redirect(w, r, "/context", http.StatusSeeOther)
                return
            }
            val := strings.TrimSpace(r.FormValue("value"))
            if val == "" {
                http.Error(w, "value required", http.StatusBadRequest)
                return
            }
            res := s.runner.Run(username, 5_000_000_000, "context", val)
            if res.Err != nil && !res.TimedOut {
                http.Error(w, res.Stderr, http.StatusBadRequest)
                return
            }
            http.Redirect(w, r, "/context", http.StatusSeeOther)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    })

    // Task erstellen (Form)
    s.mux.HandleFunc("/tasks/new", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        // Fetch existing projects and tags
        projRes := s.runner.Run(username, 5_000_000_000, "show-projects")
        tagRes := s.runner.Run(username, 5_000_000_000, "show-tags")
        projects := parseProjectsFromOutput(projRes.Stdout)
        tags := parseTagsFromOutput(tagRes.Stdout)
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        t := template.Must(s.layoutTpl.Clone())
        _, _ = t.New("content").Parse(`
<h2>New task</h2>
<form method="post" action="/tasks">
  <div><label>Summary: <input name="summary" required style="width:60%"></label></div>
  <div>
    <label>Project:</label>
    <select name="projectSelect">
      <option value="">(none)</option>
      {{range .Projects}}<option value="{{.}}">{{.}}</option>{{end}}
    </select>
    <span style="margin:0 8px;">or</span>
    <input name="project" placeholder="new project" />
  </div>
  <div>
    <label>Tags:</label>
    <div style="max-height:140px;overflow:auto;border:1px solid #ddd;padding:6px;">
      {{range .Tags}}<label style="display:inline-block;margin-right:8px;">
        <input type="checkbox" name="tagsExisting" value="{{.}}"/> {{.}}
      </label>{{end}}
    </div>
    <div style="margin-top:6px;">
      <label>Add tags (comma-separated): <input name="tags"></label>
    </div>
  </div>
  <div>
    <label>Due:</label>
    <input type="date" name="dueDate" />
    <span style="margin:0 8px;">or</span>
    <input name="due" placeholder="e.g. friday / 2025-12-31" />
  </div>
  <div style="margin-top:8px;"><button type="submit">Create</button></div>
 </form>
        `)
        _ = t.Execute(w, map[string]any{"Active": activeFromPath(r.URL.Path), "Projects": projects, "Tags": tags})
    })

    // Task erstellen (POST)
    s.mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        if err := r.ParseForm(); err != nil {
            http.Error(w, "invalid form", http.StatusBadRequest)
            return
        }
        summary := strings.TrimSpace(r.FormValue("summary"))
        if summary == "" {
            http.Error(w, "summary required", http.StatusBadRequest)
            return
        }
        tags := strings.TrimSpace(r.FormValue("tags"))
        project := strings.TrimSpace(r.FormValue("project"))
        // Prefer selected existing project if provided
        if ps := strings.TrimSpace(r.FormValue("projectSelect")); ps != "" {
            project = ps
        }
        // Due: prefer date picker if present
        dueDate := strings.TrimSpace(r.FormValue("dueDate"))
        due := strings.TrimSpace(r.FormValue("due"))
        if dueDate != "" {
            due = dueDate
        }

        // Compose args per dstask add Syntax: add <summary tokens...> +tags project: due:
        args := []string{"add"}
        args = append(args, summaryTokens(summary)...)
        // Collect tags from existing checkboxes
        for _, t := range r.Form["tagsExisting"] {
            t = strings.TrimSpace(t)
            if t == "" { continue }
            t = normalizeTag(t)
            if !strings.HasPrefix(t, "+") { args = append(args, "+"+t) } else { args = append(args, t) }
        }
        if tags != "" {
            for _, t := range strings.Split(tags, ",") {
                t = strings.TrimSpace(t)
                if t == "" { continue }
                t = normalizeTag(t)
                if strings.HasPrefix(t, "+") { // allow user to prefix (rare)
                    args = append(args, t)
                } else {
                    args = append(args, "+"+t)
                }
            }
        }
        if project != "" {
            args = append(args, "project:"+quoteIfNeeded(project))
        }
        if due != "" {
            args = append(args, "due:"+quoteIfNeeded(due))
        }
        username, _ := auth.UsernameFromRequest(r)
        res := s.runner.Run(username, 10_000_000_000, args...) // 10s
        if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadRequest)
            return
        }
        http.Redirect(w, r, "/open?html=1&ok=created", http.StatusSeeOther)
    })

    // Task Aktionen: /tasks/{id}/start|stop|done|remove|log|note
    s.mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
        // Erwartet Pfad wie /tasks/123/start
        id, action := parseTaskAction(r.URL.Path)
        if id == "" || action == "" {
            http.NotFound(w, r)
            return
        }
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        username, _ := auth.UsernameFromRequest(r)
        var res dstask.Result
        timeout := 10 * time.Second
        switch action {
        case "start", "stop", "done", "remove", "log":
            // Einige dstask-Versionen bevorzugen die Syntax: dstask <action> <id>
            res = s.runner.Run(username, timeout, action, id)
        case "note":
            if err := r.ParseForm(); err != nil {
                http.Error(w, "invalid form", http.StatusBadRequest)
                return
            }
            note := strings.TrimSpace(r.FormValue("note"))
            if note == "" {
                http.Error(w, "note required", http.StatusBadRequest)
                return
            }
            // Für note: Übergabe per stdin wäre ideal; dstask akzeptiert Interaktivität.
            // Vereinfachung: note Text direkt als Argument anhängen (dstask erlaubt dies mittels / ?)
            // Fallback: id note und dann / als Trenner mit Text
            // Wir nutzen: dstask <id> note <text>
            // Für note ebenfalls: dstask note <id> <text>
            res = s.runner.Run(username, timeout, "note", id, note)
        default:
            http.NotFound(w, r)
            return
        }
        if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadRequest)
            return
        }
        http.Redirect(w, r, "/open?html=1&ok=action", http.StatusSeeOther)
    })

    // Einfache Aktionsseite (UI-Politur): ID + Aktion auswählen
    s.mux.HandleFunc("/tasks/action", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            t := template.Must(s.layoutTpl.Clone())
            _, _ = t.New("content").Parse(`
<h2>Task actions</h2>
<form method="post" action="/tasks/submit">
  <div><label>Task ID: <input name="id" required></label></div>
  <div><label>Action:
    <select name="action">
      <option value="start">start</option>
      <option value="stop">stop</option>
      <option value="done">done</option>
      <option value="remove">remove</option>
      <option value="log">log</option>
      <option value="note">note</option>
    </select>
  </label></div>
  <div><label>Note (for action "note"):<br><textarea name="note" rows="3" cols="40"></textarea></label></div>
  <div><button type="submit">Execute</button></div>
</form>`)
            _ = t.Execute(w, map[string]any{"Active": activeFromPath(r.URL.Path)})
        case http.MethodPost:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    })

    // Submission der Aktionsseite
    s.mux.HandleFunc("/tasks/submit", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        if err := r.ParseForm(); err != nil {
            http.Error(w, "invalid form", http.StatusBadRequest)
            return
        }
        id := strings.TrimSpace(r.FormValue("id"))
        action := strings.TrimSpace(r.FormValue("action"))
        note := strings.TrimSpace(r.FormValue("note"))
        if id == "" || action == "" {
            http.Error(w, "id/action required", http.StatusBadRequest)
            return
        }
        username, _ := auth.UsernameFromRequest(r)
        timeout := 10 * time.Second
        var res dstask.Result
        switch action {
        case "start", "stop", "done", "remove", "log":
            res = s.runner.Run(username, timeout, action, id)
        case "note":
            if note == "" {
                http.Error(w, "note required", http.StatusBadRequest)
                return
            }
            res = s.runner.Run(username, timeout, "note", id, note)
        default:
            http.Error(w, "unknown action", http.StatusBadRequest)
            return
        }
        if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadRequest)
            return
        }
        http.Redirect(w, r, "/open?html=1&ok=action", http.StatusSeeOther)
    })

    // Task ändern (Project/Priority/Due/Tags)
    s.mux.HandleFunc("/tasks/modify", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        if err := r.ParseForm(); err != nil {
            http.Error(w, "invalid form", http.StatusBadRequest)
            return
        }
        id := strings.TrimSpace(r.FormValue("id"))
        if id == "" {
            http.Error(w, "id required", http.StatusBadRequest)
            return
        }
        project := strings.TrimSpace(r.FormValue("project"))
        priority := strings.TrimSpace(r.FormValue("priority"))
        due := strings.TrimSpace(r.FormValue("due"))
        addTags := strings.TrimSpace(r.FormValue("addTags"))
        removeTags := strings.TrimSpace(r.FormValue("removeTags"))

        args := []string{"modify"}
        if project != "" {
            args = append(args, "project:"+quoteIfNeeded(project))
        }
        if priority != "" {
            args = append(args, priority)
        }
        if due != "" {
            args = append(args, "due:"+quoteIfNeeded(due))
        }
        if addTags != "" {
            for _, t := range strings.Split(addTags, ",") {
                t = strings.TrimSpace(t)
                if t == "" { continue }
                t = normalizeTag(t)
                if !strings.HasPrefix(t, "+") { t = "+"+t }
                args = append(args, t)
            }
        }
        if removeTags != "" {
            for _, t := range strings.Split(removeTags, ",") {
                t = strings.TrimSpace(t)
                if t == "" { continue }
                t = normalizeTag(t)
                if !strings.HasPrefix(t, "-") { t = "-"+t }
                args = append(args, t)
            }
        }
        username, _ := auth.UsernameFromRequest(r)
        // dstask modify erwartet Syntax: dstask <id> modify ... (laut usage)
        full := append([]string{id}, args...)
        res := s.runner.Run(username, 10*time.Second, full...)
        if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadRequest)
            return
        }
        http.Redirect(w, r, "/open?html=1&ok=modified", http.StatusSeeOther)
    })

    // Version anzeigen
    s.mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
        username, _ := auth.UsernameFromRequest(r)
        res := s.runner.Run(username, 5_000_000_000, "version")
        if res.Err != nil && !res.TimedOut {
            http.Error(w, res.Stderr, http.StatusBadGateway)
            return
        }
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        _, _ = w.Write([]byte(strings.TrimSpace(res.Stdout)))
    })

    // Sync anzeigen/ausführen
    s.mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            t := template.Must(s.layoutTpl.Clone())
            _, _ = t.New("content").Parse(`<h2>Sync</h2>
<p>Runs <code>dstask sync</code> (pull, merge, push). The underlying repo must have a remote with an upstream branch.</p>
<form method="post" action="/sync"><button type="submit">Sync now</button></form>`)
            _ = t.Execute(w, nil)
        case http.MethodPost:
            username, _ := auth.UsernameFromRequest(r)
            res := s.runner.Run(username, 30_000_000_000, "sync") // 30s
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            t := template.Must(s.layoutTpl.Clone())
            // Erkennung gängiger Git-Fehler für hilfreiche Hinweise
            hint := ""
            out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
            if strings.Contains(out, "There is no tracking information for the current branch") {
                hint = `Es ist kein Upstream gesetzt. Setze ihn im .dstask-Repo, z. B.:<br>
<pre>git remote add origin &lt;REMOTE_URL&gt;
git push -u origin master</pre>`
            }
            status := "Erfolg"
            if res.Err != nil || res.ExitCode != 0 {
                status = "Fehler"
            }
            _, _ = t.New("content").Parse(`<h2>Sync: {{.Status}}</h2>
{{if .Hint}}<div style="background:#fff3cd;padding:8px;border:1px solid #ffeeba;margin-bottom:8px;">{{.Hint}}</div>{{end}}
<pre style="white-space: pre-wrap;">{{.Out}}</pre>
<p><a href="/open?html=1">Back to list</a></p>`) 
            _ = t.Execute(w, map[string]any{
                "Status": status,
                "Out":    out,
                "Hint":   template.HTML(hint),
                "Active": activeFromPath(r.URL.Path),
            })
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    })
}

// parseTaskAction extrahiert ID und Aktion aus Pfaden wie /tasks/123/start
func parseTaskAction(path string) (id, action string) {
    // TrimPrefix
    if !strings.HasPrefix(path, "/tasks/") {
        return "", ""
    }
    rest := strings.TrimPrefix(path, "/tasks/")
    parts := strings.Split(rest, "/")
    if len(parts) < 2 {
        return "", ""
    }
    id = strings.TrimSpace(parts[0])
    action = strings.TrimSpace(parts[1])
    if id == "" || action == "" {
        return "", ""
    }
    return id, action
}

func (s *Server) Handler() http.Handler {
    // Basic Auth für alle außer /healthz
    protected := http.NewServeMux()
    protected.HandleFunc("/", s.mux.ServeHTTP)

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/healthz" {
            s.mux.ServeHTTP(w, r)
            return
        }
        realm := fmt.Sprintf("dstask")
        authMiddleware := auth.BasicAuthMiddleware(s.userStore, realm, protected)
        authMiddleware.ServeHTTP(w, r)
    })
}


