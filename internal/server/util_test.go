package server

import (
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
