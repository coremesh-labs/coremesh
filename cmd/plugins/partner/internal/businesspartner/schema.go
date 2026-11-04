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
  column "qr_template" {
    type = text
    null = true
  }
  primary_key { columns = [column.code] }
}

table "partner__comm_types" {
  schema = schema.main
  column "code"          { type = text }
  column "category_code" { type = text }
  column "description"   { type = text }
  column "vcard_type" {
    type = text
    null = true
  }
  column "is_main" {
    type    = boolean
    default = false
  }
  column "keywords" {
    type = text
    null = true
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code, column.valid_from] }
  foreign_key "partner__comm_types_category" {
    columns     = [column.category_code]
    ref_columns = [table.partner__comm_categories.column.code]
  }
}

table "partner__dial_codes" {
  schema = schema.main
  column "country"      { type = text }
  column "description"  { type = text }
  column "dial_code"    { type = text }
  column "trunk_prefix" {
    type = text
    null = true
  }
  column "zip_pattern" {
    type = text
    null = true
  }
  column "is_main" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.country] }
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

# Anreden (seit 0.11.0): Vorlage der Briefanrede je Art und Geschlecht
table "partner__salutations" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  column "letter_text" { type = text }
  column "person_type" {
    type = text
    null = true
  }
  column "gender" {
    type = text
    null = true
  }
  column "sort_order" {
    type    = bigint
    default = 0
  }
  column "is_active" {
    type    = boolean
    default = true
  }
  primary_key { columns = [column.code] }
}

table "partner__bp" {
  schema = schema.main
  column "id"   { type = text }
  column "type" { type = text }
  # seit 0.11.0: Anrede (Katalog partner__salutations), Geschlecht natürlicher Personen
  column "salutation_code" {
    type = text
    null = true
  }
  column "gender" {
    type = text
    null = true
  }
  column "name1" { type = text }
  column "name2" {
    type = text
    null = true
  }
  column "search_term" {
    type = text
    null = true
  }
  # Kürzel (eindeutig), z. B. im Dateinamen des DMS
  column "abbreviation" {
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
  # Partnergruppe (seit 0.10.0): Intervall des Nummernkreises BusinessPartner
  column "group_code" {
    type = text
    null = true
  }
  # GUID bis 0.9.0 (Migration auf BP-Nummern; BusinessPartnerService.resolve)
  column "legacy_id" {
    type = text
    null = true
  }
  primary_key { columns = [column.id, column.valid_from] }
  index "partner__bp_search" { columns = [column.search_term] }
  index "partner__bp_legacy" { columns = [column.legacy_id] }
}

table "partner__groups" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  column "range_key" {
    type = text
    null = true
  }
  column "is_default" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.code] }
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

// commKeywords (seit 0.17.0): Erkennungswörter der Kommunikationstypen für
// „Kontaktdaten aus Text übernehmen“ – Grundbestand, im Katalog änderbar.
var commKeywords = map[string]string{
	"PHONE_WORK": "Telefon, Tel, Tel.-Nr, Fon, Phone, Festnetz",
	"MOBILE":     "Mobil, Mobiltelefon, Handy, Mobile, Funk",
	"FAX_WORK":   "Telefax, Fax, Fax-Nr",
	"EMAIL_WORK": "E-Mail, Email, Mail, E-Mail-Adresse",
	"WEB":        "Internet, Web, Webseite, Homepage, URL",
}

// zipPatterns (seit 0.17.0): Aufbau der Postleitzahl je Land (regulärer
// Ausdruck ohne ^/$) – Grundbestand, im Katalog Ländervorwahlen änderbar.
var zipPatterns = map[string]string{
	"DE": `\d{5}`, "AT": `\d{4}`, "CH": `\d{4}`, "NL": `\d{4} ?[A-Z]{2}`, "BE": `\d{4}`, "LU": `\d{4}`,
	"FR": `\d{5}`, "IT": `\d{5}`, "ES": `\d{5}`, "DK": `\d{4}`, "PL": `\d{2}-\d{3}`, "CZ": `\d{3} ?\d{2}`,
	"GB": `[A-Z]{1,2}\d[A-Z\d]? ?\d[A-Z]{2}`, "US": `\d{5}(-\d{4})?`, "CN": `\d{6}`,
}

// seeds: Grundbestand der Kataloge. DBSchema fügt nur fehlende Zeilen ein;
// geänderte oder ergänzte Einträge im Bestand bleiben erhalten.
var seeds = []sdk.SchemaSeed{
	{Table: "partner__groups", Rows: []map[string]any{
		row("code", defaultGroup, "description", "Standard", "range_key", defaultGroup, "is_default", true),
	}},
	{Table: "partner__comm_categories", Rows: []map[string]any{
		row("code", "PHONE", "description", "Telefon", "qr_template", "tel:{number}"),
		row("code", "EMAIL", "description", "E-Mail"),
		row("code", "WEB", "description", "Webseite/URL"),
		row("code", "FAX", "description", "Telefax"),
	}},
	{Table: "partner__comm_types", Rows: []map[string]any{
		row("code", "EMAIL_WORK", "category_code", "EMAIL", "description", "E-Mail geschäftlich", "is_main", true, "vcard_type", "WORK", "keywords", commKeywords["EMAIL_WORK"], "valid_from", dateMin, "valid_to", dateMax),
		row("code", "PHONE_WORK", "category_code", "PHONE", "description", "Telefon geschäftlich", "is_main", true, "vcard_type", "WORK,VOICE", "keywords", commKeywords["PHONE_WORK"], "valid_from", dateMin, "valid_to", dateMax),
		row("code", "MOBILE", "category_code", "PHONE", "description", "Mobiltelefon", "is_main", false, "vcard_type", "CELL", "keywords", commKeywords["MOBILE"], "valid_from", dateMin, "valid_to", dateMax),
		// seit 0.17.0 im Grundbestand (vorher von Hand angelegt)
		row("code", "FAX_WORK", "category_code", "FAX", "description", "Fax geschäftlich", "is_main", true, "vcard_type", "WORK,FAX", "keywords", commKeywords["FAX_WORK"], "valid_from", dateMin, "valid_to", dateMax),
		row("code", "WEB", "category_code", "WEB", "description", "Webseite", "is_main", true, "keywords", commKeywords["WEB"], "valid_from", dateMin, "valid_to", dateMax),
	}},
	// seit 0.16.0: Ländervorwahlen (Nummern international, z. B. für tel:-Links und vCards)
	{Table: "partner__dial_codes", Rows: []map[string]any{
		row("country", "DE", "description", "Deutschland", "dial_code", "49", "trunk_prefix", "0", "is_main", true, "zip_pattern", zipPatterns["DE"]),
		row("country", "AT", "description", "Österreich", "dial_code", "43", "trunk_prefix", "0", "is_main", false, "zip_pattern", zipPatterns["AT"]),
		row("country", "CH", "description", "Schweiz", "dial_code", "41", "trunk_prefix", "0", "is_main", false, "zip_pattern", zipPatterns["CH"]),
		row("country", "NL", "description", "Niederlande", "dial_code", "31", "trunk_prefix", "0", "is_main", false, "zip_pattern", zipPatterns["NL"]),
		row("country", "BE", "description", "Belgien", "dial_code", "32", "trunk_prefix", "0", "is_main", false, "zip_pattern", zipPatterns["BE"]),
		row("country", "LU", "description", "Luxemburg", "dial_code", "352", "trunk_prefix", "", "is_main", false, "zip_pattern", zipPatterns["LU"]),
		row("country", "FR", "description", "Frankreich", "dial_code", "33", "trunk_prefix", "0", "is_main", false, "zip_pattern", zipPatterns["FR"]),
		row("country", "IT", "description", "Italien", "dial_code", "39", "trunk_prefix", "", "is_main", false, "zip_pattern", zipPatterns["IT"]),
		row("country", "ES", "description", "Spanien", "dial_code", "34", "trunk_prefix", "", "is_main", false, "zip_pattern", zipPatterns["ES"]),
		row("country", "DK", "description", "Dänemark", "dial_code", "45", "trunk_prefix", "", "is_main", false, "zip_pattern", zipPatterns["DK"]),
		row("country", "PL", "description", "Polen", "dial_code", "48", "trunk_prefix", "", "is_main", false, "zip_pattern", zipPatterns["PL"]),
		row("country", "CZ", "description", "Tschechien", "dial_code", "420", "trunk_prefix", "", "is_main", false, "zip_pattern", zipPatterns["CZ"]),
		row("country", "GB", "description", "Vereinigtes Königreich", "dial_code", "44", "trunk_prefix", "0", "is_main", false, "zip_pattern", zipPatterns["GB"]),
		row("country", "US", "description", "Vereinigte Staaten", "dial_code", "1", "trunk_prefix", "1", "is_main", false, "zip_pattern", zipPatterns["US"]),
		row("country", "CN", "description", "China", "dial_code", "86", "trunk_prefix", "0", "is_main", false, "zip_pattern", zipPatterns["CN"]),
	}},
	{Table: "partner__address_roles", Rows: []map[string]any{
		row("code", "MAIN", "description", "Hauptanschrift", "is_main", true, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "INVOICE", "description", "Rechnungsanschrift", "is_main", false, "valid_from", dateMin, "valid_to", dateMax),
	}},
	{Table: "partner__salutations", Rows: []map[string]any{
		row("code", "FRAU", "description", "Frau", "letter_text", "Sehr geehrte Frau {name1}", "person_type", "PERSON", "gender", "FEMALE", "sort_order", 10, "is_active", true),
		row("code", "HERR", "description", "Herr", "letter_text", "Sehr geehrter Herr {name1}", "person_type", "PERSON", "gender", "MALE", "sort_order", 20, "is_active", true),
		row("code", "NEUTRAL", "description", "Guten Tag", "letter_text", "Guten Tag {name2} {name1}", "person_type", "PERSON", "gender", "DIVERSE", "sort_order", 30, "is_active", true),
		row("code", "FIRMA", "description", "Firma", "letter_text", "Sehr geehrte Damen und Herren", "person_type", "ORGANIZATION", "sort_order", 40, "is_active", true),
	}},
	{Table: "partner__role_types", Rows: []map[string]any{
		row("code", "DEBITOR", "description", "Debitor", "is_debitor", true, "is_creditor", false, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "CREDITOR", "description", "Kreditor", "is_debitor", false, "is_creditor", true, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "TENANT", "description", "Mieter", "is_debitor", true, "is_creditor", false, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "LANDLORD", "description", "Vermieter", "is_debitor", false, "is_creditor", true, "valid_from", dateMin, "valid_to", dateMax),
		row("code", "AUTHORITY", "description", "Behörde / Amt", "is_debitor", false, "is_creditor", true, "valid_from", dateMin, "valid_to", dateMax),
	}},
}
