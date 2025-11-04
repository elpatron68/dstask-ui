package server

import (
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/elpatron68/dstask-ui/internal/auth"
    "github.com/elpatron68/dstask-ui/internal/config"
)

func TestRenderExportTable_ShowsHovercard_NoButton(t *testing.T) {
    cfg := config.Default()
    s := NewServerWithConfig(auth.NewInMemoryUserStore(), cfg)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/open?html=1", nil)

    rows := []map[string]string{
        {
            "id":       "1",
            "status":   "pending",
            "summary":  "Test task",
            "project":  "demo",
            "priority": "P2",
            "due":      "",
            "tags":     "",
            "created":  "2025-01-01",
            "resolved": "",
            "notes":    "Hello\n\n- a\n- b",
        },
    }
    s.renderExportTable(rr, req, "Open", rows)
    body := rr.Body.String()
    if !strings.Contains(body, "hovercard") {
        t.Fatalf("expected hovercard in output, got: %s", body)
    }
    if strings.Contains(body, ">Show notes<") || strings.Contains(body, "notes-row-") {
        t.Fatalf("unexpected legacy notes button/row in output")
    }
}


