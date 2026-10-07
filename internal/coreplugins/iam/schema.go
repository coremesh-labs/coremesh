package iam

import (
	"embed"

	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/module"
)

// schemaHCL: Tabellen des Moduls iam (Präfix iam__), über DBSchema.Init.
// active ist integer (0/1) – portabel zwischen SQLite und PostgreSQL.
const schemaHCL = `
schema "main" {}

table "iam__users" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "username" {
    type = text
  }
  column "password_hash" {
    type = text
  }
  column "display_name" {
    type = text
    null = true
  }
  column "tenant_id" {
    type = text
    null = true
  }
  column "active" {
    type    = integer
    default = 1
  }
  # Sprache des Benutzers (BCP 47: de, en, zh-CN); leer = automatisch. Seit 0.4.0.
  column "locale" {
    type = text
    null = true
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
  index "iam__users_username" {
    unique  = true
    columns = [column.username]
  }
}

table "iam__roles" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "name" {
    type = text
  }
  column "description" {
    type = text
    null = true
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "iam__roles_name" {
    unique  = true
    columns = [column.name]
  }
}

table "iam__role_permissions" {
  schema = schema.main
  column "role_id" {
    type = text
  }
  column "object" {
    type = text
  }
  column "action" {
    type = text
  }
  # Buchungskreis, für den die Berechtigung gilt; "*" = alle. Seit 0.2.0 –
  # bestehende Zeilen erhalten beim Upgrade "*" und gelten weiter überall.
  column "company_code" {
    type    = text
    default = "*"
  }
  primary_key {
    columns = [column.role_id, column.object, column.action, column.company_code]
  }
  foreign_key "iam__role_permissions_role" {
    columns     = [column.role_id]
    ref_columns = [table.iam__roles.column.id]
    on_delete   = CASCADE
  }
}

# Berechtigungen seit 0.5.0: je Rolle und Object.Action eine Zeile, die
# Feldwerte in iam__role_auth_value. iam__role_permissions (bis 0.4.0) wird
# beim ersten Start einmalig übernommen und danach nicht mehr verwendet.
table "iam__role_auth" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "role_id" {
    type = text
  }
  column "object" {
    type = text
  }
  column "action" {
    type = text
  }
  # Buchungskreise, kommagetrennt; "*" = alle
  column "company_codes" {
    type    = text
    default = "*"
  }
  column "active" {
    type    = integer
    default = 1
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "iam__role_auth_role" {
    columns = [column.role_id]
  }
  foreign_key "iam__role_auth_role_fk" {
    columns     = [column.role_id]
    ref_columns = [table.iam__roles.column.id]
    on_delete   = CASCADE
  }
}

table "iam__role_auth_value" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "auth_id" {
    type = text
  }
  column "field" {
    type = text
  }
  # Einzelwert oder Muster (* ?); mit high ein Bereich low..high
  column "low" {
    type = text
  }
  column "high" {
    type = text
    null = true
  }
  column "active" {
    type    = integer
    default = 1
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "iam__role_auth_value_auth" {
    columns = [column.auth_id]
  }
  foreign_key "iam__role_auth_value_auth_fk" {
    columns     = [column.auth_id]
    ref_columns = [table.iam__role_auth.column.id]
    on_delete   = CASCADE
  }
}

# Darstellungsregeln seit 0.6.0: je Object Regeln (optional für Rollen), die
# bei erfüllten Bedingungen Felder ausblenden oder unänderbar machen.
table "iam__display_rule" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "object" {
    type = text
  }
  column "name" {
    type = text
  }
  # Rollennamen, kommagetrennt; leer = alle Benutzer
  column "roles" {
    type = text
    null = true
  }
  column "active" {
    type    = integer
    default = 1
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "iam__display_rule_object" {
    columns = [column.object]
  }
}

table "iam__display_rule_cond" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "rule_id" {
    type = text
  }
  column "field" {
    type = text
  }
  # erlaubte Werte, kommagetrennt
  column "field_values" {
    type = text
  }
  column "active" {
    type    = integer
    default = 1
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "iam__display_rule_cond_rule" {
    columns = [column.rule_id]
  }
  foreign_key "iam__display_rule_cond_rule_fk" {
    columns     = [column.rule_id]
    ref_columns = [table.iam__display_rule.column.id]
    on_delete   = CASCADE
  }
}

table "iam__display_rule_field" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "rule_id" {
    type = text
  }
  column "field" {
    type = text
  }
  # hidden | readonly
  column "mode" {
    type = text
  }
  column "active" {
    type    = integer
    default = 1
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "iam__display_rule_field_rule" {
    columns = [column.rule_id]
  }
  foreign_key "iam__display_rule_field_rule_fk" {
    columns     = [column.rule_id]
    ref_columns = [table.iam__display_rule.column.id]
    on_delete   = CASCADE
  }
}

table "iam__company_codes" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "description" {
    type = text
    null = true
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
}

table "iam__user_roles" {
  schema = schema.main
  column "user_id" {
    type = text
  }
  column "role_id" {
    type = text
  }
  primary_key {
    columns = [column.user_id, column.role_id]
  }
  index "iam__user_roles_role" {
    columns = [column.role_id]
  }
  foreign_key "iam__user_roles_user" {
    columns     = [column.user_id]
    ref_columns = [table.iam__users.column.id]
    on_delete   = CASCADE
  }
  foreign_key "iam__user_roles_role_fk" {
    columns     = [column.role_id]
    ref_columns = [table.iam__roles.column.id]
    on_delete   = CASCADE
  }
}
`

var crud = func(object string) []metamodel.ActionConfig {
	return []metamodel.ActionConfig{
		{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
		{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		{Name: "create", Kind: metamodel.KindCreate, Label: "Neu"},
		{Name: "update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
	}
}

// Metamodelle für die generische Oberfläche (Catalog.Describe).
var (
	userDef = metamodel.ObjectDefinition{
		Name: "User", Title: "Benutzer", Icon: "icon-user",
		Fields: []metamodel.FieldDefinition{
			{Key: "username", Label: "Benutzername", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
			{Key: "display_name", Label: "Name", Type: metamodel.TypeText, Listable: true, Editable: true},
			{Key: "tenant_id", Label: "Mandant", Type: metamodel.TypeText, Listable: true, Editable: true},
			{Key: "roles", Label: "Rollen (eine pro Zeile)", Type: metamodel.TypeTextarea, Listable: true, Editable: true},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
			{Key: "password", Label: "Passwort (bei Bearbeitung leer = unverändert)", Type: metamodel.TypePassword, Editable: true},
			{Key: "created_at", Label: "Angelegt", Type: metamodel.TypeText, Listable: true},
		},
		// Lebenszyklus status: Benutzer werden inaktiviert, nie gelöscht.
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Actions: append(crud("Benutzer"), metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate,
			Label: "Inaktivieren", Confirm: "Benutzer inaktivieren? Er kann sich danach nicht mehr anmelden."}),
	}
	roleDef = metamodel.ObjectDefinition{
		Name: "Role", Title: "Rollen", Icon: "icon-shield",
		Fields: []metamodel.FieldDefinition{
			{Key: "name", Label: "Name", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeText, Listable: true, Editable: true},
			// Übersicht in Textform; gepflegt wird im Abschnitt Berechtigungen.
			{Key: "permissions", Label: "Berechtigungen", Type: metamodel.TypeTextarea, Listable: true},
		},
		Actions: crud("Rolle"),
		Sections: []metamodel.SectionDefinition{
			{Key: "berechtigungen", Title: "Berechtigungen", Relation: &metamodel.Relation{Object: "RoleAuth", ForeignKey: "role_id",
				Columns: []string{"object", "action", "company_codes", "restrictions"}}},
		},
	}
	// RoleAuth: eine Zeile je Rolle und Object.Action. Object und Action kommen
	// aus dem Catalog (FormState), die Feldwerte stehen in RoleAuthValue.
	roleAuthDef = metamodel.ObjectDefinition{
		Name: "RoleAuth", Title: "Berechtigungen", Icon: "icon-shield", FormState: formStateAction,
		Fields: []metamodel.FieldDefinition{
			{Key: "role_id", Label: "Rolle", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "Role", ValueField: "id", LabelFields: []string{"name"}}},
			{Key: "object", Label: "Object", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true, Trigger: true},
			{Key: "action", Label: "Action", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true},
			{Key: "company_codes", Label: "Buchungskreise (kommagetrennt, * = alle)", Type: metamodel.TypeText, Listable: true, Editable: true},
			{Key: "restrictions", Label: "Feldwerte", Type: metamodel.TypeText, Listable: true},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
		},
		Filters:   []string{"role_id", "object"},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Actions: append(crud("Berechtigung"), metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate,
			Label: "Entziehen", Confirm: "Berechtigung entziehen?"}),
		Sections: []metamodel.SectionDefinition{
			{Key: "werte", Title: "Feldwerte (leer = alle Werte)", Relation: &metamodel.Relation{Object: "RoleAuthValue", ForeignKey: "auth_id",
				Columns: []string{"field", "low", "high"}}},
		},
	}
	// RoleAuthValue: erlaubter Wert eines Berechtigungsfelds. Die Felder kommen
	// aus metamodel.Authorization des Objects (FormState).
	roleAuthValueDef = metamodel.ObjectDefinition{
		Name: "RoleAuthValue", Title: "Berechtigungen – Feldwerte", Icon: "icon-list", FormState: formStateAction,
		Fields: []metamodel.FieldDefinition{
			{Key: "auth_id", Label: "Berechtigung", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "RoleAuth", ValueField: "id", LabelFields: []string{"object", "action"}}},
			{Key: "field", Label: "Feld", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true, Trigger: true},
			{Key: "low", Label: "Wert / von (* und ? als Platzhalter)", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
			{Key: "high", Label: "bis (leer = Einzelwert)", Type: metamodel.TypeText, Listable: true, Editable: true},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
		},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Actions: append(crud("Feldwert"), metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate,
			Label: "Entfernen", Confirm: "Feldwert entfernen?"}),
	}
	// Darstellungsregeln: je Object, optional für Rollen; Bedingungen auf
	// Feldwerte, Wirkung je Feld (ausblenden, unänderbar). Nur Darstellung –
	// geschützt wird über Berechtigungen.
	displayRuleDef = metamodel.ObjectDefinition{
		Name: "DisplayRule", Title: "Darstellungsregeln", Icon: "icon-eye", FormState: formStateAction, TitleField: "name",
		Fields: []metamodel.FieldDefinition{
			{Key: "name", Label: "Bezeichnung", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
			{Key: "object", Label: "Object", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true},
			{Key: "roles", Label: "Nur für Rollen (kommagetrennt, leer = alle)", Type: metamodel.TypeText, Listable: true, Editable: true},
			{Key: "summary", Label: "Regel", Type: metamodel.TypeText, Listable: true},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
		},
		Filters:   []string{"object"},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Actions: append(crud("Darstellungsregel"), metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate,
			Label: "Inaktivieren", Confirm: "Darstellungsregel inaktivieren?"}),
		Sections: []metamodel.SectionDefinition{
			{Key: "bedingungen", Title: "Bedingungen (alle müssen zutreffen; keine = immer)", Relation: &metamodel.Relation{Object: "DisplayRuleCondition", ForeignKey: "rule_id",
				Columns: []string{"field", "field_values"}}},
			{Key: "felder", Title: "Felder", Relation: &metamodel.Relation{Object: "DisplayRuleField", ForeignKey: "rule_id",
				Columns: []string{"field", "mode"}}},
		},
	}
	displayRuleCondDef = metamodel.ObjectDefinition{
		Name: "DisplayRuleCondition", Title: "Darstellungsregeln – Bedingungen", Icon: "icon-list", FormState: formStateAction,
		Fields: []metamodel.FieldDefinition{
			{Key: "rule_id", Label: "Regel", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "DisplayRule", ValueField: "id", LabelFields: []string{"name"}}},
			{Key: "field", Label: "Feld", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true, Trigger: true},
			{Key: "field_values", Label: "Werte (kommagetrennt, einer muss zutreffen)", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
		},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Actions: append(crud("Bedingung"), metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate,
			Label: "Entfernen", Confirm: "Bedingung entfernen?"}),
	}
	displayRuleFieldDef = metamodel.ObjectDefinition{
		Name: "DisplayRuleField", Title: "Darstellungsregeln – Felder", Icon: "icon-list", FormState: formStateAction,
		Fields: []metamodel.FieldDefinition{
			{Key: "rule_id", Label: "Regel", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "DisplayRule", ValueField: "id", LabelFields: []string{"name"}}},
			{Key: "field", Label: "Feld", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true},
			{Key: "mode", Label: "Darstellung", Type: metamodel.TypeSelect, Required: true, Listable: true, Editable: true, Trigger: true, Options: displayModes},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
		},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Actions: append(crud("Feld"), metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate,
			Label: "Entfernen", Confirm: "Feld aus der Regel entfernen?"}),
	}
	displayModes   = []metamodel.Option{{Value: modeHidden, Label: "ausblenden"}, {Value: modeReadonly, Label: "unänderbar"}}
	companyCodeDef = metamodel.ObjectDefinition{
		Name: "CompanyCode", Title: "Buchungskreise", Icon: "icon-building",
		Fields: []metamodel.FieldDefinition{
			{Key: "code", Label: "Buchungskreis", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeText, Listable: true, Editable: true},
		},
		Actions: crud("Buchungskreis"),
	}
)

// adminModule bündelt die Benutzerverwaltung als fachliches Modul
// (Oberfläche: /m/admin).
var adminModule = metamodel.ModuleDefinition{
	Name: "admin", Title: "Administration", Icon: "icon-shield",
	Description: "Benutzer, Rollen und Buchungskreise",
	Objects: []metamodel.ModuleObject{
		{Object: "User", Section: "Zugriff"},
		{Object: "Role", Section: "Zugriff"},
		{Object: "RoleAuth", Section: "Zugriff"},
		{Object: "RoleAuthValue", Section: "Zugriff"},
		{Object: "DisplayRule", Section: "Darstellung"},
		{Object: "DisplayRuleCondition", Section: "Darstellung"},
		{Object: "DisplayRuleField", Section: "Darstellung"},
		{Object: "CompanyCode", Section: "Organisation"},
	},
}

// Übersetzungen des Moduls admin (de, en, zh-CN) – Schlüssel nach der
// Konvention von metamodel.WithKeys.
//
//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")
