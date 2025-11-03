package ui

import (
    "strings"
    "sync"
    "time"
)

type CommandEntry struct {
    When    time.Time
    Context string
    Args    []string
}

type CommandLogStore struct {
    mu        sync.Mutex
    userToBuf map[string][]CommandEntry
    max       int
}

func NewCommandLogStore(max int) *CommandLogStore {
    if max <= 0 { max = 200 }
    return &CommandLogStore{userToBuf: make(map[string][]CommandEntry), max: max}
}

func (s *CommandLogStore) SetMax(max int) {
    s.mu.Lock(); defer s.mu.Unlock()
    if max <= 0 { return }
    s.max = max
}

func (s *CommandLogStore) Append(username, ctx string, args []string) {
    s.mu.Lock(); defer s.mu.Unlock()
    buf := s.userToBuf[username]
    e := CommandEntry{When: time.Now(), Context: ctx, Args: append([]string(nil), args...)}
    buf = append(buf, e)
    if len(buf) > s.max {
        // drop oldest
        buf = buf[len(buf)-s.max:]
    }
    s.userToBuf[username] = buf
}

func (s *CommandLogStore) List(username string, n int) []CommandEntry {
    s.mu.Lock(); defer s.mu.Unlock()
    buf := s.userToBuf[username]
    if n <= 0 || n > len(buf) { n = len(buf) }
    out := make([]CommandEntry, 0, n)
    start := len(buf) - n
    for i := start; i < len(buf); i++ { out = append(out, buf[i]) }
    return out
}

func JoinArgs(args []string) string { return strings.Join(args, " ") }


