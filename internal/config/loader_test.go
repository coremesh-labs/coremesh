package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadDirMergesAlphanumerically(t *testing.T) {
	t.Setenv("TEST_TOKEN", "geheim")
	dir := writeFiles(t, map[string]string{
		"03-domain.yaml": `
plugins:
  partner-service:
    version: 1.2.0
    args: [b]
    settings: { token: "${env:TEST_TOKEN}" }
    databases: { main: { access: write } }
`,
		"01-system.yaml": `
host: { plugin_dir: ./p, tx_timeout: 5s }
databases:
  main: { driver: sqlite, dsn: "file:a.db" }
plugins:
  partner-service:
    version: 1.0.0
    args: [a, x]
    settings: { region: CH }
`,
		"02-db.yaml": "plugins:\n  dbschema: { kind: internal }\n",
		"README.md":  "wird ignoriert",
	})

	cfg, files, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || !strings.HasSuffix(files[0], "01-system.yaml") || !strings.HasSuffix(files[2], "03-domain.yaml") {
		t.Fatalf("Dateien/Reihenfolge: %v", files)
	}
	p := cfg.Plugins["partner-service"]
	if p.Version != "1.2.0" {
		t.Errorf("spätere Datei muss Version überschreiben: %s", p.Version)
	}
	if len(p.Args) != 1 || p.Args[0] != "b" {
		t.Errorf("Listen werden ersetzt, nicht gemergt: %v", p.Args)
	}
	if p.Settings["region"] != "CH" || p.Settings["token"] != "geheim" {
		t.Errorf("Maps werden rekursiv gemergt, ${env:} ersetzt: %v", p.Settings)
	}
	if p.Kind != KindExternal || cfg.Plugins["dbschema"].Kind != KindInternal {
		t.Errorf("Kind-Defaults: %+v", cfg.Plugins)
	}
	if cfg.Host.TxTimeout != 5*time.Second || cfg.Host.MaxCallDepth != 8 {
		t.Errorf("Host: %+v", cfg.Host)
	}
}

func TestLoadDirErrors(t *testing.T) {
	cases := map[string]string{
		"unbekannter Schlüssel": "host: { plugin_dri: x }\n",
		"fehlende Env":          "plugins:\n  ab: { version: 1.0.0, settings: { t: \"${env:GIBT_ES_NICHT_123}\" } }\n",
		"ungültiger Name":       "plugins:\n  Partner_Service: { version: 1.0.0 }\n",
		"Grant ohne Datenbank":  "plugins:\n  ab: { version: 1.0.0, databases: { main: { access: read } } }\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := LoadDir(writeFiles(t, map[string]string{"01.yaml": content})); err == nil {
				t.Fatal("Fehler erwartet")
			}
		})
	}
}
