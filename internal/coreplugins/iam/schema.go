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
			{Key: "permissions", Label: "Berechtigungen (Object.Action[@Buchungskreis,…], eine pro Zeile, * als Platzhalter)", Type: metamodel.TypeTextarea, Listable: true, Editable: true},
		},
		Actions: crud("Rolle"),
	}
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
		{Object: "CompanyCode", Section: "Organisation"},
	},
}

// Übersetzungen des Moduls admin (de, en, zh-CN) – Schlüssel nach der
// Konvention von metamodel.WithKeys.
//
//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")
