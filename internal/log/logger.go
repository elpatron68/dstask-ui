package log

import (
    stdlog "log"
    "os"
    "strings"
)

type Level int

const (
    Debug Level = iota
    Info
    Warn
    Error
)

var current Level = Info

func ParseLevel(s string) Level {
    switch strings.ToLower(strings.TrimSpace(s)) {
    case "debug":
        return Debug
    case "info", "":
        return Info
    case "warn", "warning":
        return Warn
    case "err", "error":
        return Error
    default:
        return Info
    }
}

func SetLevel(l Level) { current = l }

func Debugf(format string, v ...any) {
    if current <= Debug { stdlog.Printf("[DEBUG] "+format, v...) }
}
func Infof(format string, v ...any)  { if current <= Info  { stdlog.Printf("[INFO] "+format, v...) } }
func Warnf(format string, v ...any)  { if current <= Warn  { stdlog.Printf("[WARN] "+format, v...) } }
func Errorf(format string, v ...any) { if current <= Error { stdlog.Printf("[ERROR] "+format, v...) } }

func InitFromEnvFallback(level string) {
    // Allow override via ENV if provided
    if env := os.Getenv("DSTWEB_LOG_LEVEL"); env != "" {
        level = env
    }
    SetLevel(ParseLevel(level))
}


