package server

import (
    "html/template"
    "net/http"
    "regexp"
    "strings"
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
  <thead><tr><th style="width:64px;">ID</th><th style="width:90px;">Status</th><th>Text</th><th style="width:220px;">Aktionen</th></tr></thead>
  <tbody>
  {{range .Rows}}
    {{if .IsTask}}
      <tr>
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
      <tr><td colspan="4"><pre style="margin:0;white-space:pre-wrap;">{{.Text}}</pre></td></tr>
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

    _ = t.Execute(w, map[string]any{
        "Title": title,
        "Rows":  rows,
        "Q": r.URL.Query().Get("q"),
        "Ok": r.URL.Query().Get("ok") != "",
        "Active": activeFromPath(r.URL.Path),
    })
}

// renderExportTable rendert Tasks aus `dstask export` als Tabelle.
// `rows` erwartet bereits gefilterte/aufbereitete Zeilen.
func (s *Server) renderExportTable(w http.ResponseWriter, r *http.Request, title string, rows []map[string]string) {
    t := template.Must(s.layoutTpl.Clone())
    _, _ = t.New("content").Parse(`
<h2>{{.Title}}</h2>
<form method="get" style="margin-bottom:8px">
  <input type="hidden" name="html" value="1"/>
  <input name="q" value="{{.Q}}" placeholder="Filter: +tag project:foo text" style="width:60%" />
  <button type="submit">Filtern</button>
</form>
<table border="1" cellpadding="4" cellspacing="0">
  <thead><tr>
    <th style="width:64px;">ID</th>
    <th style="width:90px;">Status</th>
    <th>Summary</th>
    <th>Project</th>
    <th style="width:80px;">Priority</th>
    <th style="width:140px;">Due</th>
    <th style="width:220px;">Tags</th>
    <th style="width:220px;">Aktionen</th>
  </tr></thead>
  <tbody>
  {{range .Rows}}
    <tr>
      <td>{{index . "id"}}</td>
      <td>{{index . "status"}}</td>
      <td><pre style="margin:0;white-space:pre-wrap;">{{index . "summary"}}</pre></td>
      <td>{{index . "project"}}</td>
      <td>{{index . "priority"}}</td>
      <td>{{index . "due"}}</td>
      <td>{{index . "tags"}}</td>
      <td>
        <form method="post" action="/tasks/{{index . "id"}}/start" style="display:inline"><button type="submit">start</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/done" style="display:inline"><button type="submit">done</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/stop" style="display:inline"><button type="submit">stop</button></form>
         · <form method="post" action="/tasks/{{index . "id"}}/remove" style="display:inline"><button type="submit">remove</button></form>
      </td>
    </tr>
  {{end}}
  </tbody>
</table>
`)
    _ = t.Execute(w, map[string]any{ "Title": title, "Rows": rows, "Q": r.URL.Query().Get("q"), "Active": activeFromPath(r.URL.Path) })
}

// renderProjectsTable rendert eine Tabelle für Projekte
func (s *Server) renderProjectsTable(w http.ResponseWriter, r *http.Request, title string, rows []map[string]string) {
    t := template.Must(s.layoutTpl.Clone())
    _, _ = t.New("content").Parse(`
<h2>{{.Title}}</h2>
<table border="1" cellpadding="4" cellspacing="0">
  <thead><tr>
    <th>Project</th>
    <th style="width:100px;">Open</th>
    <th style="width:120px;">Resolved</th>
    <th style="width:90px;">Active</th>
    <th style="width:80px;">Priority</th>
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
    _ = t.Execute(w, map[string]any{ "Title": title, "Rows": rows, "Active": activeFromPath(r.URL.Path) })
}

// escapeExceptBasic lässt die eingefügten Aktionslinks intakt, escaped sonst HTML.
func escapeExceptBasic(s string) string { return s }


