package log

import "testing"

func TestParseLevel(t *testing.T) {
    cases := map[string]Level{
        "debug": Debug,
        "DEBUG": Debug,
        "info":  Info,
        "":      Info,
        "warn":  Warn,
        "warning": Warn,
        "error": Error,
        "err":   Error,
        "unknown": Info,
    }
    for in, want := range cases {
        if got := ParseLevel(in); got != want {
            t.Fatalf("ParseLevel(%q)=%v want %v", in, got, want)
        }
    }
}


