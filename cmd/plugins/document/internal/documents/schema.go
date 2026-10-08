package documents

const schemaHCL = `
# Dokumentart (Katalog, für alle Buchungskreise)
table "document__type" {
  schema = schema.main
  column "code" { type = text }
  column "name" { type = text }
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

# Dokumentverweis an einem Datensatz (Object + fachlicher Schlüssel)
table "document__reference" {
  schema = schema.main
  column "id"          { type = text }
  column "entity_type" { type = text }
  column "entity_id"   { type = text }
  column "doc_type"    { type = text }
  column "title"       { type = text }
  column "doc_date" {
    type = date
    null = true
  }
  # Dateiname, Ablageort oder Link
  column "location"    { type = text }
  column "valid_from" {
    type = date
    null = true
  }
  column "valid_to" {
    type = date
    null = true
  }
  column "note" {
    type = text
    null = true
  }
  column "created_at"  { type = text }
  column "created_by" {
    type = text
    null = true
  }
  # entfernt (bleibt zur Nachvollziehbarkeit erhalten)
  column "removed_at" {
    type = text
    null = true
  }
  column "removed_by" {
    type = text
    null = true
  }
  primary_key { columns = [column.id] }
  index "document__reference_target" { columns = [column.entity_type, column.entity_id] }
  foreign_key "document__reference_type_fk" {
    columns     = [column.doc_type]
    ref_columns = [table.document__type.column.code]
  }
}
`

func row(kv ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

// Vorschlagswerte der Dokumentarten (DBSchema fügt nur fehlende ein).
var seedTypes = []map[string]any{
	row("code", "CONTRACT", "name", "Vertrag", "sort_order", 10),
	row("code", "AMENDMENT", "name", "Nachtrag", "sort_order", 20),
	row("code", "AGB", "name", "Allgemeine Geschäftsbedingungen", "sort_order", 30),
	row("code", "PRICES", "name", "Preisblatt / Leistungsbeschreibung", "sort_order", 40),
	row("code", "WITHDRAWAL", "name", "Widerrufsbelehrung", "sort_order", 50),
	row("code", "CONFIRMATION", "name", "Bestätigung", "sort_order", 60),
	row("code", "QUOTE", "name", "Angebot", "sort_order", 70),
	row("code", "INVOICE", "name", "Rechnung", "sort_order", 80),
	row("code", "NOTICE", "name", "Bescheid", "sort_order", 90),
	row("code", "STATEMENT", "name", "Abrechnung", "sort_order", 100),
	row("code", "CORRESPONDENCE", "name", "Schriftverkehr", "sort_order", 110),
	row("code", "ID_CARD", "name", "Personalausweis", "sort_order", 120),
	row("code", "OTHER", "name", "Sonstiges", "sort_order", 900),
}
