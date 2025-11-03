package ui

import (
    "fmt"
    "testing"
)

func TestCommandLogAppendAndListLimit(t *testing.T) {
    s := NewCommandLogStore(3)
    for i := 0; i < 5; i++ {
        s.Append("alice", "ctx", []string{"cmd", "arg", fmt.Sprintf("%d", i)})
    }
    // only last 3 retained
    got := s.List("alice", 10)
    if len(got) != 3 { t.Fatalf("expected 3 entries, got %d", len(got)) }
}

func TestJoinArgs(t *testing.T) {
    if JoinArgs([]string{"dstask", "next"}) != "dstask next" {
        t.Fatalf("join failed")
    }
}


