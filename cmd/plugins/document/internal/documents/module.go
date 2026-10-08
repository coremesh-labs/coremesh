// Package documents ist das Modul der Dokumentverweise: beliebig viele
// Dokumente (Dateiname, Ablageort oder Link) an jedem Datensatz jedes Objects,
// mit Dokumentart, Datum und Gültigkeit.
//
// Schnittstelle nach außen ist nur pkg/sdk/docservice (Object Documents) –
// ein Dokumentenmanagementsystem ersetzt dieses Plugin über ein Adapter-Plugin
// mit derselben Schnittstelle. Die Rechte folgen dem Ziel-Datensatz:
// lesen = <Object>.get als Benutzer, ändern = <Object>.update im Buchungskreis
// des Datensatzes. Entfernte Verweise bleiben gespeichert (removed_at).
package documents

import (
	"context"
	"embed"
	"log/slog"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/docservice"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Name ist der Namensraum des Moduls (/m/documents).
const Name = "documents"

type Module struct {
	db       module.DB
	services module.Services
	log      *slog.Logger
	set      *crud.Set
}

var (
	_ module.Module         = (*Module)(nil)
	_ module.SchemaProvider = (*Module)(nil)
	_ module.Translator     = (*Module)(nil)
)

func New() *Module {
	m := &Module{}
	m.set = crud.NewSet(m.docType())
	return m
}

func (m *Module) Descriptor() module.Descriptor {
	return module.Descriptor{Name: Name, Title: "Dokumente", Icon: "icon-file",
		Description: "Dokumentverweise an Verträgen, Rechnungen, Partnern und allen anderen Objects"}
}

func (m *Module) RegisterRoutes(r *module.Router) {
	m.set.Register(r, "Kataloge")
	r.Object(docservice.Object).
		Handle(docservice.ActionList, m.listAction).
		Handle(docservice.ActionAttach, m.attachAction).
		Handle(docservice.ActionUpdate, m.updateAction).
		Handle(docservice.ActionDetach, m.detachAction).
		Handle(docservice.ActionTypes, m.typesAction).
		Handle(events.CallbackAction, m.onRekey)
}

func (m *Module) Initialize(ctx context.Context, env module.Env) error {
	m.db, m.services, m.log = env.DB, env.Services, env.Log
	m.set.Bind(env.DB)
	if err := events.Register(ctx, env.Services, events.Subscription{Object: events.All, Action: "rekey", CompanyCode: events.All,
		Callback: docservice.Object}); err != nil {
		m.log.WarnContext(ctx, "Event nicht abonniert", "action", "rekey", "err", err.Error())
	}
	m.log.InfoContext(ctx, "Modul bereit", "database", env.DB.Name())
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }

func (m *Module) Schema() module.Schema {
	return module.Schema{HCL: schemaHCL, Seed: []sdk.SchemaSeed{{Table: "document__type", Rows: seedTypes}}}
}

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")

func (m *Module) Translations() metamodel.Translations { return translations }

// docType: Katalog der Dokumentarten.
func (m *Module) docType() *crud.Entity {
	return &crud.Entity{
		Object: "DocumentKind", Title: "Dokumentarten", Icon: "icon-tag", Table: "document__type", Section: "Kataloge",
		Keys: []string{"code"}, Order: "sort_order, code", StatusField: "is_active", TitleField: "name", Search: []string{"code", "name"},
		Fields: []crud.Field{
			{Key: "code", Label: "Code", Type: metamodel.TypeText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Bezeichnung", Type: metamodel.TypeText, Required: true, Listable: true},
			{Key: "sort_order", Label: "Reihenfolge", Type: metamodel.TypeNumber},
			{Key: "is_active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, ReadOnly: true},
		},
		Validate: func(_ context.Context, rec, _ crud.Record) error {
			if rec["sort_order"] == nil || crud.Str(rec["sort_order"]) == "" {
				rec["sort_order"] = 0
			}
			return nil
		},
	}
}
