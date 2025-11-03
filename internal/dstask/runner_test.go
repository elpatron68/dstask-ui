package dstask

import "testing"

func TestNormalizeNewlines(t *testing.T) {
    in := "a\r\nb\rc\n"
    got := normalizeNewlines(in)
    if got != "a\nb\nc\n" { t.Fatalf("unexpected: %q", got) }
}

func TestJoinArgs(t *testing.T) {
    if JoinArgs("a", "", "b") != "a b" { t.Fatalf("JoinArgs failed") }
}

func TestTruncate(t *testing.T) {
    if truncate("hello", 10) != "hello" { t.Fatalf("truncate no-op") }
    if truncate("helloworld", 5) != "hello..." { t.Fatalf("truncate short") }
}


