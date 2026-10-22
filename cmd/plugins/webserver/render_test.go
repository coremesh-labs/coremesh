package main

import (
	"testing"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

func TestLocalTimestamp(t *testing.T) {
	old := time.Local
	time.Local, _ = time.LoadLocation("Europe/Berlin")
	defer func() { time.Local = old }()
	for in, want := range map[string]string{
		"2026-10-21T21:23:24Z":          "21.10.2026 23:23",
		"2026-01-05T08:00:00.123Z":      "05.01.2026 09:00",
		"2026-10-21T23:23:24+02:00":     "21.10.2026 23:23",
		"2026-10-21":                    "2026-10-21",
		"Rechnung 2026-10-21T21:23:24Z": "Rechnung 2026-10-21T21:23:24Z",
	} {
		if got := displayValue(record{"x": in}, metamodel.FieldDefinition{Key: "x", Type: metamodel.TypeText}); got != want {
			t.Errorf("%s: %s, erwartet %s", in, got, want)
		}
	}
}
