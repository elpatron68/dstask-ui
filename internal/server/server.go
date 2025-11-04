package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
	"github.com/elpatron68/dstask-ui/internal/dstask"
	applog "github.com/elpatron68/dstask-ui/internal/log"
	"github.com/elpatron68/dstask-ui/internal/ui"
)

type Server struct {
	userStore auth.UserStore
	mux       *http.ServeMux
	layoutTpl *template.Template
	cfg       *config.Config
	runner    *dstask.Runner
	cmdStore  *ui.CommandLogStore
	uiCfg     config.UIConfig
}

func NewServer(userStore auth.UserStore) *Server {
	return NewServerWithConfig(userStore, config.Default())
}

func NewServerWithConfig(userStore auth.UserStore, cfg *config.Config) *Server {
	s := &Server{userStore: userStore, cfg: cfg, uiCfg: cfg.UI}
	s.runner = dstask.NewRunner(cfg)
	s.mux = http.NewServeMux()
	s.cmdStore = ui.NewCommandLogStore(cfg.UI.CommandLogMax)

	// Templates: register helpers (e.g., split)
	baseTpl := template.New("layout").Funcs(template.FuncMap{
		"split": func(s, sep string) []string { return strings.Split(s, sep) },
	})
	s.layoutTpl = template.Must(baseTpl.Parse(`<!doctype html><html><head><meta charset="utf-8"><title>dstask</title>
<style>
body{font-family:system-ui,-apple-system,Segoe UI,Roboto,Ubuntu,Helvetica,Arial,sans-serif;margin:16px}
nav a{padding:6px 10px; text-decoration:none; color:#0366d6; border-radius:4px}
nav a.active{background:#0366d6;color:#fff}
nav{margin-bottom:12px}
.cmdlog{margin-top:16px;border-top:1px solid #eee;padding-top:8px}
.cmdlog .hdr{display:flex;justify-content:space-between;align-items:center}
.cmdlog pre{background:#fff;border:1px solid #d0d7de;padding:8px;max-height:160px;overflow:auto}
.cmdlog .ts{color:#6a737d}
.cmdlog .cmd{color:#24292e;font-weight:600}
.cmdlog .ctx{color:#111827;font-weight:600}
table{border-collapse:collapse;width:100%}
thead th{position:sticky;top:0;background:#f6f8fa;border-bottom:1px solid #d0d7de}
tbody tr:nth-child(even){background:#f9fbfd}
.table-mono, .table-mono th, .table-mono td, table, th, td {font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace}
table, th, td, table pre {font-size:13px}
.badge{display:inline-block;padding:2px 6px;border-radius:12px;font-size:inherit;line-height:1}
.badge.status.active{background:#dcfce7;color:#166534}
.badge.status.pending{background:#e0e7ff;color:#3730a3}
.badge.status.paused{background:#fef3c7;color:#92400e}
.badge.status.resolved{background:#e5e7eb;color:#374151}
.badge.prio{background:#eef2ff;color:#1f2937}
.badge.prio.P0{background:#fee2e2;color:#991b1b}
.badge.prio.P1{background:#ffedd5;color:#9a3412}
.badge.prio.P2{background:#dbeafe;color:#1e3a8a}
.badge.prio.P3{background:#e5e7eb;color:#374151}
.pill{display:inline-block;padding:2px 6px;border-radius:999px;background:#e5e7eb;color:#374151;margin-right:6px;font-size:inherit}
.due.overdue{color:#991b1b;font-weight:600}
</style>
</head><body>
<nav>
  <a href="/" class="{{if eq .Active "home"}}active{{end}}">Home</a>
  <a href="/next?html=1" class="{{if eq .Active "next"}}active{{end}}">Next</a>
  <a href="/open?html=1" class="{{if eq .Active "open"}}active{{end}}">Open</a>
  <a href="/active?html=1" class="{{if eq .Active "active"}}active{{end}}">Active</a>
  <a href="/paused?html=1" class="{{if eq .Active "paused"}}active{{end}}">Paused</a>
  <a href="/resolved?html=1" class="{{if eq .Active "resolved"}}active{{end}}">Resolved</a>
  <a href="/tags" class="{{if eq .Active "tags"}}active{{end}}">Tags</a>
  <a href="/projects" class="{{if eq .Active "projects"}}active{{end}}">Projects</a>
  <a href="/templates" class="{{if eq .Active "templates"}}active{{end}}">Templates</a>
  <a href="/context" class="{{if eq .Active "context"}}active{{end}}">Context</a>
  <a href="/tasks/new" class="{{if eq .Active "new"}}active{{end}}">New task</a>
  <a href="/tasks/action" class="{{if eq .Active "action"}}active{{end}}">Actions</a>
  <a href="/version" class="{{if eq .Active "version"}}active{{end}}">Version</a>
</nav>
{{ template "content" . }}
{{ if .ShowCmdLog }}
<div class="cmdlog">
  <div class="hdr">
    <strong>Recent dstask commands</strong>
    <div>
      <a href="/__cmdlog?show=0&return={{.ReturnURL}}">Hide</a>
      {{if .CanShowMore}} | <a href="{{.MoreURL}}">Show more</a>{{end}}
    </div>
  </div>
  <pre>{{range .CmdEntries}}<span class="ts">{{.When}}</span> — <span class="ctx">{{.Context}}:</span> <span class="cmd">dstask {{.Args}}</span>
{{end}}</pre>
</div>
{{ else }}
<div class="cmdlog">
  <a href="/__cmdlog?show=1&return={{.ReturnURL}}">Show recent dstask commands</a>
</div>
{{ end }}
{{ if .Flash }}
<div class="flash {{.Flash.Type}}" style="margin:10px 0;padding:8px;border:1px solid #d0d7de; border-left-width:4px; background:#fff;">
  {{.Flash.Text}}
</div>
{{ end }}
</body></html>`))

	s.routes()
	return s
}

func (s *Server) routes() {
	// Batch actions
	s.mux.HandleFunc("/tasks/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		ids := r.Form["ids"]
		action := strings.TrimSpace(r.FormValue("action"))
		note := strings.TrimSpace(r.FormValue("note"))
		if len(ids) == 0 || action == "" {
			http.Error(w, "ids/action required", http.StatusBadRequest)
			return
		}
		username, _ := auth.UsernameFromRequest(r)
		var ok, skipped, failed int
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			var res dstask.Result
			switch action {
			case "start", "stop", "done", "remove", "log":
				res = s.runner.Run(username, 10*time.Second, action, id)
			case "note":
				if note == "" {
					skipped++
					continue
				}
				res = s.runner.Run(username, 10*time.Second, "note", id, note)
			default:
				skipped++
				continue
			}
			if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
				failed++
			} else {
				ok++
			}
		}
		msg := fmt.Sprintf("Batch %s: %d ok, %d skipped, %d failed", action, ok, skipped, failed)
		s.setFlash(w, "info", msg)
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
	})
	// Toggle command log visibility via cookie
	s.mux.HandleFunc("/__cmdlog", func(w http.ResponseWriter, r *http.Request) {
		show := r.URL.Query().Get("show")
		ret := r.URL.Query().Get("return")
		if show == "0" {
			http.SetCookie(w, &http.Cookie{Name: "cmdlog", Value: "off", Path: "/", MaxAge: 86400 * 365})
		} else if show == "1" {
			http.SetCookie(w, &http.Cookie{Name: "cmdlog", Value: "on", Path: "/", MaxAge: 86400 * 365})
		}
		if ret == "" {
			ret = "/"
		}
		http.Redirect(w, r, ret, http.StatusSeeOther)
	})
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`<h1>dstask Web UI</h1><p>Signed in as: {{.User}}</p>
<form method="post" action="/sync" style="margin-top:8px"><button type="submit">Sync</button></form>`) // placeholder
		username, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, username)
		_ = t.Execute(w, map[string]any{
			"User":        username,
			"Active":      activeFromPath(r.URL.Path),
			"Flash":       s.getFlash(r),
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
	})

	s.mux.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		s.cmdStore.Append(username, "List next tasks", []string{"next"})
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
		s.cmdStore.Append(username, "List open tasks", []string{"show-open"})
		if r.URL.Query().Get("html") == "1" {
			// Primär: export rohen JSON-Text holen und parsen (robuster, da wir Json sehen)
			exp := s.runner.Run(username, 5_000_000_000, "export")
			if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
				if tasks, ok := decodeTasksJSON(exp.Stdout); ok && len(tasks) > 0 {
					rows := make([]map[string]string, 0, len(tasks))
					for _, t := range tasks {
						// Zeige alle offenen und aktiven; resolved werden unten ggf. herausgefiltert
						id := str(firstOf(t, "id", "ID", "Id", "uuid", "UUID"))
						if id == "" {
							continue
						}
						rows = append(rows, map[string]string{
							"id":       id,
							"status":   str(firstOf(t, "status", "state")),
							"summary":  trimQuotes(str(firstOf(t, "summary", "Summary", "description", "Description"))),
							"project":  trimQuotes(str(firstOf(t, "project", "Project"))),
							"priority": str(firstOf(t, "priority", "Priority")),
							"due":      trimQuotes(str(firstOf(t, "due", "Due", "dueDate", "DueDate"))),
							"created":  trimQuotes(str(firstOf(t, "created", "Created"))),
							"resolved": trimQuotes(str(firstOf(t, "resolved", "Resolved"))),
							"age":      ageInDays(trimQuotes(str(firstOf(t, "created", "Created")))),
							"tags":     joinTags(firstOf(t, "tags", "Tags")),
						})
					}
					dueFilter := buildDueFilterToken(r.URL.Query())
					rows = applyDueFilter(rows, dueFilter)
					if len(rows) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						s.renderExportTable(w, r, "Open", rows)
						return
					}
				} else {
					// Loose Parser über den Rohtext
					rows := parseTasksLooseFromJSONText(exp.Stdout)
					rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
					dueFilter := buildDueFilterToken(r.URL.Query())
					rows = applyDueFilter(rows, dueFilter)
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
					if id == "" {
						continue
					}
					rows = append(rows, map[string]string{
						"id":       id,
						"status":   str(firstOf(t, "status", "state")),
						"summary":  trimQuotes(str(firstOf(t, "summary", "description"))),
						"project":  trimQuotes(str(firstOf(t, "project"))),
						"priority": str(firstOf(t, "priority")),
						"due":      trimQuotes(str(firstOf(t, "due"))),
						"created":  trimQuotes(str(firstOf(t, "created"))),
						"resolved": trimQuotes(str(firstOf(t, "resolved"))),
						"age":      ageInDays(trimQuotes(str(firstOf(t, "created")))),
						"tags":     joinTags(firstOf(t, "tags")),
					})
				}
				rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
				dueFilter := buildDueFilterToken(r.URL.Query())
				rows = applyDueFilter(rows, dueFilter)
				if len(rows) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					s.renderExportTable(w, r, "Open", rows)
					return
				}
			}
			rows := parseOpenPlain(res.Stdout)
			rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
			dueFilter := buildDueFilterToken(r.URL.Query())
			rows = applyDueFilter(rows, dueFilter)
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
		s.cmdStore.Append(username, "List active tasks", []string{"show-active"})
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
		s.cmdStore.Append(username, "List paused tasks", []string{"show-paused"})
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
		s.cmdStore.Append(username, "List resolved tasks", []string{"show-resolved"})
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
		s.cmdStore.Append(username, "List tags", []string{"show-tags"})
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		if r.URL.Query().Get("raw") != "1" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			_, _ = t.New("content").Parse(`<h2>Tags</h2>
<pre style="white-space:pre-wrap;">{{.Out}}</pre>`)
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Out":         strings.TrimSpace(res.Stdout),
				"Active":      activeFromPath(r.URL.Path),
				"Flash":       s.getFlash(r),
				"ShowCmdLog":  show,
				"CmdEntries":  entries,
				"MoreURL":     moreURL,
				"CanShowMore": canMore,
				"ReturnURL":   ret,
			})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		out := strings.TrimSpace(res.Stdout)
		if out == "" {
			out = "Keine Tags vorhanden"
		}
		_, _ = w.Write([]byte(out))
	})

	s.mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 5_000_000_000, "show-projects")
		s.cmdStore.Append(username, "List projects", []string{"show-projects"})
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
					if name == "" {
						continue
					}
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
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Out":         strings.TrimSpace(res.Stdout),
				"Active":      activeFromPath(r.URL.Path),
				"Flash":       s.getFlash(r),
				"ShowCmdLog":  show,
				"CmdEntries":  entries,
				"MoreURL":     moreURL,
				"CanShowMore": canMore,
				"ReturnURL":   ret,
			})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		out := strings.TrimSpace(res.Stdout)
		if out == "" {
			out = "Keine Projekte vorhanden"
		}
		_, _ = w.Write([]byte(out))
	})

	// Templates anzeigen
	s.mux.HandleFunc("/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// POST: Template erstellen
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			summary := strings.TrimSpace(r.FormValue("summary"))
			if summary == "" {
				http.Error(w, "summary required", http.StatusBadRequest)
				return
			}
			username, _ := auth.UsernameFromRequest(r)
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

			// Compose args per dstask template Syntax: template <summary tokens...> +tags project: due:
			args := []string{"template"}
			args = append(args, summaryTokens(summary)...)
			// Collect tags from existing checkboxes
			for _, t := range r.Form["tagsExisting"] {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "+") {
					args = append(args, "+"+t)
				} else {
					args = append(args, t)
				}
			}
			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					t = normalizeTag(t)
					if !strings.HasPrefix(t, "+") {
						args = append(args, "+"+t)
					} else {
						args = append(args, t)
					}
				}
			}
			if project != "" {
				args = append(args, "project:"+project)
			}
			if due != "" {
				args = append(args, "due:"+quoteIfNeeded(due))
			}

			res := s.runner.Run(username, 10_000_000_000, args...) // 10s
			s.cmdStore.Append(username, "Create template", args)
			if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
				applog.Warnf("template creation failed: code=%d timeout=%v err=%v", res.ExitCode, res.TimedOut, res.Err)
				s.setFlash(w, "error", "Template creation failed: "+res.Stderr)
				http.Redirect(w, r, "/templates/new", http.StatusSeeOther)
				return
			}
			applog.Infof("template created successfully")
			s.setFlash(w, "success", "Template created successfully")
			http.Redirect(w, r, "/templates", http.StatusSeeOther)
			return
		}

		// GET: Templates anzeigen
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 5_000_000_000, "show-templates")
		s.cmdStore.Append(username, "List templates", []string{"show-templates"})
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		templates := parseTemplatesFromOutput(res.Stdout)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`
<h2>Templates <a href="/templates/new" style="font-size:14px;font-weight:normal;margin-left:8px;">(New template)</a></h2>
{{if .Templates}}
<table border="1" cellpadding="4" cellspacing="0">
  <thead><tr>
    <th style="width:64px;">ID</th>
    <th>Summary</th>
    <th>Project</th>
    <th>Tags</th>
    <th style="width:160px;">Actions</th>
  </tr></thead>
  <tbody>
  {{range .Templates}}
    <tr>
      <td>{{index . "id"}}</td>
      <td><pre style="margin:0;white-space:pre-wrap;">{{index . "summary"}}</pre></td>
      <td>{{index . "project"}}</td>
      <td>{{index . "tags"}}</td>
      <td><a href="/tasks/new?template={{index . "id"}}">Use template</a></td>
    </tr>
  {{end}}
  </tbody>
</table>
{{else}}
<p>No templates found.</p>
{{end}}
`)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Templates":   templates,
			"Active":      activeFromPath(r.URL.Path),
			"Flash":       s.getFlash(r),
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
	})

	// Template erstellen (Form)
	s.mux.HandleFunc("/templates/new", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		// Fetch existing projects and tags
		projRes := s.runner.Run(username, 5_000_000_000, "show-projects")
		tagRes := s.runner.Run(username, 5_000_000_000, "show-tags")
		projects := parseProjectsFromOutput(projRes.Stdout)
		tags := parseTagsFromOutput(tagRes.Stdout)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`
<h2>New template</h2>
<form method="post" action="/templates">
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
  <div style="margin-top:8px;">
    <button type="submit">Create template</button>
    <a href="/templates" style="margin-left:8px;">Cancel</a>
  </div>
 </form>
        `)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Active":      activeFromPath(r.URL.Path),
			"Projects":    projects,
			"Tags":        tags,
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
	})

	// Context anzeigen/setzen
	s.mux.HandleFunc("/context", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			username, _ := auth.UsernameFromRequest(r)
			res := s.runner.Run(username, 5_000_000_000, "context")
			s.cmdStore.Append(username, "Show context", []string{"context"})
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
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Out":         strings.TrimSpace(res.Stdout),
				"Active":      activeFromPath(r.URL.Path),
				"Flash":       s.getFlash(r),
				"ShowCmdLog":  show,
				"CmdEntries":  entries,
				"MoreURL":     moreURL,
				"CanShowMore": canMore,
				"ReturnURL":   ret,
			})
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			username, _ := auth.UsernameFromRequest(r)
			if r.FormValue("clear") == "1" {
				res := s.runner.Run(username, 5_000_000_000, "context", "none")
				s.cmdStore.Append(username, "Clear context", []string{"context", "none"})
				if res.Err != nil && !res.TimedOut {
					s.setFlash(w, "error", "Failed to clear context")
					http.Redirect(w, r, "/context", http.StatusSeeOther)
					return
				}
				s.setFlash(w, "success", "Context cleared")
				http.Redirect(w, r, "/context", http.StatusSeeOther)
				return
			}
			val := strings.TrimSpace(r.FormValue("value"))
			if val == "" {
				http.Error(w, "value required", http.StatusBadRequest)
				return
			}
			res := s.runner.Run(username, 5_000_000_000, "context", val)
			s.cmdStore.Append(username, "Set context", []string{"context", val})
			if res.Err != nil && !res.TimedOut {
				s.setFlash(w, "error", "Failed to set context")
				http.Redirect(w, r, "/context", http.StatusSeeOther)
				return
			}
			s.setFlash(w, "success", "Context set")
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
		tmplRes := s.runner.Run(username, 5_000_000_000, "show-templates")
		projects := parseProjectsFromOutput(projRes.Stdout)
		tags := parseTagsFromOutput(tagRes.Stdout)
		templates := parseTemplatesFromOutput(tmplRes.Stdout)
		selectedTemplate := r.URL.Query().Get("template")
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
  <div>
    <label>Template:</label>
    <select name="template">
      <option value="">(none)</option>
      {{range .Templates}}<option value="{{index . "id"}}" {{if eq $.SelectedTemplate (index . "id")}}selected{{end}}>#{{index . "id"}}: {{index . "summary"}}</option>{{end}}
    </select>
  </div>
  <div style="margin-top:8px;"><button type="submit">Create</button></div>
 </form>
        `)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Active": activeFromPath(r.URL.Path), "Projects": projects, "Tags": tags, "Templates": templates, "SelectedTemplate": selectedTemplate,
			"ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
		})
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
			if t == "" {
				continue
			}
			t = normalizeTag(t)
			if !strings.HasPrefix(t, "+") {
				args = append(args, "+"+t)
			} else {
				args = append(args, t)
			}
		}
		if tags != "" {
			for _, t := range strings.Split(tags, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
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
		// Template support
		if templateID := strings.TrimSpace(r.FormValue("template")); templateID != "" {
			args = append(args, "template:"+templateID)
		}
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 10_000_000_000, args...) // 10s
		s.cmdStore.Append(username, "New task", append([]string{"add"}, args[1:]...))
		if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
			s.setFlash(w, "error", "Failed to create task")
			http.Redirect(w, r, "/tasks/new", http.StatusSeeOther)
			return
		}
		s.setFlash(w, "success", "Task created")
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
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
			s.cmdStore.Append(username, "Task action", []string{action, id})
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
			s.cmdStore.Append(username, "Task action", []string{"note", id})
		default:
			http.NotFound(w, r)
			return
		}
		if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
			applog.Warnf("action %s failed for id=%s: code=%d timeout=%v err=%v", action, id, res.ExitCode, res.TimedOut, res.Err)
			s.setFlash(w, "error", "Task action failed")
			http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
			return
		}
		applog.Infof("action %s succeeded for id=%s", action, id)
		s.setFlash(w, "success", "Task action applied")
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
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
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Active":     activeFromPath(r.URL.Path),
				"ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
			})
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
		s.setFlash(w, "success", "Task action applied")
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
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
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "+") {
					t = "+" + t
				}
				args = append(args, t)
			}
		}
		if removeTags != "" {
			for _, t := range strings.Split(removeTags, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "-") {
					t = "-" + t
				}
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
		s.setFlash(w, "success", "Task modified")
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
	})

	// Version anzeigen
	s.mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 5_000_000_000, "version")
		s.cmdStore.Append(username, "Show version", []string{"version"})
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		out := strings.TrimSpace(res.Stdout)
		if out == "" {
			out = "Unknown"
		}
		if r.URL.Query().Get("raw") == "1" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(out))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`<h2>dstask version</h2>
<pre style="white-space:pre-wrap;">{{.Out}}</pre>`)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Out":         out,
			"Active":      activeFromPath(r.URL.Path),
			"Flash":       s.getFlash(r),
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
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
			s.cmdStore.Append(username, "Sync", []string{"sync"})
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
				"Flash":  s.getFlash(r),
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// helpers to prepare footer data
type footerEntry struct{ When, Context, Args string }

func (s *Server) footerData(r *http.Request, username string) (show bool, entries []footerEntry, moreURL string, canShowMore bool, returnURL string) {
	// cookie vs config default
	show = s.uiCfg.ShowCommandLog
	if c, err := r.Cookie("cmdlog"); err == nil {
		if c.Value == "off" {
			show = false
		} else if c.Value == "on" {
			show = true
		}
	}
	returnURL = r.URL.Path
	// count
	n := 5
	q := r.URL.Query()
	if q.Get("all") == "1" {
		n = 0
	} else if q.Get("more") == "1" {
		n = 20
	} else {
		canShowMore = true
		// build moreURL preserving query but setting more=1
		vals := r.URL.Query()
		vals.Set("more", "1")
		vals.Del("all")
		moreURL = r.URL.Path + "?" + vals.Encode()
	}
	if !show {
		return
	}
	raw := s.cmdStore.List(username, n)
	entries = make([]footerEntry, 0, len(raw))
	for _, e := range raw {
		ts := e.When.Format("15:04:05")
		entries = append(entries, footerEntry{When: ts, Context: e.Context, Args: ui.JoinArgs(e.Args)})
	}
	return
}

// flash support
type flash struct{ Type, Text string }

func (s *Server) setFlash(w http.ResponseWriter, typ, text string) {
	if typ == "" {
		typ = "info"
	}
	// simple cookie, short-lived
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: urlQueryEscape(typ + "|" + text), Path: "/", MaxAge: 5})
}

func (s *Server) getFlash(r *http.Request) *flash {
	c, err := r.Cookie("flash")
	if err != nil || c.Value == "" {
		return nil
	}
	val := urlQueryUnescape(c.Value)
	parts := strings.SplitN(val, "|", 2)
	f := &flash{}
	if len(parts) == 2 {
		f.Type = parts[0]
		f.Text = parts[1]
	} else {
		f.Type = "info"
		f.Text = val
	}
	return f
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", "/"), "\n", " ")
}
func urlQueryUnescape(s string) string { return s }

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
