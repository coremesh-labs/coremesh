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
	"embed"
	"log/slog"

	"github.com/coremesh-lab/coremesh/pkg/sdk/crud"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-lab/coremesh/pkg/sdk/module"
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

	set *crud.Set // Entities (pkg/sdk/crud) mit der Datenbank des Moduls
}

var (
	_ module.Module         = (*Module)(nil)
	_ module.SchemaProvider = (*Module)(nil)
)

// New erzeugt das Modul mit allen Entitäten.
func New() *Module {
	m := &Module{}
	m.set = crud.NewSet(m.entities()...)
	return m
}

func (m *Module) Descriptor() module.Descriptor {
	return module.Descriptor{
		Name: Name, Title: "Geschäftspartner", Icon: "icon-users",
		Description: "Partner mit Rollen, Adressen, Kommunikation, Bankverbindungen und Buchungskreisdaten",
	}
}

// RegisterRoutes meldet je Entität die Actions mit Metamodell an. Statt
// delete gibt es je nach Lebenszyklus expire (Zeitscheibe) oder deactivate
// (Status-Flag); Entitäten ohne beides (immutable) haben keine Ende-Action.
func (m *Module) RegisterRoutes(r *module.Router) {
	m.set.Register(r, "Partnerdaten")
}

// Initialize übernimmt Datenbank, Services und Logger.
func (m *Module) Initialize(ctx context.Context, env module.Env) error {
	m.db, m.services, m.log = env.DB, env.Services, env.Log
	m.set.Bind(env.DB)
	m.log.InfoContext(ctx, "Modul bereit", "objects", len(m.set.Entities()), "database", env.DB.Name())
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }

// Schema liefert Tabellen und Stammdaten der Kataloge (DBSchema.Init).
func (m *Module) Schema() module.Schema {
	return module.Schema{HCL: schemaHCL, Seed: seeds}
}

// Übersetzungen (de, en, zh-CN) für Titel, Felder, Abschnitte und Navigation.
// Schlüssel nach der Konvention von metamodel.WithKeys; Framework-Texte
// (Speichern, Beenden …) bringt der WebServer mit.
//
//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")

// Translations implementiert module.Translator.
func (m *Module) Translations() metamodel.Translations { return translations }
