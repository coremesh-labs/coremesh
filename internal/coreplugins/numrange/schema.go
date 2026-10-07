package numrange

import (
	"embed"

	api "github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// schemaHCL: Objekte (von den Modulen angemeldet), Intervalle (vom
// Administrator gepflegt, fehlende legt der erste Abruf an) und Protokoll der
// vergebenen Nummern (Präfix numrange__).
const schemaHCL = `
schema "main" {}

table "numrange__object" {
  schema = schema.main
  column "object" {
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
  column "per_company_code" {
    type    = integer
    default = 0
  }
  column "per_year" {
    type    = integer
    default = 0
  }
  # Standardwerte für neue Intervalle
  column "pattern" {
    type = text
  }
  column "width" {
    type    = integer
    default = 0
  }
  column "from_number" {
    type = bigint
  }
  column "to_number" {
    type = bigint
  }
  column "overflow" {
    type = text
  }
  column "warn_percent" {
    type    = integer
    default = 90
  }
  column "defined_at" {
    type = text
  }
  column "updated_at" {
    type = text
  }
  primary_key {
    columns = [column.object]
  }
}

# Intervall je Objekt, Buchungskreis (* = alle), Schlüssel und Jahr (0 = ohne).
# current_number ist die zuletzt vergebene Nummer (0 = noch keine).
table "numrange__interval" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "object" {
    type = text
  }
  column "company_code" {
    type = text
  }
  column "range_key" {
    type = text
  }
  column "year" {
    type    = integer
    default = 0
  }
  column "description" {
    type = text
    null = true
  }
  column "from_number" {
    type = bigint
  }
  column "to_number" {
    type = bigint
  }
  column "current_number" {
    type    = bigint
    default = 0
  }
  column "width" {
    type    = integer
    default = 0
  }
  column "pattern" {
    type = text
  }
  # ERROR | RESTART | NEXT
  column "overflow" {
    type = text
  }
  # Folgeintervall bei NEXT (Schlüssel, gleiches Objekt, Buchungskreis und Jahr)
  column "next_key" {
    type = text
    null = true
  }
  column "warn_percent" {
    type    = integer
    default = 90
  }
  column "active" {
    type    = integer
    default = 1
  }
  column "created_at" {
    type = text
  }
  column "updated_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "numrange__interval_key" {
    unique  = true
    columns = [column.object, column.company_code, column.range_key, column.year]
  }
}

# Jede vergebene Nummer (Lücken erklären, Revision).
table "numrange__log" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "interval_id" {
    type = text
  }
  column "object" {
    type = text
  }
  column "company_code" {
    type = text
  }
  column "range_key" {
    type = text
  }
  column "year" {
    type = integer
  }
  column "value" {
    type = bigint
  }
  column "number" {
    type = text
  }
  column "reference" {
    type = text
    null = true
  }
  column "user_id" {
    type = text
    null = true
  }
  column "request_id" {
    type = text
    null = true
  }
  column "drawn_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "numrange__log_interval" {
    columns = [column.interval_id, column.value]
  }
}
`

var overflowOptions = []metamodel.Option{
	{Value: api.OverflowError, Label: "Fehler (nichts vergeben)"},
	{Value: api.OverflowRestart, Label: "von vorn beginnen"},
	{Value: api.OverflowNext, Label: "im Folgeintervall weiter"},
}

var (
	objectDef = metamodel.ObjectDefinition{
		Name: "NumberRangeObject", Title: "Nummernkreis-Objekte", Icon: "icon-hash", TitleField: "object",
		Fields: []metamodel.FieldDefinition{
			{Key: "object", Label: "Objekt", Type: metamodel.TypeText, Listable: true},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeText, Listable: true},
			{Key: "owner", Label: "Modul", Type: metamodel.TypeText, Listable: true},
			{Key: "per_company_code", Label: "Je Buchungskreis", Type: metamodel.TypeBoolean, Listable: true},
			{Key: "per_year", Label: "Je Jahr", Type: metamodel.TypeBoolean, Listable: true},
			{Key: "pattern", Label: "Standard-Format", Type: metamodel.TypeText, Listable: true},
			{Key: "width", Label: "Standard-Stellenzahl", Type: metamodel.TypeNumber},
			{Key: "from_number", Label: "Standard von", Type: metamodel.TypeNumber},
			{Key: "to_number", Label: "Standard bis", Type: metamodel.TypeNumber},
			{Key: "overflow", Label: "Standard bei Überlauf", Type: metamodel.TypeSelect, Options: overflowOptions},
			{Key: "warn_percent", Label: "Standard-Warnschwelle (%)", Type: metamodel.TypeNumber},
			{Key: "updated_at", Label: "Zuletzt angemeldet", Type: metamodel.TypeText},
		},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "intervalle", Title: "Intervalle", Relation: &metamodel.Relation{Object: "NumberRange", ForeignKey: "object",
				Columns: []string{"company_code", "range_key", "year", "from_number", "to_number", "current_number", "used_percent", "active"}}},
		},
	}
	intervalDef = metamodel.ObjectDefinition{
		Name: "NumberRange", Title: "Nummernkreise", Icon: "icon-hash", TitleField: "id",
		Fields: []metamodel.FieldDefinition{
			{Key: "id", Label: "ID", Type: metamodel.TypeText},
			{Key: "object", Label: "Objekt", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "NumberRangeObject", ValueField: "object", LabelFields: []string{"description"}}},
			{Key: "company_code", Label: "Buchungskreis (* = alle)", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "CompanyCode", ValueField: "code", LabelFields: []string{"description"}}},
			{Key: "range_key", Label: "Intervallschlüssel", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
			{Key: "year", Label: "Jahr (0 = ohne)", Type: metamodel.TypeNumber, Listable: true, Editable: true},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeText, Listable: true, Editable: true},
			{Key: "from_number", Label: "von", Type: metamodel.TypeNumber, Required: true, Listable: true, Editable: true, Group: "Intervall"},
			{Key: "to_number", Label: "bis", Type: metamodel.TypeNumber, Required: true, Listable: true, Editable: true, Group: "Intervall"},
			{Key: "current_number", Label: "Stand (zuletzt vergeben, 0 = keine)", Type: metamodel.TypeNumber, Listable: true, Editable: true, Group: "Intervall"},
			{Key: "used_percent", Label: "Belegt (%)", Type: metamodel.TypeNumber, Listable: true, Group: "Intervall"},
			{Key: "width", Label: "Stellenzahl (0 = ohne führende Nullen)", Type: metamodel.TypeNumber, Editable: true, Group: "Format"},
			{Key: "pattern", Label: "Format ({KEY}, {CC}, {YYYY}, {YY}, {N})", Type: metamodel.TypeText, Required: true, Editable: true, Group: "Format"},
			{Key: "next_number", Label: "Nächste Nummer", Type: metamodel.TypeText, Listable: true, Group: "Format"},
			{Key: "overflow", Label: "Bei Überlauf", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true, Options: overflowOptions, Group: "Überlauf"},
			{Key: "next_key", Label: "Folgeintervall (Schlüssel)", Type: metamodel.TypeText, Editable: true, Group: "Überlauf"},
			{Key: "warn_percent", Label: "Warnschwelle (%; 0 = keine)", Type: metamodel.TypeNumber, Editable: true, Group: "Überlauf"},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true},
			{Key: "updated_at", Label: "Geändert am", Type: metamodel.TypeText},
		},
		Filters:   []string{"object", "company_code", "range_key"},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
			{Name: "create", Kind: metamodel.KindCreate, Label: "Neu"},
			{Name: "update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
			{Name: "deactivate", Kind: metamodel.KindDeactivate, Label: "Inaktivieren",
				Confirm: "Intervall inaktivieren? Abrufe legen dann kein neues an, sondern scheitern."},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "protokoll", Title: "Vergebene Nummern", Collapsed: true, Relation: &metamodel.Relation{Object: "NumberRangeLog", ForeignKey: "interval_id",
				Columns: []string{"number", "reference", "user_id", "drawn_at"}}},
		},
	}
	logDef = metamodel.ObjectDefinition{
		Name: "NumberRangeLog", Title: "Vergebene Nummern", Icon: "icon-list", TitleField: "number",
		Fields: []metamodel.FieldDefinition{
			{Key: "number", Label: "Nummer", Type: metamodel.TypeText, Listable: true},
			{Key: "object", Label: "Objekt", Type: metamodel.TypeText, Listable: true},
			{Key: "company_code", Label: "Buchungskreis", Type: metamodel.TypeText, Listable: true},
			{Key: "range_key", Label: "Intervallschlüssel", Type: metamodel.TypeText, Listable: true},
			{Key: "year", Label: "Jahr", Type: metamodel.TypeNumber},
			{Key: "value", Label: "Laufende Nummer", Type: metamodel.TypeNumber},
			{Key: "interval_id", Label: "Intervall", Type: metamodel.TypeText},
			{Key: "reference", Label: "Referenz", Type: metamodel.TypeText, Listable: true},
			{Key: "user_id", Label: "Benutzer", Type: metamodel.TypeText, Listable: true},
			{Key: "request_id", Label: "Request", Type: metamodel.TypeText},
			{Key: "drawn_at", Label: "Vergeben am", Type: metamodel.TypeText, Listable: true},
		},
		Filters: []string{"object", "company_code", "range_key", "interval_id"},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		},
	}
	numrangeModule = metamodel.ModuleDefinition{
		Name: "numranges", Title: "Nummernkreise", Icon: "icon-hash",
		Description: "Nummernkreise der Module: Intervalle, Format, Überlauf, vergebene Nummern",
		Objects: []metamodel.ModuleObject{
			{Object: "NumberRange", Section: "Nummernkreise"},
			{Object: "NumberRangeObject", Section: "Nummernkreise"},
			{Object: "NumberRangeLog", Section: "Protokoll"},
		},
	}
)

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")
