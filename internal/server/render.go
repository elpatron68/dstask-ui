package server

import (
    "html/template"
    "net/http"
    "regexp"
    "strings"
    "github.com/elpatron68/dstask-ui/internal/auth"
    "sort"
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
        if l == "" { continue }
        if idxs := idLineRe.FindStringSubmatchIndex(l); len(idxs) == 4 {
            id := l[idxs[2]:idxs[3]]
            after := l[idxs[3]:]
            text := strings.TrimSpace(after)
            rows = append(rows, row{IsTask:true, ID:id, Status:"", Text:text})
            taskCount++
        } else {
            rows = append(rows, row{IsTask:false, Text:l})
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
        if strings.ToLower(sortDir) == "desc" { return !less }
        return less
    })
    mk := func(col string) string {
        q := r.URL.Query(); dir := q.Get("dir")
        if q.Get("sort") == col { if strings.ToLower(dir)=="asc"{dir="desc"} else {dir="asc"} } else { dir="asc" }
        q.Set("sort", col); q.Set("dir", dir); q.Set("html","1")
        return r.URL.Path + "?" + q.Encode()
    }
    uname, _ := auth.UsernameFromRequest(r)
    show, entries, moreURL, canMore, ret := s.footerData(r, uname)
    _ = t.Execute(w, map[string]any{
        "Title": title,
        "Rows":  rows,
        "Q": r.URL.Query().Get("q"),
        "Ok": r.URL.Query().Get("ok") != "",
        "Active": activeFromPath(r.URL.Path),
        "Flash": s.getFlash(r),
        "ShowCmdLog": show,
        "CmdEntries": entries,
        "MoreURL": moreURL,
        "CanShowMore": canMore,
        "ReturnURL": ret,
        "Sort": map[string]string{"ID": mk("id"), "Status": mk("status"), "Text": mk("text")},
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
        for k, v := range m { mm[k] = v }
        mm["canStart"] = canStart
        mm["canStop"] = canStop
        mm["canDone"] = canDone
        if due, ok := m["due"]; ok { mm["overdue"] = isOverdue(due) }
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
            if strings.ToLower(dir) == "asc" { dir = "desc" } else { dir = "asc" }
        } else { dir = "asc" }
        q.Set("sort", col); q.Set("dir", dir); q.Set("html", "1")
        return r.URL.Path + "?" + q.Encode()
    }
    _, _ = t.New("content").Parse(`
<h2>{{.Title}}</h2>
<form method="get" style="margin-bottom:8px">
  <input type="hidden" name="html" value="1"/>
  <input name="q" value="{{.Q}}" placeholder="Filter: +tag project:foo text" style="width:60%" />
  <button type="submit">Filtern</button>
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
      <td>{{index . "id"}}</td>
      <td><span class="badge status {{index . "status"}}" title="{{index . "status"}}">{{index . "status"}}</span></td>
      <td><pre style="margin:0;white-space:pre-wrap;">{{index . "summary"}}</pre></td>
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
        <form method="post" action="/tasks/{{index . "id"}}/start" style="display:inline"><button type="submit" {{if not .canStart}}disabled{{end}}>start</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/done" style="display:inline"><button type="submit" {{if not .canDone}}disabled{{end}}>done</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/stop" style="display:inline"><button type="submit" {{if not .canStop}}disabled{{end}}>stop</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/remove" style="display:inline"><button type="submit">remove</button></form>
      </td>
    </tr>
  {{end}}
  </tbody>
</table>
<form id="batchForm" method="post" action="/tasks/batch" style="margin-top:8px;">
  <label>Batch action:
    <select name="action">
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
`)
    uname, _ := auth.UsernameFromRequest(r)
    show, entries, moreURL, canMore, ret := s.footerData(r, uname)
    _ = t.Execute(w, map[string]any{ "Title": title, "Rows": rowsAny, "Q": r.URL.Query().Get("q"), "Active": activeFromPath(r.URL.Path),
        "Flash": s.getFlash(r),
        "ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
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
        q := r.URL.Query(); dir := q.Get("dir")
        if q.Get("sort") == col { if strings.ToLower(dir)=="asc"{dir="desc"} else {dir="asc"} } else { dir="asc" }
        q.Set("sort", col); q.Set("dir", dir)
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
    _ = t.Execute(w, map[string]any{ "Title": title, "Rows": rows, "Active": activeFromPath(r.URL.Path),
        "ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
        "Sort": map[string]string{ "Name": mk("name"), "Open": mk("taskCount"), "Resolved": mk("resolvedCount"), "Active": mk("active"), "Priority": mk("priority") },
    })
}

// escapeExceptBasic lässt die eingefügten Aktionslinks intakt, escaped sonst HTML.
func escapeExceptBasic(s string) string { return s }


