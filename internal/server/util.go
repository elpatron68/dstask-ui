package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
)

func str(v any) string {
	if v == nil {
		return ""
	}
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
	if v == nil {
		return ""
	}
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
		if l == "" {
			continue
		}
		// Header-Zeile überspringen
		if strings.HasPrefix(l, "ID") && strings.Contains(l, "Summary") {
			continue
		}
		if m := openLineRe.FindStringSubmatch(l); len(m) == 5 {
			rows = append(rows, map[string]string{
				"id":       m[1],
				"priority": m[2],
				"project":  trimQuotes(m[3]),
				"summary":  trimQuotes(m[4]),
				"due":      "",
				"tags":     "",
				"notes":    "",
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
				if !isResolved(t) {
					continue
				}
			} else {
				// wenn explizit active/pending/paused gefiltert wird, resolved ausschließen
				if isResolved(t) {
					continue
				}
			}
		} else {
			// Kein expliziter Filter: Resolved ausschließen
			if statusFilter == "" && isResolved(t) {
				continue
			}
		}
		id := str(firstOf(t, "id", "ID", "uuid"))
		if id == "" {
			continue
		}
		created := trimQuotes(str(firstOf(t, "created", "Created")))
		resolved := trimQuotes(str(firstOf(t, "resolved", "Resolved")))
		notes := trimQuotes(str(firstOf(t, "notes", "annotations", "note")))
		rows = append(rows, map[string]string{
			"id":       id,
			"status":   st,
			"summary":  trimQuotes(str(firstOf(t, "summary", "description"))),
			"project":  trimQuotes(str(firstOf(t, "project"))),
			"priority": str(firstOf(t, "priority")),
			"due":      trimQuotes(str(firstOf(t, "due", "dueDate"))),
			"tags":     joinTags(firstOf(t, "tags")),
			"notes":    notes,
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
	if q == "" {
		return rows
	}
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
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "+") {
			tag := strings.TrimPrefix(t, "+")
			if !strings.Contains(strings.ToLower(r["tags"]), strings.ToLower(tag)) {
				return false
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(t), "project:") {
			p := t[len("project:"):]
			if !strings.EqualFold(r["project"], p) {
				return false
			}
			continue
		}
		// Substring in summary
		if !strings.Contains(strings.ToLower(r["summary"]), strings.ToLower(t)) {
			return false
		}
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
	case strings.HasPrefix(path, "/projects"):
		return "projects"
	case strings.HasPrefix(path, "/templates"):
		return "templates"
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
	case strings.HasPrefix(path, "/undo"):
		return "undo"
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

// sanitizeDueValue entfernt Platzhalter-Werte wie 0001-01-01T00:00:00Z, die von dstask für "kein Fälligkeitsdatum" verwendet werden.
func sanitizeDueValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.EqualFold(s, "0001-01-01T00:00:00Z") {
		return ""
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
// URLs werden als vollständige Tokens erhalten.
func summaryTokens(summary string) []string {
	s := strings.TrimSpace(summary)
	if s == "" {
		return nil
	}

	// URL-Regex (gleiches Pattern wie in extractURLs)
	urlRe := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

	// Finde alle URLs und ihre Positionen
	type urlMatch struct {
		url        string
		start, end int
	}
	var urls []urlMatch
	for _, match := range urlRe.FindAllStringIndex(s, -1) {
		urls = append(urls, urlMatch{
			url:   s[match[0]:match[1]],
			start: match[0],
			end:   match[1],
		})
	}

	// Ersetze URLs durch Platzhalter, um sie zu schützen
	urlPlaceholders := make(map[string]string)
	for i, url := range urls {
		placeholder := fmt.Sprintf("__URL_PLACEHOLDER_%d__", i)
		urlPlaceholders[placeholder] = url.url
		// Ersetze URL durch Platzhalter (rückwärts, um Indizes nicht zu verschieben)
	}

	// Baue String mit Platzhaltern auf
	result := strings.Builder{}
	lastEnd := 0
	for i, url := range urls {
		// Text vor URL
		if url.start > lastEnd {
			result.WriteString(s[lastEnd:url.start])
		}
		// Platzhalter für URL
		result.WriteString(fmt.Sprintf("__URL_PLACEHOLDER_%d__", i))
		lastEnd = url.end
	}
	// Rest nach letzter URL
	if lastEnd < len(s) {
		result.WriteString(s[lastEnd:])
	}
	protected := result.String()

	// Ersetze '/' durch Leerzeichen (nur außerhalb von URLs, die wir schon geschützt haben)
	protected = strings.ReplaceAll(protected, "/", " ")

	// Splitte in Tokens
	fields := strings.Fields(protected)

	// Ersetze Platzhalter wieder durch URLs
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if url, ok := urlPlaceholders[field]; ok {
			tokens = append(tokens, url)
		} else {
			tokens = append(tokens, field)
		}
	}

	return tokens
}

// trimQuotes entfernt führende und abschließende doppelte Anführungszeichen
func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return s[1 : len(s)-1]
	}
	return s
}

// truncate truncates a string to max length, appending "..." if truncated
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// helpers for JSON IO
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func jsonNewDecoder(r *http.Request) *json.Decoder {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec
}

// stripANSI removes ANSI escape sequences (e.g., color codes) from a string.
// This is useful when displaying command output that may contain terminal formatting.
func stripANSI(text string) string {
	// Regular expression to match common ANSI escape codes
	// Match ESC [ followed by numbers, semicolons, and ending with a letter (color codes, cursor movements, etc.)
	// Also match ESC ] followed by OSC sequences (like window titles)
	ansi1 := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	ansi2 := regexp.MustCompile(`\x1b\]\d+;.*?\x07`)
	result := ansi1.ReplaceAllString(text, "")
	result = ansi2.ReplaceAllString(result, "")
	// Also handle CSI sequences (0x9b) - convert to bytes for handling
	bytes := []byte(result)
	var filtered []byte
	for i := 0; i < len(bytes); i++ {
		if bytes[i] == 0x9b { // CSI
			// Skip until we find a letter (a-zA-Z) which ends the CSI sequence
			i++
			for i < len(bytes) && ((bytes[i] >= '0' && bytes[i] <= '9') || bytes[i] == ';' || bytes[i] == ':') {
				i++
			}
			if i < len(bytes) && ((bytes[i] >= 'a' && bytes[i] <= 'z') || (bytes[i] >= 'A' && bytes[i] <= 'Z')) {
				i++ // Skip the ending letter
			}
			i-- // Adjust for loop increment
		} else {
			filtered = append(filtered, bytes[i])
		}
	}
	return string(filtered)
}

// generateCSRFToken generates a secure random CSRF token.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// validateCSRFToken compares the provided token with the token from the request cookie.
func validateCSRFToken(r *http.Request, providedToken string) bool {
	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		return false
	}
	return cookie.Value != "" && cookie.Value == providedToken
}

// sort helpers
func priorityRank(p string) int {
	switch strings.ToUpper(p) {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 99
	}
}

func SortRowsMaps(rows []map[string]string, key string, dir string) {
	if key == "" {
		return
	}
	desc := strings.ToLower(dir) == "desc"
	cmp := func(a, b map[string]string) bool {
		va := a[key]
		vb := b[key]
		switch key {
		case "id", "taskCount", "resolvedCount":
			ia, _ := strconv.Atoi(va)
			ib, _ := strconv.Atoi(vb)
			if desc {
				return ia > ib
			}
			return ia < ib
		case "priority":
			ra := priorityRank(va)
			rb := priorityRank(vb)
			if desc {
				return ra > rb
			}
			return ra < rb
		case "created", "resolved":
			ta := parseTimeOrZero(va)
			tb := parseTimeOrZero(vb)
			if desc {
				return ta.After(tb)
			}
			return ta.Before(tb)
		case "age":
			// age in days (string) -> int compare
			ia, _ := strconv.Atoi(va)
			ib, _ := strconv.Atoi(vb)
			if desc {
				return ia > ib
			}
			return ia < ib
		default:
			if desc {
				return strings.ToLower(va) > strings.ToLower(vb)
			}
			return strings.ToLower(va) < strings.ToLower(vb)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return cmp(rows[i], rows[j]) })
}

// parseTimeOrZero tries RFC3339 and returns zero time on failure
func parseTimeOrZero(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" || s == "0001-01-01T00:00:00Z" {
		return time.Time{}
	}
	// common layouts
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ageInDays computes full days between given time and now; zero if invalid
func ageInDays(created string) string {
	t := parseTimeOrZero(created)
	if t.IsZero() {
		return ""
	}
	days := int(time.Since(t).Hours() / 24)
	return strconv.Itoa(days)
}

// isOverdue returns true if due is a valid past time (strictly before now)
func isOverdue(due string) bool {
	t := parseTimeOrZero(due)
	if t.IsZero() {
		return false
	}
	return time.Now().After(t)
}

// formatDateShort returns a compact, readable datetime (YYYY-MM-DD HH:MM) if parseable
func formatDateShort(s string) string {
	t := parseTimeOrZero(s)
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
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
		s = s[start : end+1]
	}
	// Objekte grob trennen
	parts := strings.Split(s, "\n  {")
	rows := make([]map[string]string, 0, len(parts))
	reID := regexp.MustCompile(`"id"\s*:\s*([0-9]+)`)
	reSummary := regexp.MustCompile(`"summary"\s*:\s*"([^"]*)"`)
	reProject := regexp.MustCompile(`"project"\s*:\s*"([^"]*)"`)
	rePriority := regexp.MustCompile(`"priority"\s*:\s*"([^"]*)"`)
	reDue := regexp.MustCompile(`"due"\s*:\s*"([^"]*)"`)
	reNotes := regexp.MustCompile(`"notes"\s*:\s*"([^"]*)"`)
	for _, p := range parts {
		id := firstGroup(reID.FindStringSubmatch(p))
		if id == "" {
			continue
		}
		summary := trimQuotes(firstGroup(reSummary.FindStringSubmatch(p)))
		project := trimQuotes(firstGroup(reProject.FindStringSubmatch(p)))
		priority := firstGroup(rePriority.FindStringSubmatch(p))
		due := trimQuotes(firstGroup(reDue.FindStringSubmatch(p)))
		notes := trimQuotes(firstGroup(reNotes.FindStringSubmatch(p)))
		rows = append(rows, map[string]string{
			"id":       id,
			"summary":  summary,
			"project":  project,
			"priority": priority,
			"due":      due,
			"tags":     "",
			"notes":    notes,
		})
	}
	return rows
}

func firstGroup(m []string) string {
	if len(m) >= 2 {
		return m[1]
	}
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
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "[") || strings.HasPrefix(l, "{") || strings.HasPrefix(l, "}") || strings.HasPrefix(l, "]") {
			continue
		}
		// einfache Heuristik: wenn Zeile 'name: xxx' enthält, extrahieren, sonst gesamte Zeile
		if i := strings.Index(strings.ToLower(l), "name:"); i >= 0 {
			n := strings.TrimSpace(l[i+len("name:"):])
			n = trimQuotes(n)
			if n != "" {
				names = append(names, n)
			}
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
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "[") || strings.HasPrefix(l, "{") {
			continue
		}
		// entferne führende '+' oder Listenmarker
		l = strings.TrimLeft(l, "+-* ")
		if l != "" {
			tags = append(tags, l)
		}
	}
	return tags
}

// parseRelativeDate parses relative date strings like "tomorrow", "friday", "next-monday", etc.
// Returns zero time if parsing fails.
func parseRelativeDate(s string) time.Time {
	s = strings.ToLower(strings.TrimSpace(s))
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch s {
	case "yesterday":
		return today.AddDate(0, 0, -1)
	case "today":
		return today
	case "tomorrow":
		return today.AddDate(0, 0, 1)
	}

	// Weekday parsing (monday, tue, wed, etc.)
	weekdayMap := map[string]time.Weekday{
		"monday": time.Monday, "mon": time.Monday, "mo": time.Monday,
		"tuesday": time.Tuesday, "tue": time.Tuesday, "tu": time.Tuesday,
		"wednesday": time.Wednesday, "wed": time.Wednesday, "we": time.Wednesday,
		"thursday": time.Thursday, "thu": time.Thursday, "th": time.Thursday,
		"friday": time.Friday, "fri": time.Friday, "fr": time.Friday,
		"saturday": time.Saturday, "sat": time.Saturday, "sa": time.Saturday,
		"sunday": time.Sunday, "sun": time.Sunday, "su": time.Sunday,
	}

	// Check for "this-", "next-" prefix
	isNext := strings.HasPrefix(s, "next-")
	isThis := strings.HasPrefix(s, "this-")
	if isNext || isThis {
		s = s[strings.Index(s, "-")+1:]
	}

	if wd, ok := weekdayMap[s]; ok {
		currentWd := today.Weekday()
		daysAhead := int(wd - currentWd)
		if daysAhead < 0 {
			daysAhead += 7
		}
		if isNext {
			daysAhead += 7
		} else if isThis && daysAhead == 0 {
			// "this-friday" when it's already friday means today
			daysAhead = 0
		}
		return today.AddDate(0, 0, daysAhead)
	}

	// Try parsing as absolute date (already handled by parseTimeOrZero)
	return time.Time{}
}

// parseDueDate parses a date string that can be relative (tomorrow, friday) or absolute (2025-12-31)
func parseDueDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	// Try relative date first
	if rel := parseRelativeDate(s); !rel.IsZero() {
		return rel
	}

	// Try absolute date formats
	return parseTimeOrZero(s)
}

// applyDueFilter filters rows based on a due date filter token.
// Supported formats: due.before:DATE, due.after:DATE, due.on:DATE, due:overdue
func applyDueFilter(rows []map[string]string, dueToken string) []map[string]string {
	dueToken = strings.TrimSpace(dueToken)
	if dueToken == "" {
		return rows
	}

	out := make([]map[string]string, 0, len(rows))
	now := time.Now()

	// Parse filter type and date
	if dueToken == "due:overdue" || dueToken == "overdue" {
		// Filter for overdue tasks (before today, not including today)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		for _, r := range rows {
			dueStr := r["due"]
			if dueStr == "" {
				continue
			}
			dueTime := parseDueDate(dueStr)
			if dueTime.IsZero() {
				// If we can't parse, use isOverdue helper
				if isOverdue(dueStr) {
					out = append(out, r)
				}
			} else {
				// Normalize to start of day for comparison
				dueTime = time.Date(dueTime.Year(), dueTime.Month(), dueTime.Day(), 0, 0, 0, 0, dueTime.Location())
				if dueTime.Before(today) {
					out = append(out, r)
				}
			}
		}
		return out
	}

	// Parse due.before:DATE, due.after:DATE, due.on:DATE
	var filterType string
	var dateStr string

	if strings.HasPrefix(dueToken, "due.before:") {
		filterType = "before"
		dateStr = dueToken[len("due.before:"):]
	} else if strings.HasPrefix(dueToken, "due.after:") {
		filterType = "after"
		dateStr = dueToken[len("due.after:"):]
	} else if strings.HasPrefix(dueToken, "due.on:") {
		filterType = "on"
		dateStr = dueToken[len("due.on:"):]
	} else {
		// Unknown format, return original rows
		return rows
	}

	filterDate := parseDueDate(dateStr)
	if filterDate.IsZero() {
		// If we can't parse the filter date, return original rows
		return rows
	}

	// Normalize filter date to start of day for comparison
	filterDate = time.Date(filterDate.Year(), filterDate.Month(), filterDate.Day(), 0, 0, 0, 0, filterDate.Location())

	for _, r := range rows {
		dueStr := r["due"]
		if dueStr == "" {
			continue
		}

		dueTime := parseDueDate(dueStr)
		if dueTime.IsZero() {
			continue
		}

		// Normalize due date to start of day for comparison
		dueTime = time.Date(dueTime.Year(), dueTime.Month(), dueTime.Day(), 0, 0, 0, 0, dueTime.Location())

		switch filterType {
		case "before":
			if dueTime.Before(filterDate) {
				out = append(out, r)
			}
		case "after":
			if dueTime.After(filterDate) {
				out = append(out, r)
			}
		case "on":
			// Check if dates are on the same day
			if dueTime.Year() == filterDate.Year() && dueTime.Month() == filterDate.Month() && dueTime.Day() == filterDate.Day() {
				out = append(out, r)
			}
		}
	}

	return out
}

// buildDueFilterToken constructs a due filter token from query parameters.
// Returns empty string if no filter is specified.
func buildDueFilterToken(q url.Values) string {
	filterType := strings.TrimSpace(q.Get("dueFilterType"))
	filterDate := strings.TrimSpace(q.Get("dueFilterDate"))

	if filterType == "" {
		return ""
	}

	if filterType == "overdue" {
		return "due:overdue"
	}

	if filterDate == "" {
		return ""
	}

	return fmt.Sprintf("due.%s:%s", filterType, filterDate)
}

// parseTemplatesFromOutput extrahiert Templates aus JSON oder Plaintext-Ausgabe von `dstask show-templates`.
func parseTemplatesFromOutput(raw string) []map[string]string {
	templates := make([]map[string]string, 0, 16)

	// Try JSON parsing first
	if arr, ok := decodeTasksJSONFlexible(raw); ok && len(arr) > 0 {
		for _, m := range arr {
			id := str(firstOf(m, "id", "ID", "uuid"))
			if id == "" {
				continue
			}
			template := map[string]string{
				"id":      id,
				"summary": trimQuotes(str(firstOf(m, "summary", "Summary", "description", "Description"))),
				"project": trimQuotes(str(firstOf(m, "project", "Project"))),
				"tags":    joinTags(firstOf(m, "tags", "Tags")),
				"due":     trimQuotes(str(firstOf(m, "due", "Due", "dueDate"))),
			}
			templates = append(templates, template)
		}
		return templates
	}

	// Plaintext parsing: try to extract ID and summary from lines like "42  Template summary text"
	reTemplate := regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		// Skip headers
		if strings.HasPrefix(l, "ID") || strings.HasPrefix(l, "[") || strings.HasPrefix(l, "{") {
			continue
		}
		if m := reTemplate.FindStringSubmatch(l); len(m) == 3 {
			templates = append(templates, map[string]string{
				"id":      m[1],
				"summary": trimQuotes(m[2]),
				"project": "",
				"tags":    "",
				"due":     "",
			})
		}
	}

	return templates
}

// extractURLs extracts HTTP(S) URLs from a given text string
func extractURLs(text string) []string {
	urlRe := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
	matches := urlRe.FindAllString(text, -1)
	urls := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, u := range matches {
		u = strings.TrimRight(u, ".,;:!?)")
		if !seen[u] {
			urls = append(urls, u)
			seen[u] = true
		}
	}
	return urls
}

// linkifyURLs converts URLs in text to HTML anchor tags.
// Returns html/template.HTML to prevent escaping of the HTML tags.
func linkifyURLs(text string) template.HTML {
	if text == "" {
		return template.HTML("")
	}

	urlRe := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

	// Find all URL positions
	type urlPos struct {
		url   string
		start int
		end   int
	}
	var urls []urlPos
	for _, match := range urlRe.FindAllStringIndex(text, -1) {
		url := text[match[0]:match[1]]
		// Trim trailing punctuation
		url = strings.TrimRight(url, ".,;:!?)")
		urls = append(urls, urlPos{
			url:   url,
			start: match[0],
			end:   match[0] + len(url),
		})
	}

	if len(urls) == 0 {
		// No URLs found, escape and return as-is
		return template.HTML(template.HTMLEscapeString(text))
	}

	// Build result with linkified URLs
	var result strings.Builder
	lastEnd := 0
	for _, u := range urls {
		// Add text before URL (escaped)
		if u.start > lastEnd {
			result.WriteString(template.HTMLEscapeString(text[lastEnd:u.start]))
		}
		// Add linkified URL
		escapedURL := template.HTMLEscapeString(u.url)
		result.WriteString(`<a href="`)
		result.WriteString(escapedURL)
		result.WriteString(`" target="_blank" rel="noopener">`)
		result.WriteString(escapedURL)
		result.WriteString(`</a>`)
		lastEnd = u.end
	}
	// Add remaining text after last URL (escaped)
	if lastEnd < len(text) {
		result.WriteString(template.HTMLEscapeString(text[lastEnd:]))
	}

	return template.HTML(result.String())
}

// renderMarkdown konvertiert Markdown-Text zu HTML und gibt es als template.HTML zurück,
// damit es nicht noch einmal escaped wird.
func renderMarkdown(text string) template.HTML {
	if text == "" {
		return template.HTML("")
	}
	// Konvertiere Markdown zu HTML
	html := markdown.ToHTML([]byte(text), nil, nil)
	return template.HTML(html)
}
