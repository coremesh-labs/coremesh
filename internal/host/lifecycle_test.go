package host

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/coreplugins/dbschema"
	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/pkg/sdk"
)

// moduleWithSchema zählt, wie oft der Host DBSchema.Init aufruft.
type moduleWithSchema struct {
	version   string
	inits     int
	installed []string
	spoof     string // meldet in Init einen fremden Modulnamen
}

func (m *moduleWithSchema) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{Name: "partner-service", Version: m.version, Capabilities: []sdk.Capability{
		{Object: "BusinessPartner", Actions: []string{"get"}},
		{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
	}}, nil
}

func (m *moduleWithSchema) Configure(context.Context, sdk.Config) error { return nil }

func (m *moduleWithSchema) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	if req.Action != sdk.ActionInit {
		return sdk.Response{}, nil
	}
	m.inits++
	var in sdk.SchemaInitRequest
	sdk.Decode(req.Payload, &in)
	m.installed = in.Installed
	// Deklarativ: immer das vollständige Soll-Schema; ab 1.1.0 mit Spalte name.
	extra := ""
	if m.version != "1.0.0" {
		extra = `column "name" {
    type = text
    null = true
  }`
	}
	module := in.Module
	if m.spoof != "" {
		module = m.spoof
	}
	return sdk.Response{Payload: sdk.SchemaInitResponse{Module: module, Schema: `
schema "main" {}
table "partner_service__business_partner" {
  schema = schema.main
  column "id" { type = text }
  ` + extra + `
  primary_key { columns = [column.id] }
}`}}, nil
}

// startHost simuliert einen Host-Start gegen dieselbe Datenbank.
func startHost(t *testing.T, dsn string, mod *moduleWithSchema, access string) error {
	t.Helper()
	cfg := &config.Config{
		Host:      config.Host{MaxCallDepth: 8},
		Databases: map[string]config.Database{"main": {Driver: "sqlite", DSN: dsn}},
		Plugins: map[string]config.Plugin{
			"dbschema":        {Kind: config.KindInternal},
			"partner-service": {Kind: config.KindExternal, Version: mod.version, Databases: map[string]config.Grant{"main": {Access: access}}},
		},
	}
	log := slog.New(slog.DiscardHandler)
	db, err := database.Open(context.Background(), cfg.Databases, database.Options{}, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	h, err := New(cfg, db, log)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := h.StartInternal(ctx, map[string]InternalFactory{
		dbschema.Name: func(d InternalDeps) sdk.Plugin { return dbschema.New(d.DB) },
	}); err != nil {
		t.Fatal(err)
	}
	pc := cfg.Plugins["partner-service"]
	return h.activate(ctx, "partner-service", mod, pc, h.services("partner-service", pc))
}

func TestInitRunsOncePerModuleVersion(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "h.db")

	v1 := &moduleWithSchema{version: "1.0.0"}
	if err := startHost(t, dsn, v1, "write"); err != nil {
		t.Fatal(err)
	}
	if v1.inits != 1 {
		t.Fatalf("1. Start: Init %d×", v1.inits)
	}

	again := &moduleWithSchema{version: "1.0.0"}
	if err := startHost(t, dsn, again, "write"); err != nil {
		t.Fatal(err)
	}
	if again.inits != 0 {
		t.Fatalf("2. Start, gleiche Version: Init muss übersprungen werden, war %d×", again.inits)
	}

	v2 := &moduleWithSchema{version: "1.1.0"}
	if err := startHost(t, dsn, v2, "write"); err != nil {
		t.Fatal(err)
	}
	if v2.inits != 1 || len(v2.installed) != 1 || v2.installed[0] != "1.0.0" {
		t.Fatalf("Upgrade: inits=%d installed=%v", v2.inits, v2.installed)
	}
}

func TestInitRequiresWriteGrant(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "h.db")
	if err := startHost(t, dsn, &moduleWithSchema{version: "1.0.0"}, "read"); err == nil {
		t.Fatal("ohne access: write muss Init scheitern")
	}
}

func TestInitRejectsForeignModuleName(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "h.db")
	if err := startHost(t, dsn, &moduleWithSchema{version: "1.0.0", spoof: "orders"}, "write"); err == nil {
		t.Fatal("Init mit fremdem Modulnamen muss scheitern")
	}
}
