package hook

import (
	"embed"

	hookapi "github.com/coremesh-labs/coremesh/pkg/sdk/hook"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// schemaHCL: Hooks und Abos (Präfix hook__). Abos werden bei jedem Start der
// Abonnenten erneuert; locked bleibt dabei erhalten.
const schemaHCL = `
schema "main" {}

table "hook__hooks" {
  schema = schema.main
  column "name" {
    type = text
  }
  column "owner" {
    type = text
    null = true
  }
  column "description" {
    type = text
    null = true
  }
  # Phasen, kommagetrennt (modify, check, commit)
  column "phases" {
    type = text
  }
  # Aufbau der Daten (Vertrag zwischen den Modulen)
  column "data_doc" {
    type = text
    null = true
  }
  # block | skip
  column "on_failure" {
    type    = text
    default = "block"
  }
  column "defined_at" {
    type = text
  }
  column "updated_at" {
    type = text
  }
  primary_key {
    columns = [column.name]
  }
}

table "hook__subscriptions" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "hook" {
    type = text
  }
  column "phase" {
    type = text
  }
  column "callback" {
    type = text
  }
  # Plugin des Callback-Objects (aus der Routing-Tabelle)
  column "subscriber" {
    type = text
    null = true
  }
  column "priority" {
    type    = integer
    default = 100
  }
  column "description" {
    type = text
    null = true
  }
  column "locked" {
    type    = integer
    default = 0
  }
  column "locked_by" {
    type = text
    null = true
  }
  column "locked_at" {
    type = text
    null = true
  }
  column "registered_at" {
    type = text
  }
  column "last_seen" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "hook__subscriptions_key" {
    unique  = true
    columns = [column.hook, column.phase, column.callback]
  }
}
`

var phaseOptions = []metamodel.Option{
	{Value: hookapi.PhaseModify, Label: "modify – Daten ändern (vor dem Speichern)"},
	{Value: hookapi.PhaseCheck, Label: "check – prüfen (vor dem Speichern)"},
	{Value: hookapi.PhaseCommit, Label: "commit – Folgeaktionen (nach dem Speichern)"},
}

var (
	hookDef = metamodel.ObjectDefinition{
		Name: "Hook", Title: "Hooks", Icon: "icon-link", TitleField: "name",
		Fields: []metamodel.FieldDefinition{
			{Key: "name", Label: "Hook", Type: metamodel.TypeText, Listable: true},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeText, Listable: true},
			{Key: "owner", Label: "Modul", Type: metamodel.TypeText, Listable: true},
			{Key: "phases", Label: "Phasen", Type: metamodel.TypeText, Listable: true},
			{Key: "on_failure", Label: "Abonnent nicht erreichbar", Type: metamodel.TypeSelect, Listable: true, Options: []metamodel.Option{
				{Value: hookapi.FailBlock, Label: "blockieren"}, {Value: hookapi.FailSkip, Label: "überspringen (Warnung)"}}},
			{Key: "subscriptions", Label: "Aktive Abos", Type: metamodel.TypeNumber, Listable: true},
			{Key: "data_doc", Label: "Aufbau der Daten", Type: metamodel.TypeTextarea},
			{Key: "defined", Label: "Definiert", Type: metamodel.TypeBoolean, Listable: true},
			{Key: "updated_at", Label: "Zuletzt angemeldet", Type: metamodel.TypeText},
		},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "abos", Title: "Abos", Relation: &metamodel.Relation{Object: "HookSubscription", ForeignKey: "hook",
				Columns: []string{"phase", "priority", "callback", "subscriber", "description", "status"}}},
		},
	}
	subscriptionDef = metamodel.ObjectDefinition{
		Name: "HookSubscription", Title: "Hook-Abos", Icon: "icon-list",
		Fields: []metamodel.FieldDefinition{
			{Key: "hook", Label: "Hook", Type: metamodel.TypeText, Listable: true,
				Lookup: &metamodel.Lookup{Object: "Hook", ValueField: "name", LabelFields: []string{"name"}}},
			{Key: "phase", Label: "Phase", Type: metamodel.TypeSelect, Listable: true, Options: phaseOptions},
			{Key: "priority", Label: "Priorität", Type: metamodel.TypeNumber, Listable: true},
			{Key: "callback", Label: "Abonnent (Object)", Type: metamodel.TypeText, Listable: true},
			{Key: "subscriber", Label: "Plugin", Type: metamodel.TypeText, Listable: true},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeText, Listable: true},
			{Key: "status", Label: "Status", Type: metamodel.TypeSelect, Listable: true, Options: []metamodel.Option{
				{Value: "active", Label: "aktiv"}, {Value: "locked", Label: "gesperrt"}, {Value: "unreachable", Label: "nicht erreichbar"}}},
			{Key: "locked_by", Label: "Gesperrt von", Type: metamodel.TypeText},
			{Key: "locked_at", Label: "Gesperrt am", Type: metamodel.TypeText},
			{Key: "registered_at", Label: "Erstmals angemeldet", Type: metamodel.TypeText},
			{Key: "last_seen", Label: "Zuletzt angemeldet", Type: metamodel.TypeText},
		},
		Filters: []string{"hook", "phase"},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
			{Name: "lock", Kind: metamodel.KindCustom, Label: "Sperren", Record: true,
				Confirm: "Abo sperren? Der Abonnent wird nicht mehr aufgerufen – auch nach einem Neustart nicht."},
			{Name: "unlock", Kind: metamodel.KindCustom, Label: "Entsperren", Record: true, Confirm: "Abo wieder aktivieren?"},
		},
	}
	hooksModule = metamodel.ModuleDefinition{
		Name: "hooks", Title: "Erweiterungen", Icon: "icon-link",
		Description: "Hooks der Module und ihre Abos",
		Objects: []metamodel.ModuleObject{
			{Object: "Hook", Section: "Hooks"},
			{Object: "HookSubscription", Section: "Hooks"},
		},
	}
)

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")
