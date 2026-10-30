//go:build linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestService: Unit schreiben und einschalten; start und stop laufen dann über
// systemctl --user, remove entfernt die Unit und den Eintrag.
func TestService(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(t.TempDir(), "mein 100%")
	in := &installation{dir: dir}
	if err := os.MkdirAll(in.path("data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(in.path("current"), []byte("2026.10.30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls []string
	old := systemctl
	t.Cleanup(func() { systemctl = old })
	systemctl = func(args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		if args[0] == "start" {
			return errors.New("kein systemd im Test")
		}
		return nil
	}
	if got := defaultUnit(dir); got != "coremesh-mein-100" {
		t.Fatalf("Name: %s", got)
	}
	if err := installService(in, "a b", func(string) {}); err == nil {
		t.Fatal("ungültiger Name")
	}
	err := installService(in, "", func(string) {})
	if err == nil || !strings.Contains(err.Error(), "kein systemd") {
		t.Fatalf("Start über den Dienst: %v", err)
	}
	unit := filepath.Join(cfg, "systemd", "user", "coremesh-mein-100.service")
	b, err := os.ReadFile(unit)
	if err != nil {
		t.Fatal(err)
	}
	want := `ExecStart="` + strings.ReplaceAll(in.path("coremesh-setup"), "%", "%%") + `" run --dir="` + strings.ReplaceAll(dir, "%", "%%") + `"`
	if !strings.Contains(string(b), want+"\n") || !strings.Contains(string(b), "WantedBy=default.target") {
		t.Fatalf("Unit:\n%s\nerwartet %s", b, want)
	}
	if in.state().Service != "coremesh-mein-100" {
		t.Fatalf("setup.json: %+v", in.state())
	}
	calls = nil
	if err := in.stop(); err != nil {
		t.Fatal(err)
	}
	if err := removeService(in, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "stop coremesh-mein-100,stop coremesh-mein-100,disable coremesh-mein-100,daemon-reload" {
		t.Fatalf("Aufrufe: %v", calls)
	}
	if _, err := os.Stat(unit); !os.IsNotExist(err) || in.state().Service != "" {
		t.Fatalf("entfernt: %v %+v", err, in.state())
	}
}
