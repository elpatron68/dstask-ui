package server

import "testing"

func TestDecodeTasksJSONFlexible_WithLeadingText(t *testing.T) {
	in := "garbage before\n[ {\n  \"id\": 1, \"summary\": \"x\"\n} ]\ntrailing"
	arr, ok := decodeTasksJSONFlexible(in)
	if !ok || len(arr) != 1 {
		t.Fatalf("expected to decode 1 task from flexible JSON")
	}
}

func TestParseOpenPlain_SimpleLine(t *testing.T) {
	raw := "ID  P  Project  Summary\n1  P2  demo  Fix bug"
	rows := parseOpenPlain(raw)
	if len(rows) != 1 || rows[0]["id"] != "1" || rows[0]["project"] != "demo" {
		t.Fatalf("parseOpenPlain failed: %v", rows)
	}
}
