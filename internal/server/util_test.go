package server

import "testing"

func TestPriorityRank(t *testing.T) {
    if priorityRank("P0") != 0 || priorityRank("P3") != 3 || priorityRank("x") != 99 {
        t.Fatalf("priorityRank unexpected")
    }
}

func TestQuoteIfNeeded(t *testing.T) {
    if quoteIfNeeded("noSpaces") != "noSpaces" { t.Fatalf("no change expected") }
    got := quoteIfNeeded("has space")
    if got != "\"has space\"" { t.Fatalf("quote expected, got %q", got) }
}

func TestNormalizeTag(t *testing.T) {
    if normalizeTag("a b ") != "a-b" { t.Fatalf("normalizeTag failed") }
}

func TestTrimQuotes(t *testing.T) {
    if trimQuotes("\"x\"") != "x" { t.Fatalf("trimQuotes failed") }
    if trimQuotes("x") != "x" { t.Fatalf("trimQuotes no-op failed") }
}

func TestActiveFromPath(t *testing.T) {
    if activeFromPath("/open?x=1") != "open" { t.Fatalf("activeFromPath open failed") }
    if activeFromPath("/") != "home" { t.Fatalf("activeFromPath home failed") }
}

func TestApplyQueryFilter(t *testing.T) {
    rows := []map[string]string{
        {"summary":"fix bug","project":"core","tags":"ui, bug"},
        {"summary":"add feature","project":"api","tags":"feat"},
    }
    out := applyQueryFilter(rows, "+bug")
    if len(out) != 1 { t.Fatalf("expected 1 row, got %d", len(out)) }
    out = applyQueryFilter(rows, "project:api")
    if len(out) != 1 || out[0]["project"] != "api" { t.Fatalf("project filter failed") }
    out = applyQueryFilter(rows, "fix")
    if len(out) != 1 { t.Fatalf("text filter failed") }
}


