// Package tagmanagement ist das fachliche Modul TagManagement: Tags für
// beliebige Objects anderer Module – polymorph über target_entity_type und
// target_entity_id, typisiert (STRING, INTEGER, CURRENCY, DATE, TIMESTAMP),
// gebündelt in Tag Sets mit Regeln, je Objekttyp und Buchungskreis
// zugewiesen und durchgängig historisiert.
//
// „Internal first, service-ready“: Das Modul ist ein eigenes Plugin mit
// eigenem Datenbank-Namensraum (tag__). Andere Module erreichen es nur über
// die Service-Schnittstelle pkg/sdk/tagservice (Actions des Objects Tags) –
// nie über Tabellen. Eine Auslagerung als eigener Dienst ändert für sie nichts.
package tagmanagement

import (
	"context"
	"embed"
	"log/slog"

	"github.com/camel/coremesh/pkg/sdk/crud"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/module"
	"github.com/camel/coremesh/pkg/sdk/tagservice"
)

// Name ist der Namensraum des Moduls (/m/tagmanagement, /api/v1/tagmanagement).
const Name = "tagmanagement"

// Module ist das TagManagement.
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

// New erzeugt das Modul.
func New() *Module {
	m := &Module{}
	m.set = crud.NewSet(m.entities()...)
	return m
}

func (m *Module) Descriptor() module.Descriptor {
	return module.Descriptor{Name: Name, Title: "Tags", Icon: "icon-tag",
		Description: "Tags, Tag Sets und Regeln für Objects aller Module"}
}

// RegisterRoutes: Verwaltung der Definitionen (CRUD) und der TagService.
func (m *Module) RegisterRoutes(r *module.Router) {
	m.set.Register(r, "Definition")
	r.Object(tagservice.Object).
		Handle("schema", m.schemaAction).
		Handle("get", m.getAction).
		Handle("set", m.setAction).
		Handle("validate", m.validateAction).
		Handle("history", m.historyAction).
		Handle("find", m.findAction)
}

func (m *Module) Initialize(ctx context.Context, env module.Env) error {
	m.db, m.services, m.log = env.DB, env.Services, env.Log
	m.set.Bind(env.DB)
	m.log.InfoContext(ctx, "Modul bereit", "database", env.DB.Name())
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }

func (m *Module) Schema() module.Schema { return module.Schema{HCL: schemaHCL} }

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")

func (m *Module) Translations() metamodel.Translations { return translations }
