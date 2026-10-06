package catalog

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/camel/coremesh/internal/dispatcher"
	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

type nop struct{}

func (nop) Handle(context.Context, sdk.Request) (sdk.Response, error) { return sdk.Response{}, nil }

// env baut einen Dispatcher mit mehreren Plugins und registriert den
// Katalog darin – so, wie der Host es tut.
type env struct {
	d   *dispatcher.Dispatcher
	cat *Plugin
	ctx context.Context
	end func()
}

func setup(t *testing.T, cacheDir string) *env {
	t.Helper()
	d := dispatcher.New(8, nil, slog.New(slog.DiscardHandler))
	reg := func(name, version string, caps ...sdk.Capability) {
		t.Helper()
		if err := d.Register(name, sdk.Manifest{Name: name, Version: version, Capabilities: caps}, nop{}); err != nil {
			t.Fatal(err)
		}
	}
	reg("partner", "1.2.0",
		sdk.Capability{Object: "BusinessPartner", Actions: []string{"list", "get"}, Description: "Geschäftspartner"},
		sdk.Capability{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}}) // Lebenszyklus
	reg("partner-search", "0.3.0",
		sdk.Capability{Object: "BusinessPartner", Actions: []string{"search"}})
	reg("realestate", "1.0.0",
		sdk.Capability{Object: "Property", Actions: []string{"list"}})
	reg("dbschema", "0.4.0",
		sdk.Capability{Object: sdk.ObjectDBSchema, Actions: []string{"Activate", "CheckVersion", "Status"}})

	cat := New(d.Catalog)
	m, _ := cat.Manifest(context.Background())
	ctx, end, _ := d.Begin(context.Background())
	t.Cleanup(end)
	if err := cat.Configure(ctx, sdk.Config{Settings: map[string]any{"cache_dir": cacheDir}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Register(Name, m, cat); err != nil {
		t.Fatal(err)
	}
	return &env{d: d, cat: cat, ctx: ctx, end: end}
}

// register ruft Catalog.Register wie der Host (Host-Route über Call).
func (e *env) register(module, version string, defs ...metamodel.ObjectDefinition) (map[string]any, error) {
	resp, err := e.d.Call(e.ctx, sdk.Request{Object: Object, Action: "Register",
		Payload: map[string]any{"module": module, "version": version, "objects": defs}})
	if err != nil {
		return nil, err
	}
	return resp.Payload.(map[string]any), nil
}

func (e *env) call(action string, payload any) (any, error) {
	resp, err := e.d.Handle(context.Background(), sdk.Request{Object: Object, Action: action, Payload: payload})
	return resp.Payload, err
}

func partnerDef() metamodel.ObjectDefinition {
	return metamodel.ObjectDefinition{
		Name: "BusinessPartner", Title: "Geschäftspartner", Icon: "icon-users",
		Fields: []metamodel.FieldDefinition{
			{Key: "company_name", Label: "Firmenname", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
		},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		},
	}
}

func TestRegisterAndGetDefinition(t *testing.T) {
	e := setup(t, "")
	got, err := e.register("partner", "1.2.0", partnerDef())
	if err != nil {
		t.Fatal(err)
	}
	if got["changed"] != true {
		t.Fatalf("Register: %v", got)
	}

	p, err := e.call("GetDefinition", map[string]any{"object": "BusinessPartner"})
	if err != nil {
		t.Fatal(err)
	}
	res := p.(map[string]any)
	def := res["definition"].(metamodel.ObjectDefinition)
	if def.Title != "Geschäftspartner" || res["module"] != "partner" || res["source"] != "registered" || res["available"] != true {
		t.Fatalf("GetDefinition: %v", res)
	}

	if _, err := e.call("GetDefinition", map[string]any{"object": "Property"}); !errors.Is(err, sdk.ErrNotFound) {
		t.Fatalf("Object ohne Definition: %v", err)
	}
}

func TestRegisterRejectsForeignObjectsAndActions(t *testing.T) {
	e := setup(t, "")
	foreign := partnerDef()
	foreign.Name, foreign.Title = "Property", "Liegenschaft"
	foreign.Actions = []metamodel.ActionConfig{{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"}}

	unknownAction := partnerDef()
	unknownAction.Actions = append(unknownAction.Actions, metamodel.ActionConfig{Name: "archive", Kind: metamodel.KindCustom, Label: "Archivieren"})

	foreignAction := partnerDef() // search gehört partner-search, nicht partner
	foreignAction.Actions = append(foreignAction.Actions, metamodel.ActionConfig{Name: "search", Kind: metamodel.KindCustom, Label: "Suchen"})

	invalid := partnerDef()
	invalid.Fields[0].Type = "money"

	for name, def := range map[string]metamodel.ObjectDefinition{
		"fremdes Object": foreign, "unbekannte Action": unknownAction, "fremde Action": foreignAction, "ungültig": invalid,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := e.register("partner", "1.2.0", def); !errors.Is(err, sdk.ErrPermissionDenied) {
				t.Fatalf("muss abgelehnt werden: %v", err)
			}
		})
	}

	// Nur ein Modul definiert ein Object – auch wenn mehrere Actions beisteuern.
	e.register("partner", "1.2.0", partnerDef())
	other := partnerDef()
	other.Actions = []metamodel.ActionConfig{{Name: "search", Kind: metamodel.KindCustom, Label: "Suchen"}}
	if _, err := e.register("partner-search", "0.3.0", other); !errors.Is(err, sdk.ErrAlreadyExists) {
		t.Fatalf("zweite Definition: %v", err)
	}
}

func TestRegisterIsHostOnly(t *testing.T) {
	e := setup(t, "")
	_, err := e.call("Register", map[string]any{"module": "partner", "version": "1", "objects": []any{}})
	if !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Register von außen: %v", err)
	}
}

func TestListObjectsAndActions(t *testing.T) {
	e := setup(t, "")
	e.register("partner", "1.2.0", partnerDef())

	p, err := e.call("ListObjects", nil)
	if err != nil {
		t.Fatal(err)
	}
	objs := map[string]ObjectInfo{}
	for _, o := range p.(map[string]any)["objects"].([]ObjectInfo) {
		objs[o.Object] = o
	}
	bp := objs["BusinessPartner"]
	if bp.Title != "Geschäftspartner" || !bp.Defined || !bp.Available || bp.Actions != 3 || len(bp.Plugins) != 2 {
		t.Fatalf("BusinessPartner: %+v", bp)
	}
	if objs["Property"].Defined || !objs["Property"].Available {
		t.Fatalf("Property: %+v", objs["Property"])
	}
	if objs["DBSchema"].Actions != 1 || objs["Catalog"].Actions != 6 { // ohne Host-Routen
		t.Fatalf("Host-Routen sichtbar: %+v / %+v", objs["DBSchema"], objs["Catalog"])
	}

	p, err = e.call("ListActions", map[string]any{"object": "BusinessPartner"})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(p.(map[string]any)["actions"].([]ActionInfo)); n != 3 {
		t.Fatalf("Actions: %d", n)
	}
}

func TestDiskCache(t *testing.T) {
	dir := t.TempDir()

	// 1. Lauf: Register schreibt den Cache.
	e := setup(t, dir)
	if got, _ := e.register("partner", "1.2.0", partnerDef()); got["changed"] != true {
		t.Fatalf("erster Lauf: %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, cacheName)); err != nil {
		t.Fatalf("Cache-Datei fehlt: %v", err)
	}

	// 2. Lauf, gleicher Stand: unverändert.
	e2 := setup(t, dir)
	if got, _ := e2.register("partner", "1.2.0", partnerDef()); got["changed"] != false {
		t.Fatalf("zweiter Lauf: %v", got)
	}

	// 3. Lauf, Modul läuft nicht: Definition kommt aus dem Cache.
	e3 := setup(t, dir)
	_ = e3.d.Unregister("partner")
	p, err := e3.call("GetDefinition", map[string]any{"object": "BusinessPartner"})
	if err != nil {
		t.Fatal(err)
	}
	if res := p.(map[string]any); res["source"] != "cache" || res["module"] != "partner" {
		t.Fatalf("aus Cache: %v", res)
	}
	p, _ = e3.call("ListObjects", map[string]any{"include_unavailable": true})
	for _, o := range p.(map[string]any)["objects"].([]ObjectInfo) {
		if o.Object == "BusinessPartner" && o.Title != "Geschäftspartner" {
			t.Fatalf("Titel aus Cache fehlt: %+v", o)
		}
	}

	// Beschädigter Cache wird verworfen, nicht fatal.
	os.WriteFile(filepath.Join(dir, cacheName), []byte("{kaputt"), 0o644)
	e4 := setup(t, dir)
	if _, err := e4.register("partner", "1.2.0", partnerDef()); err != nil {
		t.Fatal(err)
	}
}

func (e *env) registerModules(module, version string, defs []metamodel.ObjectDefinition, mods ...metamodel.ModuleDefinition) error {
	_, err := e.d.Call(e.ctx, sdk.Request{Object: Object, Action: "Register",
		Payload: map[string]any{"module": module, "version": version, "objects": defs, "modules": mods}})
	return err
}

func TestModules(t *testing.T) {
	e := setup(t, t.TempDir())
	bp := metamodel.ModuleDefinition{Name: "businesspartner", Title: "Geschäftspartner",
		Objects: []metamodel.ModuleObject{{Object: "BusinessPartner"}}}

	// Ein Modul bündelt nur eigene Objects mit Metamodell.
	foreign := bp
	foreign.Objects = []metamodel.ModuleObject{{Object: "BusinessPartner"}, {Object: "Property"}}
	if err := e.registerModules("partner", "1.2.0", []metamodel.ObjectDefinition{partnerDef()}, foreign); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("fremdes Object im Modul: %v", err)
	}
	if err := e.registerModules("partner", "1.2.0", []metamodel.ObjectDefinition{partnerDef()}, bp); err != nil {
		t.Fatal(err)
	}

	// Modulnamen sind systemweit eindeutig.
	prop := metamodel.ObjectDefinition{Name: "Property", Title: "Liegenschaft",
		Fields:  []metamodel.FieldDefinition{{Key: "name", Label: "Name", Type: metamodel.TypeText}},
		Actions: []metamodel.ActionConfig{{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"}}}
	taken := metamodel.ModuleDefinition{Name: "businesspartner", Title: "X", Objects: []metamodel.ModuleObject{{Object: "Property"}}}
	if err := e.registerModules("realestate", "1.0.0", []metamodel.ObjectDefinition{prop}, taken); !errors.Is(err, sdk.ErrAlreadyExists) {
		t.Fatalf("doppelter Modulname: %v", err)
	}
	re := metamodel.ModuleDefinition{Name: "realestate", Title: "Immobilien",
		Objects: []metamodel.ModuleObject{{Object: "Property", Section: "Bestand"}}}
	if err := e.registerModules("realestate", "1.0.0", []metamodel.ObjectDefinition{prop}, re); err != nil {
		t.Fatal(err)
	}

	p, err := e.call("ListModules", nil)
	if err != nil {
		t.Fatal(err)
	}
	mods := p.(map[string]any)["modules"].([]ModuleInfo)
	if len(mods) != 2 || mods[0].Name != "businesspartner" || mods[1].Name != "realestate" {
		t.Fatalf("Module (nach Titel sortiert): %+v", mods)
	}
	if o := mods[1].Objects[0]; o.Title != "Liegenschaft" || o.Section != "Bestand" || !o.Available || mods[1].Plugin != "realestate" {
		t.Fatalf("Modul-Object: %+v", mods[1])
	}

	resp, err := e.call("GetModule", map[string]any{"module": "realestate"})
	if err != nil || resp.(ModuleInfo).Title != "Immobilien" {
		t.Fatalf("GetModule: %+v %v", resp, err)
	}
	if _, err := e.call("GetModule", map[string]any{"module": "gibtsnicht"}); !errors.Is(err, sdk.ErrNotFound) {
		t.Fatalf("unbekanntes Modul: %v", err)
	}

	// ListObjects nennt das Modul jedes Objects.
	p, _ = e.call("ListObjects", nil)
	for _, o := range p.(map[string]any)["objects"].([]ObjectInfo) {
		if o.Object == "Property" && o.Module != "realestate" {
			t.Fatalf("ListObjects: %+v", o)
		}
	}
}

func TestRelationsOnlyToOwnObjects(t *testing.T) {
	e := setup(t, t.TempDir())
	bp := partnerDef()
	bp.Sections = []metamodel.SectionDefinition{{Key: "props", Title: "Liegenschaften",
		Relation: &metamodel.Relation{Object: "Property", ForeignKey: "bp_id"}}}
	if _, err := e.register("partner", "1.2.0", bp); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Relation auf fremdes Object: %v", err)
	}
}

func TestTranslations(t *testing.T) {
	e := setup(t, t.TempDir())
	bp := metamodel.ModuleDefinition{Name: "businesspartner", Title: "Geschäftspartner",
		Objects: []metamodel.ModuleObject{{Object: "BusinessPartner"}}}
	register := func(tr metamodel.Translations) error {
		_, err := e.d.Call(e.ctx, sdk.Request{Object: Object, Action: "Register", Payload: map[string]any{
			"module": "partner", "version": "1.2.0", "objects": []metamodel.ObjectDefinition{partnerDef()},
			"modules": []metamodel.ModuleDefinition{bp}, "translations": tr}})
		return err
	}
	if err := register(metamodel.Translations{"en": {"iam.User.title": "fremd"}}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("fremder Namensraum: %v", err)
	}
	if err := register(metamodel.Translations{
		"de":    {"businesspartner.module.title": "Geschäftspartner", "businesspartner.BusinessPartner.title": "Partner"},
		"zh-CN": {"businesspartner.module.title": "业务伙伴"},
	}); err != nil {
		t.Fatal(err)
	}
	p, err := e.call("Translations", map[string]any{"locale": "zh"})
	if err != nil {
		t.Fatal(err)
	}
	m := p.(map[string]any)
	tr := m["translations"].(map[string]string)
	// zh-CN überschreibt, fehlende Schlüssel fallen auf de zurück.
	if m["locale"] != "zh-CN" || tr["businesspartner.module.title"] != "业务伙伴" || tr["businesspartner.BusinessPartner.title"] != "Partner" {
		t.Fatalf("Übersetzungen: %v", m)
	}
	if _, err := e.call("Translations", map[string]any{"locale": "fr"}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("unbekannte Sprache: %v", err)
	}
}
