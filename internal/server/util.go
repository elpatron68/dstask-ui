package server

import (
    "fmt"
    "encoding/json"
    "regexp"
    "strings"
    "sort"
    "strconv"
    "time"
)

func getAny(m map[string]any, key string) any {
    if v, ok := m[key]; ok {
        return v
    }
    // manchmal sind Keys anders benannt; versuche einfache Varianten
    switch key {
    case "status":
        if v, ok := m["state"]; ok { return v }
        if v, ok := m["Status"]; ok { return v }
        if v, ok := m["State"]; ok { return v }
    }
    return nil
}

func str(v any) string {
    if v == nil { return "" }
    switch t := v.(type) {
    case string:
        return t
    case fmt.Stringer:
        return t.String()
    default:
        return fmt.Sprintf("%v", v)
    }
}

func joinTags(v any) string {
    if v == nil { return "" }
    switch t := v.(type) {
    case []any:
        parts := make([]string, 0, len(t))
        for _, it := range t {
            parts = append(parts, str(it))
        }
        return strings.Join(parts, ", ")
    case []string:
        return strings.Join(t, ", ")
    default:
        return str(v)
    }
}

func firstOf(m map[string]any, keys ...string) any {
    for _, k := range keys {
        if v, ok := m[k]; ok {
            return v
        }
    }
    return nil
}

func isResolved(m map[string]any) bool {
    // Prüfe Status-Feld(er)
    s := strings.ToLower(str(firstOf(m, "status", "state", "Status", "State")))
    if s == "resolved" || s == "done" {
        return true
    }
    // manche Exporte haben boolesches Feld
    if v := firstOf(m, "resolved", "isResolved"); v != nil {
        if strings.ToLower(str(v)) == "true" {
            return true
        }
    }
    return false
}

var openLineRe = regexp.MustCompile(`^\s*(\d+)\s+(P[0-3])\s+(\S+)\s+(.*\S)\s*$`)

// parseOpenPlain parsed die übliche Tabellenansicht von `dstask show-open`
// mit Header "ID  P  Project  Summary" in strukturierte Zeilen.
func parseOpenPlain(raw string) []map[string]string {
    lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
    rows := make([]map[string]string, 0, len(lines))
    for _, line := range lines {
        l := strings.TrimSpace(line)
        if l == "" { continue }
        // Header-Zeile überspringen
        if strings.HasPrefix(l, "ID") && strings.Contains(l, "Summary") {
            continue
        }
        if m := openLineRe.FindStringSubmatch(l); len(m) == 5 {
            rows = append(rows, map[string]string{
                "id": m[1],
                "priority": m[2],
                "project": trimQuotes(m[3]),
                "summary": trimQuotes(m[4]),
                "due": "",
                "tags": "",
            })
            continue
        }
    }
    return rows
}

// buildRowsFromTasks filtert Tasks optional nach Status und baut Tabellenzeilen.
// statusFilter: "" (kein Filter), oder z. B. "active", "pending", "resolved".
func buildRowsFromTasks(tasks []map[string]any, statusFilter string) []map[string]string {
    rows := make([]map[string]string, 0, len(tasks))
    for _, t := range tasks {
        st := strings.ToLower(str(firstOf(t, "status", "state")))
        if statusFilter != "" && st != statusFilter {
            // Sonderfall: resolved
            if statusFilter == "resolved" {
                if !isResolved(t) { continue }
            } else {
                // wenn explizit active/pending/paused gefiltert wird, resolved ausschließen
                if isResolved(t) { continue }
            }
        } else {
            // Kein expliziter Filter: Resolved ausschließen
            if statusFilter == "" && isResolved(t) { continue }
        }
        id := str(firstOf(t, "id", "ID", "uuid"))
        if id == "" { continue }
        created := trimQuotes(str(firstOf(t, "created", "Created")))
        resolved := trimQuotes(str(firstOf(t, "resolved", "Resolved")))
        rows = append(rows, map[string]string{
            "id":       id,
            "status":   st,
            "summary":  trimQuotes(str(firstOf(t, "summary", "description"))),
            "project":  trimQuotes(str(firstOf(t, "project"))),
            "priority": str(firstOf(t, "priority")),
            "due":      trimQuotes(str(firstOf(t, "due", "dueDate"))),
            "tags":     joinTags(firstOf(t, "tags")),
            "created":  created,
            "resolved": resolved,
            "age":      ageInDays(created),
        })
    }
    return rows
}

// applyQueryFilter filtert Zeilen anhand eines Suchausdrucks q.
// Unterstützt: +tag, project:foo, normaler Text (Substring in summary)
func applyQueryFilter(rows []map[string]string, q string) []map[string]string {
    q = strings.TrimSpace(q)
    if q == "" { return rows }
    tokens := strings.Fields(q)
    out := make([]map[string]string, 0, len(rows))
    for _, r := range rows {
        if rowMatches(r, tokens) {
            out = append(out, r)
        }
    }
    return out
}

func rowMatches(r map[string]string, tokens []string) bool {
    for _, t := range tokens {
        if t == "" { continue }
        if strings.HasPrefix(t, "+") {
            tag := strings.TrimPrefix(t, "+")
            if !strings.Contains(strings.ToLower(r["tags"]), strings.ToLower(tag)) { return false }
            continue
        }
        if strings.HasPrefix(strings.ToLower(t), "project:") {
            p := t[len("project:"):]
            if !strings.EqualFold(r["project"], p) { return false }
            continue
        }
        // Substring in summary
        if !strings.Contains(strings.ToLower(r["summary"]), strings.ToLower(t)) { return false }
    }
    return true
}

// activeFromPath leitet einen einfachen Aktionsnamen für die Navbar ab
func activeFromPath(path string) string {
    path = strings.ToLower(path)
    switch {
    case strings.HasPrefix(path, "/next"):
        return "next"
    case strings.HasPrefix(path, "/open"):
        return "open"
    case strings.HasPrefix(path, "/active"):
        return "active"
    case strings.HasPrefix(path, "/paused"):
        return "paused"
    case strings.HasPrefix(path, "/resolved"):
        return "resolved"
    case strings.HasPrefix(path, "/context"):
        return "context"
    case strings.HasPrefix(path, "/tasks/new"):
        return "new"
    case strings.HasPrefix(path, "/tasks/action"):
        return "action"
    case strings.HasPrefix(path, "/version"):
        return "version"
    case strings.HasPrefix(path, "/sync"):
        return "sync"
    default:
        return "home"
    }
}

// quoteIfNeeded setzt doppelte Anführungszeichen um Werte mit Leerzeichen.
func quoteIfNeeded(s string) string {
    if strings.ContainsAny(s, " \t") {
        // einfache Escapes von doppelten Anführungszeichen
        s = strings.ReplaceAll(s, "\"", "\\\"")
        return "\"" + s + "\""
    }
    return s
}

// normalizeTag wandelt Leerzeichen im Tag in '-' um, da dstask Tags i.d.R. tokens sind
func normalizeTag(tag string) string {
    tag = strings.TrimSpace(tag)
    tag = strings.ReplaceAll(tag, " ", "-")
    return tag
}

// summaryTokens zerlegt die Summary in Tokens und entfernt führende/nützliche '/'-Notentrenner,
// damit nachfolgende Operatoren (+tag, project:) nicht als Note gewertet werden.
func summaryTokens(summary string) []string {
    s := strings.TrimSpace(summary)
    // Entferne einzelne '/' Tokens am Anfang und mitten im Text
    s = strings.ReplaceAll(s, "/", " ")
    fields := strings.Fields(s)
    return fields
}

// trimQuotes entfernt führende und abschließende doppelte Anführungszeichen
func trimQuotes(s string) string {
    s = strings.TrimSpace(s)
    if len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
        return s[1:len(s)-1]
    }
    return s
}

// sort helpers
func priorityRank(p string) int {
    switch strings.ToUpper(p) {
    case "P0": return 0
    case "P1": return 1
    case "P2": return 2
    case "P3": return 3
    default: return 99
    }
}

func SortRowsMaps(rows []map[string]string, key string, dir string) {
    if key == "" { return }
    desc := strings.ToLower(dir) == "desc"
    cmp := func(a, b map[string]string) bool {
        va := a[key]
        vb := b[key]
        switch key {
        case "id", "taskCount", "resolvedCount":
            ia, _ := strconv.Atoi(va)
            ib, _ := strconv.Atoi(vb)
            if desc { return ia > ib }
            return ia < ib
        case "priority":
            ra := priorityRank(va)
            rb := priorityRank(vb)
            if desc { return ra > rb }
            return ra < rb
        case "created", "resolved":
            ta := parseTimeOrZero(va)
            tb := parseTimeOrZero(vb)
            if desc { return ta.After(tb) }
            return ta.Before(tb)
        case "age":
            // age in days (string) -> int compare
            ia, _ := strconv.Atoi(va)
            ib, _ := strconv.Atoi(vb)
            if desc { return ia > ib }
            return ia < ib
        default:
            if desc { return strings.ToLower(va) > strings.ToLower(vb) }
            return strings.ToLower(va) < strings.ToLower(vb)
        }
    }
    sort.SliceStable(rows, func(i, j int) bool { return cmp(rows[i], rows[j]) })
}

// parseTimeOrZero tries RFC3339 and returns zero time on failure
func parseTimeOrZero(s string) time.Time {
    s = strings.TrimSpace(s)
    if s == "" || s == "0001-01-01T00:00:00Z" { return time.Time{} }
    // common layouts
    layouts := []string{
        time.RFC3339Nano,
        time.RFC3339,
        "2006-01-02 15:04:05 -0700 MST",
        "2006-01-02",
    }
    for _, layout := range layouts {
        if t, err := time.Parse(layout, s); err == nil { return t }
    }
    return time.Time{}
}

// ageInDays computes full days between given time and now; zero if invalid
func ageInDays(created string) string {
    t := parseTimeOrZero(created)
    if t.IsZero() { return "" }
    days := int(time.Since(t).Hours() / 24)
    return strconv.Itoa(days)
}


// decodeTasksJSON versucht, eine JSON-Array-Ausgabe (wie von `dstask export`) zu parsen.
func decodeTasksJSON(raw string) ([]map[string]any, bool) {
    // Versuche direkt
    dec := json.NewDecoder(strings.NewReader(raw))
    dec.UseNumber()
    var arr []any
    if err := dec.Decode(&arr); err != nil {
        // Fallback: schneide auf erstes '[' bis letztes ']' zu
        s := strings.TrimSpace(raw)
        start := strings.Index(s, "[")
        end := strings.LastIndex(s, "]")
        if start >= 0 && end > start {
            s = s[start : end+1]
            // JSON-Sanitisierung: trailing comma vor ']' entfernen
            s = regexp.MustCompile(`,\s*\]`).ReplaceAllString(s, "]")
            dec2 := json.NewDecoder(strings.NewReader(s))
            dec2.UseNumber()
            arr = nil
            if err2 := dec2.Decode(&arr); err2 != nil {
                return nil, false
            }
        } else {
            return nil, false
        }
    }
    tasks := make([]map[string]any, 0, len(arr))
    for _, it := range arr {
        if m, ok := it.(map[string]any); ok {
            tasks = append(tasks, m)
        }
    }
    return tasks, true
}

// decodeTasksJSONFlexible unterstützt sowohl Arrays als auch einzelne Objekte.
func decodeTasksJSONFlexible(raw string) ([]map[string]any, bool) {
    // 1) Versuche direkt als Array
    if arr, ok := decodeTasksJSON(raw); ok {
        return arr, true
    }
    // 2) Versuche einzelnes Objekt
    var obj map[string]any
    dec := json.NewDecoder(strings.NewReader(raw))
    dec.UseNumber()
    if err := dec.Decode(&obj); err == nil && len(obj) > 0 {
        return []map[string]any{obj}, true
    }
    // 3) Heuristik: zwischen erstem '{' und letztem '}' ausschneiden und erneut versuchen
    s := strings.TrimSpace(raw)
    start := strings.Index(s, "{")
    end := strings.LastIndex(s, "}")
    if start >= 0 && end > start {
        s = s[start : end+1]
        dec2 := json.NewDecoder(strings.NewReader(s))
        dec2.UseNumber()
        obj = nil
        if err := dec2.Decode(&obj); err == nil && len(obj) > 0 {
            return []map[string]any{obj}, true
        }
    }
    return nil, false
}

// parseTasksLooseFromJSONText extrahiert einfache Felder per Regex, falls JSON-Decoding scheitert.
func parseTasksLooseFromJSONText(raw string) []map[string]string {
    s := strings.ReplaceAll(raw, "\r\n", "\n")
    // Schneide auf erstes '[' bis letztes ']'
    start := strings.Index(s, "[")
    end := strings.LastIndex(s, "]")
    if start >= 0 && end > start {
        s = s[start:end+1]
    }
    // Objekte grob trennen
    parts := strings.Split(s, "\n  {")
    rows := make([]map[string]string, 0, len(parts))
    reID := regexp.MustCompile(`"id"\s*:\s*([0-9]+)`)     
    reSummary := regexp.MustCompile(`"summary"\s*:\s*"([^"]*)"`)
    reProject := regexp.MustCompile(`"project"\s*:\s*"([^"]*)"`)
    rePriority := regexp.MustCompile(`"priority"\s*:\s*"([^"]*)"`)
    reDue := regexp.MustCompile(`"due"\s*:\s*"([^"]*)"`)
    for _, p := range parts {
        id := firstGroup(reID.FindStringSubmatch(p))
        if id == "" { continue }
        summary := trimQuotes(firstGroup(reSummary.FindStringSubmatch(p)))
        project := trimQuotes(firstGroup(reProject.FindStringSubmatch(p)))
        priority := firstGroup(rePriority.FindStringSubmatch(p))
        due := trimQuotes(firstGroup(reDue.FindStringSubmatch(p)))
        rows = append(rows, map[string]string{
            "id": id,
            "summary": summary,
            "project": project,
            "priority": priority,
            "due": due,
            "tags": "",
        })
    }
    return rows
}

func firstGroup(m []string) string {
    if len(m) >= 2 { return m[1] }
    return ""
}

// parseProjectsFromOutput extrahiert Projekt-Namen aus JSON oder Plaintext
func parseProjectsFromOutput(raw string) []string {
    names := make([]string, 0, 16)
    if arr, ok := decodeTasksJSONFlexible(raw); ok && len(arr) > 0 {
        for _, m := range arr {
            n := trimQuotes(str(firstOf(m, "name", "project")))
            if n != "" {
                names = append(names, n)
            }
        }
        return names
    }
    // Plaintext: eine pro Zeile, ggf. Header oder JSON-artig ignorieren
    for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
        l := strings.TrimSpace(line)
        if l == "" { continue }
        if strings.HasPrefix(l, "[") || strings.HasPrefix(l, "{") || strings.HasPrefix(l, "}") || strings.HasPrefix(l, "]") {
            continue
        }
        // einfache Heuristik: wenn Zeile 'name: xxx' enthält, extrahieren, sonst gesamte Zeile
        if i := strings.Index(strings.ToLower(l), "name:"); i >= 0 {
            n := strings.TrimSpace(l[i+len("name:"):])
            n = trimQuotes(n)
            if n != "" { names = append(names, n) }
            continue
        }
        l = trimQuotes(l)
        names = append(names, l)
    }
    return names
}

// parseTagsFromOutput extrahiert Tags aus Plaintext-Liste
func parseTagsFromOutput(raw string) []string {
    tags := make([]string, 0, 32)
    for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
        l := strings.TrimSpace(line)
        if l == "" { continue }
        if strings.HasPrefix(l, "[") || strings.HasPrefix(l, "{") { continue }
        // entferne führende '+' oder Listenmarker
        l = strings.TrimLeft(l, "+-* ")
        if l != "" { tags = append(tags, l) }
    }
    return tags
}


