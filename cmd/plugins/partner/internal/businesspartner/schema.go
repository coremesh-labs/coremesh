package businesspartner

import "github.com/coremesh-labs/coremesh/pkg/sdk"

// schemaHCL: alle Tabellen des Moduls businesspartner (Präfix partner__ des
// Plugins, bei PostgreSQL-Schema-Isolation im Schema mod_partner). Nur
// table-Blöcke: Den schema-Block ergänzt module.Plugin.
//
// Typen sind portabel zwischen SQLite und PostgreSQL: text, boolean, date.
// Zeitscheiben: valid_from/valid_to (Datum), Standard valid_to = 9999-12-31.
//
// Regel: Bei jeder Tabelle mit Zeitscheibe ist valid_from Teil des Primärschlüssels.
// Ein fachlicher Schlüssel (id, code) kann so mehrere Zeitscheiben haben. Verweise
// auf solche Tabellen sind deshalb keine Fremdschlüssel der Datenbank (sie zeigten
// auf keine eindeutige Zeile); die Engine prüft sie zeitbezogen: Das Ziel muss am
// Stichtag (valid_from des verweisenden Datensatzes) gültig sein.
const schemaHCL = `
# --- Kataloge (Stammdaten) ---------------------------------------------------

table "partner__address_roles" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  column "is_main" {
    type    = boolean
    default = false
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code, column.valid_from] }
}

table "partner__comm_categories" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  primary_key { columns = [column.code] }
}

table "partner__comm_types" {
  schema = schema.main
  column "code"          { type = text }
  column "category_code" { type = text }
  column "description"   { type = text }
  column "is_main" {
    type    = boolean
    default = false
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code, column.valid_from] }
  foreign_key "partner__comm_types_category" {
    columns     = [column.category_code]
    ref_columns = [table.partner__comm_categories.column.code]
  }
}

table "partner__role_types" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  column "is_debitor" {
    type    = boolean
    default = false
  }
  column "is_creditor" {
    type    = boolean
    default = false
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code, column.valid_from] }
}

# --- Geschäftspartner und Beziehungen -----------------------------------------

table "partner__bp" {
  schema = schema.main
  column "id"   { type = text }
  column "type" { type = text }
  column "name1" { type = text }
  column "name2" {
    type = text
    null = true
  }
  column "search_term" {
    type = text
    null = true
  }
  column "is_blocked" {
    type    = boolean
    default = false
  }
  # Bis 0.4.0 Status-Flag (Lebenszyklus status). Seit 0.5.0 hat der Partner eine
  # Zeitscheibe; die Spalte bleibt, weil DBSchema DROP COLUMN ablehnt (ungenutzt).
  column "is_active" {
    type    = boolean
    default = true
  }
  # Zeitscheibe seit 0.5.0. Bestehende Partner erhalten 1900-01-01 bis 9999-12-31.
  column "valid_from" {
    type    = date
    default = "1900-01-01"
  }
  column "valid_to" {
    type    = date
    default = "9999-12-31"
  }
  primary_key { columns = [column.id, column.valid_from] }
  index "partner__bp_search" { columns = [column.search_term] }
}

table "partner__roles" {
  schema = schema.main
  column "bp_id"      { type = text }
  column "role_code"  { type = text }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.bp_id, column.role_code, column.valid_from] }
}

table "partner__addresses" {
  schema = schema.main
  column "id"     { type = text }
  column "street" { type = text }
  column "house_no" {
    type = text
    null = true
  }
  column "zip_code" { type = text }
  column "city"     { type = text }
  column "country"  { type = text }
  primary_key { columns = [column.id] }
}

table "partner__bp_addresses" {
  schema = schema.main
  column "id"                { type = text }
  column "bp_id"             { type = text }
  column "address_id"        { type = text }
  column "address_role_code" { type = text }
  column "valid_from"        { type = date }
  column "valid_to"          { type = date }
  column "is_default" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.id, column.valid_from] }
  index "partner__bp_addresses_bp" { columns = [column.bp_id] }
  foreign_key "partner__bp_addresses_address" {
    columns     = [column.address_id]
    ref_columns = [table.partner__addresses.column.id]
  }
}

table "partner__contacts" {
  schema = schema.main
  column "id"             { type = text }
  column "bp_id"          { type = text }
  column "comm_type_code" { type = text }
  column "value"          { type = text }
  column "valid_from"     { type = date }
  column "valid_to"       { type = date }
  column "is_default" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.id, column.valid_from] }
  index "partner__contacts_bp" { columns = [column.bp_id] }
}

table "partner__bank_details" {
  schema = schema.main
  column "id"    { type = text }
  column "bp_id" { type = text }
  column "iban"  { type = text }
  column "bic" {
    type = text
    null = true
  }
  column "bank_name" {
    type = text
    null = true
  }
  column "account_holder" {
    type = text
    null = true
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  column "is_default" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.id, column.valid_from] }
  index "partner__bank_details_bp" { columns = [column.bp_id] }
}

table "partner__company_codes" {
  schema = schema.main
  column "bp_id"        { type = text }
  column "company_code" { type = text }
  column "role_code"    { type = text }
  column "reconciliation_account" {
    type = text
    null = true
  }
  column "payment_terms" {
    type = text
    null = true
  }
  column "dunning_block" {
    type    = boolean
    default = false
  }
  column "posting_block" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.bp_id, column.company_code, column.role_code] }
}
`

const (
	dateMin = "1900-01-01"
	dateMax = "9999-12-31" // Standard für valid_to
)

func row(kv ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

// seeds: Grundbestand der Kataloge. DBSchema fügt nur fehlende Zeilen ein;
// geänderte oder ergänzte Einträge im Bestand bleiben erhalten.
var seeds = []sdk.SchemaSeed{
	{Table: "partner__comm_categories", Rows: []map[string]any{
		row("code", "PHONE", "description", "Telefon"),
		row("code", "EMAIL", "description", "E-Mail"),
		row("code", "WEB", "description", "Webseite/URL"),
		row("code", "FAX", "description", "Telefax"),
	}},
	{Table: "partner__comm_types", Rows: []map[string]any{
		row("code", "EMAIL_WORK", "category_code", "EMAIL", "description", "E-Mail geschäftlich", "is_main", true, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "PHONE_WORK", "category_code", "PHONE", "description", "Telefon geschäftlich", "is_main", true, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "MOBILE", "category_code", "PHONE", "description", "Mobiltelefon", "is_main", false, "valid_from", dateMin, "valid_to", dateMax),
	}},
	{Table: "partner__address_roles", Rows: []map[string]any{
		row("code", "MAIN", "description", "Hauptanschrift", "is_main", true, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "INVOICE", "description", "Rechnungsanschrift", "is_main", false, "valid_from", dateMin, "valid_to", dateMax),
	}},
	{Table: "partner__role_types", Rows: []map[string]any{
		row("code", "DEBITOR", "description", "Debitor", "is_debitor", true, "is_creditor", false, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "CREDITOR", "description", "Kreditor", "is_debitor", false, "is_creditor", true, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "TENANT", "description", "Mieter", "is_debitor", true, "is_creditor", false, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "LANDLORD", "description", "Vermieter", "is_debitor", false, "is_creditor", true, "valid_from", dateMin, "valid_to", dateMax),
	}},
}
