package dstask

// Task ist eine flexible Repräsentation für die Ausgabe von `dstask export`.
// Wir verwenden eine Map, um Schema-Änderungen robust zu tolerieren.
type Task = map[string]any


