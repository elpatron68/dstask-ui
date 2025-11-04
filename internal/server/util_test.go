package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPriorityRank(t *testing.T) {
	if priorityRank("P0") != 0 || priorityRank("P3") != 3 || priorityRank("x") != 99 {
		t.Fatalf("priorityRank unexpected")
	}
}

func TestQuoteIfNeeded(t *testing.T) {
	if quoteIfNeeded("noSpaces") != "noSpaces" {
		t.Fatalf("no change expected")
	}
	got := quoteIfNeeded("has space")
	if got != "\"has space\"" {
		t.Fatalf("quote expected, got %q", got)
	}
}

func TestNormalizeTag(t *testing.T) {
	if normalizeTag("a b ") != "a-b" {
		t.Fatalf("normalizeTag failed")
	}
}

func TestTrimQuotes(t *testing.T) {
	if trimQuotes("\"x\"") != "x" {
		t.Fatalf("trimQuotes failed")
	}
	if trimQuotes("x") != "x" {
		t.Fatalf("trimQuotes no-op failed")
	}
}

func TestActiveFromPath(t *testing.T) {
	if activeFromPath("/open?x=1") != "open" {
		t.Fatalf("activeFromPath open failed")
	}
	if activeFromPath("/") != "home" {
		t.Fatalf("activeFromPath home failed")
	}
}

func TestApplyQueryFilter(t *testing.T) {
	rows := []map[string]string{
		{"summary": "fix bug", "project": "core", "tags": "ui, bug"},
		{"summary": "add feature", "project": "api", "tags": "feat"},
	}
	out := applyQueryFilter(rows, "+bug")
	if len(out) != 1 {
		t.Fatalf("expected 1 row, got %d", len(out))
	}
	out = applyQueryFilter(rows, "project:api")
	if len(out) != 1 || out[0]["project"] != "api" {
		t.Fatalf("project filter failed")
	}
	out = applyQueryFilter(rows, "fix")
	if len(out) != 1 {
		t.Fatalf("text filter failed")
	}
}

func TestApplyDueFilter(t *testing.T) {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	yesterday := now.AddDate(0, 0, -1)

	rows := []map[string]string{
		{"id": "1", "summary": "task 1", "due": yesterday.Format("2006-01-02")},
		{"id": "2", "summary": "task 2", "due": now.Format("2006-01-02")},
		{"id": "3", "summary": "task 3", "due": tomorrow.Format("2006-01-02")},
		{"id": "4", "summary": "task 4", "due": ""},
	}

	// Test due:overdue (should only include yesterday, not today)
	out := applyDueFilter(rows, "due:overdue")
	if len(out) < 1 {
		t.Fatalf("overdue filter failed: expected at least 1 row, got %d rows", len(out))
	}
	// Verify that task 1 (yesterday) is included
	found := false
	for _, r := range out {
		if r["id"] == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("overdue filter failed: expected task 1 to be included")
	}

	// Test due.before
	out = applyDueFilter(rows, "due.before:"+tomorrow.Format("2006-01-02"))
	if len(out) < 2 { // Should include yesterday and today
		t.Fatalf("before filter failed: expected at least 2 rows, got %d", len(out))
	}

	// Test due.after
	out = applyDueFilter(rows, "due.after:"+yesterday.Format("2006-01-02"))
	if len(out) < 2 { // Should include today and tomorrow
		t.Fatalf("after filter failed: expected at least 2 rows, got %d", len(out))
	}

	// Test due.on
	out = applyDueFilter(rows, "due.on:"+now.Format("2006-01-02"))
	if len(out) != 1 || out[0]["id"] != "2" {
		t.Fatalf("on filter failed: expected 1 row with id=2, got %d rows", len(out))
	}

	// Test empty filter
	out = applyDueFilter(rows, "")
	if len(out) != len(rows) {
		t.Fatalf("empty filter should return all rows: expected %d, got %d", len(rows), len(out))
	}
}

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"No URLs here", 0},
		{"Check https://example.com for details", 1},
		{"See https://foo.com and http://bar.org", 2},
		{"Same URL: https://test.com and https://test.com", 1}, // deduplicate
		{"URL with trailing punct: https://example.com.", 1},
	}

	for _, tt := range tests {
		urls := extractURLs(tt.input)
		if len(urls) != tt.want {
			t.Errorf("extractURLs(%q) = %d URLs, want %d", tt.input, len(urls), tt.want)
		}
	}
}

func TestSummaryTokens(t *testing.T) {
	tests := []struct {
		input        string
		wantContains string // Token sollte enthalten sein
	}{
		{"test http://localhost:8080 tasks", "http://localhost:8080"},
		{"fix bug https://example.com/page with details", "https://example.com/page"},
		{"task / note separator", "task"},                                              // '/' sollte entfernt werden (außerhalb URLs)
		{"task https://example.com/path with / separator", "https://example.com/path"}, // URL sollte intakt bleiben
	}

	for _, tt := range tests {
		tokens := summaryTokens(tt.input)
		found := false
		for _, token := range tokens {
			if strings.Contains(token, tt.wantContains) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("summaryTokens(%q) should contain %q, got tokens: %v", tt.input, tt.wantContains, tokens)
		}
	}

	// Spezieller Test: URL sollte als ein Token kommen
	tokens := summaryTokens("test http://localhost:8080 new")
	urlFound := false
	for _, token := range tokens {
		if token == "http://localhost:8080" {
			urlFound = true
			break
		}
	}
	if !urlFound {
		t.Errorf("summaryTokens should preserve URL as single token, got: %v", tokens)
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	// Test that tokens are generated and are unique
	token1, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() failed: %v", err)
	}
	if token1 == "" {
		t.Fatalf("generateCSRFToken() returned empty token")
	}
	if len(token1) < 10 {
		t.Fatalf("generateCSRFToken() returned token too short: %d chars", len(token1))
	}

	// Generate another token - should be different
	token2, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() failed on second call: %v", err)
	}
	if token1 == token2 {
		t.Fatalf("generateCSRFToken() returned same token twice (should be unique)")
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "ANSI color codes",
			input:    "\x1b[31mRed\x1b[0m text",
			expected: "Red text",
		},
		{
			name:     "ANSI escape sequence",
			input:    "\u001B[1mBold\u001B[0m text",
			expected: "Bold text",
		},
		{
			name:     "multiple ANSI codes",
			input:    "\x1b[32mGreen\x1b[0m and \x1b[33mYellow\x1b[0m",
			expected: "Green and Yellow",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripANSI(tt.input)
			if result != tt.expected {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateCSRFToken(t *testing.T) {
	// Generate a valid token
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() failed: %v", err)
	}

	// Test with valid token in cookie
	req := httptest.NewRequest(http.MethodPost, "/tasks/batch", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	valid := validateCSRFToken(req, token)
	if !valid {
		t.Errorf("validateCSRFToken() with valid token should return true")
	}

	// Test with mismatched token
	req2 := httptest.NewRequest(http.MethodPost, "/tasks/batch", nil)
	req2.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	otherToken, _ := generateCSRFToken()
	valid2 := validateCSRFToken(req2, otherToken)
	if valid2 {
		t.Errorf("validateCSRFToken() with mismatched token should return false")
	}

	// Test with missing cookie
	req3 := httptest.NewRequest(http.MethodPost, "/tasks/batch", nil)
	valid3 := validateCSRFToken(req3, token)
	if valid3 {
		t.Errorf("validateCSRFToken() with missing cookie should return false")
	}

	// Test with empty token in form
	req4 := httptest.NewRequest(http.MethodPost, "/tasks/batch", nil)
	req4.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	valid4 := validateCSRFToken(req4, "")
	if valid4 {
		t.Errorf("validateCSRFToken() with empty form token should return false")
	}

	// Test with empty cookie value
	req5 := httptest.NewRequest(http.MethodPost, "/tasks/batch", nil)
	req5.AddCookie(&http.Cookie{Name: "csrf_token", Value: ""})
	valid5 := validateCSRFToken(req5, token)
	if valid5 {
		t.Errorf("validateCSRFToken() with empty cookie value should return false")
	}
}
