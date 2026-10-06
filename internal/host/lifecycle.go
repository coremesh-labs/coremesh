package host

import (
	"context"
	"fmt"

	"github.com/camel/coremesh/pkg/sdk"
)

// checkVersionResult ist die Antwort von DBSchema.CheckVersion.
type checkVersionResult struct {
	Migrated   bool     `json:"migrated"`
	ExecutedAt string   `json:"executed_at"`
	Installed  []string `json:"installed"`
}

// initSchema führt DBSchema.Init eines Moduls genau einmal pro Modulversion aus:
//
//  1. DBSchema.CheckVersion(module, version) beim Core-Plugin DBSchema
//  2. migrated == true:  Init wird übersprungen
//  3. migrated == false: Init auf dem Modul liefert Modulname + Soll-Schema (HCL),
//     DBSchema.Activate gleicht es per Atlas isoliert ab und registriert
//     (module, version) im selben Schritt.
//
// ctx gehört zur Wurzelanfrage aus activate.
func (h *Host) initSchema(ctx context.Context, m sdk.Manifest, p sdk.Plugin, svc *pluginHost) error {
	module := map[string]any{"module": m.Name, "version": m.Version}

	// 1. Versionsprüfung
	resp, err := h.disp.Call(ctx, sdk.Request{Object: sdk.ObjectDBSchema, Action: "CheckVersion", Payload: module})
	if err != nil {
		return fmt.Errorf("CheckVersion: %w", err)
	}
	var check checkVersionResult
	if err := sdk.Decode(resp.Payload, &check); err != nil {
		return err
	}

	// 2. Bereits migriert → überspringen
	if check.Migrated {
		h.log.Info("Schema aktuell – Init übersprungen", "plugin", m.Name, "version", m.Version, "migriert_am", check.ExecutedAt)
		return nil
	}

	// 3. Init auf dem Modul: liefert die Statements für diese Version
	resp, err = p.Handle(sdk.WithHost(ctx, svc), sdk.Request{
		Object: sdk.ObjectDBSchema,
		Action: sdk.ActionInit,
		Payload: sdk.SchemaInitRequest{
			Module:    m.Name,
			Version:   m.Version,
			Installed: check.Installed,
		},
	})
	if err != nil {
		return err
	}
	var init sdk.SchemaInitResponse
	if err := sdk.Decode(resp.Payload, &init); err != nil {
		return err
	}

	// Ein Modul liefert nur sein eigenes Schema.
	if init.Module != m.Name {
		return fmt.Errorf("%w: Init meldet Modul %q, erwartet %q", sdk.ErrPermissionDenied, init.Module, m.Name)
	}
	// Schema-Änderungen nur mit access: write auf der Schema-Datenbank.
	db := h.schemaDatabase()
	if g, ok := svc.grants[db]; !ok || g.Access != "write" {
		return fmt.Errorf("%w: Plugin %s braucht access: write auf Datenbank %q für Schema-Änderungen",
			sdk.ErrPermissionDenied, m.Name, db)
	}

	// Atlas-Diff/Apply isoliert auf die Tabellen des Moduls (DBSchema prüft
	// Präfix, Fremdverweise und destruktive Änderungen).
	resp, err = h.disp.Call(ctx, sdk.Request{
		Object: sdk.ObjectDBSchema,
		Action: "Activate",
		Payload: map[string]any{
			"module": m.Name, "version": m.Version, "schema": init.Schema, "seed": init.Seed,
		},
	})
	if err != nil {
		return fmt.Errorf("Activate: %w", err)
	}
	h.log.Info("Schema migriert", "plugin", m.Name, "version", m.Version, "ergebnis", resp.Payload)
	return nil
}

// ActivateSchema legt beim Host-Start die Migrationstabelle an und wendet die
// Host-eigenen Migrationen an (DBSchema.Activate ohne Payload). Activate ist
// eine Host-Route und daher nicht über Handle erreichbar.
func (h *Host) ActivateSchema(ctx context.Context) (sdk.Response, error) {
	ctx, end, err := h.disp.Begin(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	defer end()
	return h.disp.Call(ctx, sdk.Request{Object: sdk.ObjectDBSchema, Action: "Activate"})
}

// schemaIsolation meldet, ob DBSchema mit isolation: schema läuft.
func (h *Host) schemaIsolation() bool {
	s, _ := h.cfg.Plugins["dbschema"].Settings["isolation"].(string)
	return s == "schema"
}

// schemaDatabase ist die Standard-Datenbank des DBSchema-Plugins.
func (h *Host) schemaDatabase() string {
	if s, ok := h.cfg.Plugins["dbschema"].Settings["database"].(string); ok && s != "" {
		return s
	}
	return "main"
}
