package server

import (
	"html/template"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/elpatron68/dstask-ui/internal/auth"
)

var idLineRe = regexp.MustCompile(`^\s*(\d+)\b`)

// renderListHTML rendert eine HTML-Tabelle: Spalten ID, Text, Aktionen.
func (s *Server) renderListHTML(w http.ResponseWriter, r *http.Request, title string, raw string) {
	t := template.Must(s.layoutTpl.Clone())
	_, _ = t.New("content").Parse(`
<h2>{{.Title}}</h2>
{{if .Ok}}<div style="background:#d4edda;border:1px solid #c3e6cb;color:#155724;padding:8px;margin-bottom:8px;">Action successful</div>{{end}}
<form method="get" style="margin-bottom:8px">
  <input type="hidden" name="html" value="1"/>
  <input name="q" value="{{.Q}}" placeholder="Filter: +tag project:foo text" style="width:60%" />
  <button type="submit">Filter</button>
</form>
<table border="1" cellpadding="4" cellspacing="0">
  <thead><tr>
    <th style="width:28px;"></th>
    <th style="width:64px;"><a href="{{.Sort.ID}}">ID</a></th>
    <th style="width:90px;"><a href="{{.Sort.Status}}">Status</a></th>
    <th><a href="{{.Sort.Text}}">Text</a></th>
    <th style="width:220px;">Actions</th>
  </tr></thead>
  <tbody>
  {{range .Rows}}
    {{if .IsTask}}
      <tr>
        <td></td>
        <td>{{.ID}}</td>
        <td>{{.Status}}</td>
        <td><pre style="margin:0;white-space:pre-wrap;">{{.Text}}</pre></td>
        <td>
          <form method="post" action="/tasks/{{.ID}}/start" style="display:inline"><button type="submit">start</button></form>
           · <form method="post" action="/tasks/{{.ID}}/done" style="display:inline"><button type="submit">done</button></form>
           · <form method="post" action="/tasks/{{.ID}}/stop" style="display:inline"><button type="submit">stop</button></form>
           · <form method="post" action="/tasks/{{.ID}}/remove" style="display:inline"><button type="submit">remove</button></form>
        </td>
      </tr>
    {{else}}
      <tr><td colspan="5"><pre style="margin:0;white-space:pre-wrap;">{{.Text}}</pre></td></tr>
    {{end}}
  {{end}}
  </tbody>
</table>
`)

	type row struct {
		IsTask bool
		ID     string
		Status string
		Text   string
	}
	rows := make([]row, 0, 64)
	taskCount := 0
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		l := strings.TrimRight(line, "\n")
		if l == "" {
			continue
		}
		if idxs := idLineRe.FindStringSubmatchIndex(l); len(idxs) == 4 {
			id := l[idxs[2]:idxs[3]]
			after := l[idxs[3]:]
			text := strings.TrimSpace(after)
			rows = append(rows, row{IsTask: true, ID: id, Status: "", Text: text})
			taskCount++
		} else {
			rows = append(rows, row{IsTask: false, Text: l})
		}
	}

	// Falls keine Task-Zeilen erkannt wurden, als Plaintext ausgeben
	if taskCount == 0 {
		_, _ = t.New("content").Parse(`
<h2>{{.Title}}</h2>
<pre style="white-space: pre-wrap;">{{.Body}}</pre>
`)
		_ = t.Execute(w, map[string]any{
			"Title": title,
			"Body":  raw,
		})
		return
	}

	// sort rows (id/status/text)
	sortKey := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	sort.SliceStable(rows, func(i, j int) bool {
		less := false
		switch sortKey {
		case "id":
			less = rows[i].ID < rows[j].ID
		case "status":
			less = strings.ToLower(rows[i].Status) < strings.ToLower(rows[j].Status)
		case "text":
			less = strings.ToLower(rows[i].Text) < strings.ToLower(rows[j].Text)
		default:
			return rows[i].ID < rows[j].ID
		}
		if strings.ToLower(sortDir) == "desc" {
			return !less
		}
		return less
	})
	mk := func(col string) string {
		q := r.URL.Query()
		dir := q.Get("dir")
		if q.Get("sort") == col {
			if strings.ToLower(dir) == "asc" {
				dir = "desc"
			} else {
				dir = "asc"
			}
		} else {
			dir = "asc"
		}
		q.Set("sort", col)
		q.Set("dir", dir)
		q.Set("html", "1")
		return r.URL.Path + "?" + q.Encode()
	}
	uname, _ := auth.UsernameFromRequest(r)
	show, entries, moreURL, canMore, ret := s.footerData(r, uname)
	_ = t.Execute(w, map[string]any{
		"Title":       title,
		"Rows":        rows,
		"Q":           r.URL.Query().Get("q"),
		"Ok":          r.URL.Query().Get("ok") != "",
		"Active":      activeFromPath(r.URL.Path),
		"Flash":       s.getFlash(r),
		"ShowCmdLog":  show,
		"CmdEntries":  entries,
		"MoreURL":     moreURL,
		"CanShowMore": canMore,
		"ReturnURL":   ret,
		"Sort":        map[string]string{"ID": mk("id"), "Status": mk("status"), "Text": mk("text")},
	})
}

// renderExportTable rendert Tasks aus `dstask export` als Tabelle.
// `rows` erwartet bereits gefilterte/aufbereitete Zeilen.
func (s *Server) renderExportTable(w http.ResponseWriter, r *http.Request, title string, rows []map[string]string) {
	t := template.Must(s.layoutTpl.Clone())
	// enrich rows with action flags and computed flags
	rowsAny := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		status := strings.ToLower(m["status"])
		canStart := status == "pending" || status == "paused"
		canStop := status == "active"
		canDone := status != "resolved" && status != "done"
		mm := map[string]any{}
		for k, v := range m {
			mm[k] = v
		}
		if cs, ok := m["created"]; ok {
			mm["created"] = formatDateShort(cs)
		}
		if rs, ok := m["resolved"]; ok {
			mm["resolved"] = formatDateShort(rs)
		}
		if ds, ok := m["due"]; ok {
			mm["due"] = formatDateShort(ds)
		}
		mm["canStart"] = canStart
		mm["canStop"] = canStop
		mm["canDone"] = canDone
		// Check for URLs in summary
		summary := m["summary"]
		mm["hasURLs"] = len(extractURLs(summary)) > 0
		// Check for notes
		notes := m["notes"]
		mm["hasNotes"] = notes != "" && strings.TrimSpace(notes) != ""
		if due, ok := m["due"]; ok {
			mm["overdue"] = isOverdue(due)
		}
		rowsAny = append(rowsAny, mm)
	}
	// sorting
	sortKey := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	SortRowsMaps(rows, sortKey, sortDir)
	mk := func(col string) string {
		q := r.URL.Query()
		dir := q.Get("dir")
		if q.Get("sort") == col {
			if strings.ToLower(dir) == "asc" {
				dir = "desc"
			} else {
				dir = "asc"
			}
		} else {
			dir = "asc"
		}
		q.Set("sort", col)
		q.Set("dir", dir)
		q.Set("html", "1")
		return r.URL.Path + "?" + q.Encode()
	}
	_, _ = t.New("content").Parse(`
<h2>{{.Title}}</h2>
<form method="get" style="margin-bottom:8px">
  <input type="hidden" name="html" value="1"/>
  <input name="q" value="{{.Q}}" placeholder="Filter: +tag project:foo text" style="width:50%" />
  <label style="margin-left:8px;">Due filter:
    <select name="dueFilterType" style="margin-left:4px;">
      <option value="">(none)</option>
      <option value="before" {{if eq .DueFilterType "before"}}selected{{end}}>before</option>
      <option value="after" {{if eq .DueFilterType "after"}}selected{{end}}>after</option>
      <option value="on" {{if eq .DueFilterType "on"}}selected{{end}}>on</option>
      <option value="overdue" {{if eq .DueFilterType "overdue"}}selected{{end}}>overdue</option>
    </select>
    <input name="dueFilterDate" value="{{.DueFilterDate}}" placeholder="friday / 2025-12-31" style="width:180px; margin-left:4px;" {{if eq .DueFilterType "overdue"}}disabled{{end}} />
  </label>
  <button type="submit" style="margin-left:8px;">Filtern</button>
</form>
<table border="1" cellpadding="4" cellspacing="0">
  <thead><tr>
    <th style="width:28px;"></th>
    <th style="width:64px;"><a href="{{.Sort.ID}}">ID</a></th>
    <th style="width:90px;"><a href="{{.Sort.Status}}">Status</a></th>
    <th><a href="{{.Sort.Summary}}">Summary</a></th>
    <th><a href="{{.Sort.Project}}">Project</a></th>
    <th style="width:80px;"><a href="{{.Sort.Priority}}">Priority</a></th>
    <th style="width:140px;"><a href="{{.Sort.Due}}">Due</a></th>
    <th style="width:220px;"><a href="{{.Sort.Tags}}">Tags</a></th>
    <th style="width:160px;"><a href="{{.Sort.Created}}">Created</a></th>
    <th style="width:160px;"><a href="{{.Sort.Resolved}}">Resolved</a></th>
    <th style="width:90px;"><a href="{{.Sort.Age}}">Age</a></th>
    <th style="width:220px;">Aktionen</th>
  </tr></thead>
  <tbody>
  {{range .Rows}}
    <tr>
      <td><input type="checkbox" name="ids" value="{{index . "id"}}" form="batchForm"/></td>
      <td>{{index . "id"}}{{if .hasNotes}} 
        <span class="hovercard"><span class="label" title="Show notes">📝</span>
          <div class="card"><div class="notes-content">{{renderMarkdown (index . "notes")}}</div></div>
        </span>
      {{end}}</td>
      <td><span class="badge status {{index . "status"}}" title="{{index . "status"}}">{{index . "status"}}</span></td>
      <td style="white-space:pre-wrap;">{{linkifyURLs (index . "summary")}}</td>
      <td>{{index . "project"}}</td>
      <td><span class="badge prio {{index . "priority"}}" title="{{index . "priority"}}">{{index . "priority"}}</span></td>
      <td class="due {{if .overdue}}overdue{{end}}">{{index . "due"}}</td>
      <td>
        {{- $tags := index . "tags" -}}
        {{- if $tags -}}
          {{- range (split $tags ", ") -}}<span class="pill" title="tag">{{.}}</span>{{- end -}}
        {{- end -}}
      </td>
      <td><code>{{index . "created"}}</code></td>
      <td><code>{{index . "resolved"}}</code></td>
      <td>{{index . "age"}}</td>
      <td>
        <form method="get" action="/tasks/{{index . "id"}}/edit" style="display:inline"><button type="submit" title="Edit task details">edit</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/start" style="display:inline"><button type="submit" {{if not .canStart}}disabled{{end}} title="Mark task as active">start</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/done" style="display:inline"><button type="submit" {{if not .canDone}}disabled{{end}} title="Mark task as completed/resolved">done</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/stop" style="display:inline"><button type="submit" {{if not .canStop}}disabled{{end}} title="Pause/stop the task">stop</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/remove" style="display:inline" onsubmit="return confirm('Are you sure you want to delete this task?');"><input type="hidden" name="csrf_token" value="{{$.CSRFToken}}"/><button type="submit" title="Delete the task">remove</button></form>
         {{if .hasURLs}} · <a href="/tasks/{{index . "id"}}/open" title="View and open URLs from this task">open</a>{{end}}
      </td>
    </tr>
    
  {{end}}
  </tbody>
</table>
<form id="batchForm" method="post" action="/tasks/batch" style="margin-top:8px;" onsubmit="var action = this.action.value; if ((action === 'remove' || action === 'done') && !confirm('Are you sure you want to ' + action + ' the selected tasks?')) { return false; }">
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}"/>
  <label>Batch action:
    <select name="action" id="batchActionSelect">
      <option value="start">start</option>
      <option value="stop">stop</option>
      <option value="done">done</option>
      <option value="remove">remove</option>
      <option value="note">note</option>
    </select>
  </label>
  <label style="margin-left:8px;">Note: <input name="note" placeholder="for action 'note'"/></label>
  <button type="submit" style="margin-left:8px;">Apply</button>
</form>
{{if .Pagination}}
<div style="margin-top:12px;padding:8px;border-top:1px solid #d0d7de;">
  <span>Showing {{.Pagination.CurrentPage}} of {{.Pagination.TotalPages}} pages ({{.Pagination.TotalRows}} total tasks)</span>
  {{if .Pagination.HasPrev}}
    <a href="{{.Pagination.FirstURL}}" style="margin-left:8px;">« First</a>
    <a href="{{.Pagination.PrevURL}}" style="margin-left:4px;">‹ Prev</a>
  {{end}}
  {{if .Pagination.HasNext}}
    <a href="{{.Pagination.NextURL}}" style="margin-left:8px;">Next ›</a>
    <a href="{{.Pagination.LastURL}}" style="margin-left:4px;">Last »</a>
  {{end}}
</div>
{{end}}
`)
	uname, _ := auth.UsernameFromRequest(r)
	show, entries, moreURL, canMore, ret := s.footerData(r, uname)
	q := r.URL.Query()
	// Pagination
	page := 1
	perPage := 50
	if p := q.Get("page"); p != "" {
		if pp, err := strconv.Atoi(p); err == nil && pp > 0 {
			page = pp
		}
	}
	if pp := q.Get("per_page"); pp != "" {
		if ppp, err := strconv.Atoi(pp); err == nil && ppp > 0 && ppp <= 500 {
			perPage = ppp
		}
	}
	totalRows := len(rowsAny)
	totalPages := (totalRows + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	end := start + perPage
	if end > totalRows {
		end = totalRows
	}
	paginatedRows := rowsAny[start:end]

	// Build pagination links
	buildPageURL := func(p int) string {
		qc := r.URL.Query()
		qc.Set("page", strconv.Itoa(p))
		return r.URL.Path + "?" + qc.Encode()
	}
	pagination := map[string]any{
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"TotalRows":   totalRows,
		"PerPage":     perPage,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"PrevURL":     "",
		"NextURL":     "",
		"FirstURL":    "",
		"LastURL":     "",
	}
	if page > 1 {
		pagination["PrevURL"] = buildPageURL(page - 1)
		pagination["FirstURL"] = buildPageURL(1)
	}
	if page < totalPages {
		pagination["NextURL"] = buildPageURL(page + 1)
		pagination["LastURL"] = buildPageURL(totalPages)
	}

	dueFilterType := q.Get("dueFilterType")
	dueFilterDate := q.Get("dueFilterDate")
	csrfToken := s.ensureCSRFToken(w, r)
	_ = t.Execute(w, map[string]any{"Title": title, "Rows": paginatedRows, "Q": q.Get("q"), "Active": activeFromPath(r.URL.Path),
		"Flash":      s.getFlash(r),
		"ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
		"DueFilterType": dueFilterType, "DueFilterDate": dueFilterDate, "CSRFToken": csrfToken,
		"Pagination": pagination,
		"Sort": map[string]string{
			"ID": mk("id"), "Status": mk("status"), "Summary": mk("summary"), "Project": mk("project"), "Priority": mk("priority"), "Due": mk("due"), "Tags": mk("tags"),
			"Created": mk("created"), "Resolved": mk("resolved"), "Age": mk("age"),
		},
	})
}

// renderProjectsTable rendert eine Tabelle für Projekte
func (s *Server) renderProjectsTable(w http.ResponseWriter, r *http.Request, title string, rows []map[string]string) {
	t := template.Must(s.layoutTpl.Clone())
	SortRowsMaps(rows, r.URL.Query().Get("sort"), r.URL.Query().Get("dir"))
	mk := func(col string) string {
		q := r.URL.Query()
		dir := q.Get("dir")
		if q.Get("sort") == col {
			if strings.ToLower(dir) == "asc" {
				dir = "desc"
			} else {
				dir = "asc"
			}
		} else {
			dir = "asc"
		}
		q.Set("sort", col)
		q.Set("dir", dir)
		return r.URL.Path + "?" + q.Encode()
	}
	_, _ = t.New("content").Parse(`
<h2>{{.Title}}</h2>
<table border="1" cellpadding="4" cellspacing="0">
  <thead><tr>
    <th><a href="{{.Sort.Name}}">Project</a></th>
    <th style="width:100px;"><a href="{{.Sort.Open}}">Open</a></th>
    <th style="width:120px;"><a href="{{.Sort.Resolved}}">Resolved</a></th>
    <th style="width:90px;"><a href="{{.Sort.Active}}">Active</a></th>
    <th style="width:80px;"><a href="{{.Sort.Priority}}">Priority</a></th>
  </tr></thead>
  <tbody>
  {{range .Rows}}
    <tr>
      <td>{{index . "name"}}</td>
      <td>{{index . "taskCount"}}</td>
      <td>{{index . "resolvedCount"}}</td>
      <td>{{index . "active"}}</td>
      <td>{{index . "priority"}}</td>
    </tr>
  {{end}}
  </tbody>
</table>
`)
	uname, _ := auth.UsernameFromRequest(r)
	show, entries, moreURL, canMore, ret := s.footerData(r, uname)
	_ = t.Execute(w, map[string]any{"Title": title, "Rows": rows, "Active": activeFromPath(r.URL.Path),
		"ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
		"Sort": map[string]string{"Name": mk("name"), "Open": mk("taskCount"), "Resolved": mk("resolvedCount"), "Active": mk("active"), "Priority": mk("priority")},
	})
}

// (entfernt) escapeExceptBasic war ungenutzt
