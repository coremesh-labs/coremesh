// Package textmanagement ist das Modul der Texte: Anmerkungen, Beschreibungen
// und andere Texte an jedem Datensatz jedes Objects – mit Kategorie, Position
// und Sprachschlüssel. Es teilt sich nur das Plugin mit dem TagManagement.
//
//   - Kategorie (TextCategory): einmal (SINGLE, Position 1) oder mehrfach
//     (MULTIPLE, Positionen 1, 2, …) je Datensatz;
//   - Zuordnung (TextAssignment): welche Kategorie an welchem Object erlaubt ist,
//     je Buchungskreis oder für alle (*), Pflicht oder optional, optional nur für
//     Datensätze mit bestimmten Feldwerten (z. B. contract_type = MV);
//   - Sprache (TextLanguage): Sprachschlüssel; je Position gibt es den Text in
//     mehreren Sprachen.
//
// Schnittstelle nach außen ist nur pkg/sdk/textservice (Object Texts). Die
// Rechte folgen dem Ziel-Datensatz: lesen = <Object>.get, schreiben =
// <Object>.update im Buchungskreis des Datensatzes.
package textmanagement

import (
	"context"
	"embed"
	"log/slog"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
	"github.com/coremesh-labs/coremesh/pkg/sdk/textservice"
)

// Name ist der Namensraum des Moduls (/m/textmanagement).
const Name = "textmanagement"

type Module struct {
	db       module.DB
	services module.Services
	log      *slog.Logger
	set      *crud.Set
}

var (
	_ module.Module         = (*Module)(nil)
	_ module.SchemaProvider = (*Module)(nil)
	_ module.TablePrefixer  = (*Module)(nil)
	_ module.Translator     = (*Module)(nil)
)

func New() *Module {
	m := &Module{}
	m.set = crud.NewSet(m.category(), m.language(), m.assignment())
	return m
}

func (m *Module) Descriptor() module.Descriptor {
	return module.Descriptor{Name: Name, Title: "Texte", Icon: "icon-file",
		Description: "Textkategorien, Sprachen und Zuordnungen für Texte an Objects aller Module"}
}

func (m *Module) RegisterRoutes(r *module.Router) {
	m.set.Register(r, "Einstellungen")
	r.Object(textservice.Object).
		Handle(textservice.ActionList, m.listAction).
		Handle(textservice.ActionSave, m.saveAction).
		Handle(textservice.ActionRemove, m.removeAction).
		Handle(textservice.ActionCheck, m.checkAction).
		Handle(events.CallbackAction, m.onRekey)
}

func (m *Module) Initialize(ctx context.Context, env module.Env) error {
	m.db, m.services, m.log = env.DB, env.Services, env.Log
	m.set.Bind(env.DB)
	if err := events.Register(ctx, env.Services, events.Subscription{Object: events.All, Action: "rekey", CompanyCode: events.All,
		Callback: textservice.Object}); err != nil {
		m.log.WarnContext(ctx, "Event nicht abonniert", "action", "rekey", "err", err.Error())
	}
	m.log.InfoContext(ctx, "Modul bereit", "database", env.DB.Name())
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }

func (m *Module) Schema() module.Schema {
	return module.Schema{HCL: schemaHCL, Seed: []sdk.SchemaSeed{{Table: "tag__text_category", Rows: seedCategories},
		{Table: "tag__text_language", Rows: seedLanguages}}}
}

// TablePrefix: eigene Tabellen tag__text_… und tag__text neben denen des TagManagements.
func (m *Module) TablePrefix() string { return "tag__text" }

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")

func (m *Module) Translations() metamodel.Translations { return translations }
