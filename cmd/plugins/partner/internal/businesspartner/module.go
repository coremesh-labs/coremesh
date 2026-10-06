// Package businesspartner ist das fachliche Modul Geschäftspartner nach dem
// Vorbild des SAP-Business-Partner-Modells: ein Partner (Person oder
// Organisation) mit Rollen, zeitabhängigen Adressen, Kommunikation und
// Bankverbindungen sowie Finanzdaten je Buchungskreis für Debitor- und
// Kreditorrollen. Alle Typen und Rollen sind Stammdaten-Kataloge.
//
// Das Paket liegt unter internal/: Kein anderes Go-Modul kann Entitäten,
// Tabellen oder Hilfsfunktionen importieren. Nach außen sichtbar sind nur
// New (für main.go) und die registrierten Routen (Object.Action).
package businesspartner

import (
	"context"
	"log/slog"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/module"
)

// Name ist der Namensraum des Moduls (/m/businesspartner, /api/v1/businesspartner).
const Name = "businesspartner"

// Module ist das BusinessPartnerModule.
type Module struct {
	// Ressourcen aus Initialize (Dependency Injection) – danach unverändert,
	// daher ohne Mutex nebenläufig lesbar.
	db       module.DB
	services module.Services
	log      *slog.Logger

	byObject map[string]*entity
	order    []*entity
}

var (
	_ module.Module         = (*Module)(nil)
	_ module.SchemaProvider = (*Module)(nil)
)

// New erzeugt das Modul mit allen Entitäten.
func New() *Module {
	m := &Module{byObject: map[string]*entity{}}
	for _, e := range m.entities() {
		e.m = m
		m.byObject[e.Object] = e
		m.order = append(m.order, e)
	}
	return m
}

func (m *Module) Descriptor() module.Descriptor {
	return module.Descriptor{
		Name: Name, Title: "Geschäftspartner", Icon: "icon-users",
		Description: "Partner mit Rollen, Adressen, Kommunikation, Bankverbindungen und Buchungskreisdaten",
	}
}

// RegisterRoutes meldet je Entität die CRUD-Actions mit Metamodell an.
func (m *Module) RegisterRoutes(r *module.Router) {
	for _, e := range m.order {
		section := e.Section
		if section == "" {
			section = "Partnerdaten"
		}
		r.Object(e.Object).Section(section).Describe(e.definition()).
			Handle("list", payloadOnly(e.list)).
			Handle("get", payloadOnly(e.get)).
			Handle("create", payloadOnly(e.create)).
			Handle("update", payloadOnly(e.update)).
			Handle("delete", payloadOnly(e.delete))
	}
}

func payloadOnly(f func(ctx context.Context, payload any) (sdk.Response, error)) module.HandlerFunc {
	return func(ctx context.Context, req sdk.Request) (sdk.Response, error) { return f(ctx, req.Payload) }
}

// Initialize übernimmt Datenbank, Services und Logger.
func (m *Module) Initialize(ctx context.Context, env module.Env) error {
	m.db, m.services, m.log = env.DB, env.Services, env.Log
	m.log.InfoContext(ctx, "Modul bereit", "objects", len(m.order), "database", env.DB.Name())
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }

// Schema liefert Tabellen und Stammdaten der Kataloge (DBSchema.Init).
func (m *Module) Schema() module.Schema {
	return module.Schema{HCL: schemaHCL, Seed: seeds}
}
