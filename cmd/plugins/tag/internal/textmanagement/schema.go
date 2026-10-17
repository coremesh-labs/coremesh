package textmanagement

// schemaHCL: Datenmodell der Texte (Plugin tag, Präfix tag__text…). Mit den
// Tags hat es nichts gemeinsam außer dem Plugin.
//
//	text_category ──< text_assignment   (Kategorie je Object und Buchungskreis, Pflicht/optional, Bedingung)
//	      │
//	      └──< text >── text_language   (Text je Datensatz, Kategorie, Position und Sprache)
const schemaHCL = `
# Textkategorie: einmal (SINGLE, Position 1) oder mehrfach (MULTIPLE) je Datensatz
table "tag__text_category" {
  schema = schema.main
  column "code"        { type = text }
  column "name"        { type = text }
  column "cardinality" { type = text }
  column "description" {
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

# Sprachschlüssel (z. B. de, en, zh-CN)
table "tag__text_language" {
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

# Zuordnung: Kategorie an einem Object (z. B. Contract) in einem Buchungskreis oder
# allen (*), Pflicht oder optional, optional nur für Datensätze mit bestimmten Feldwerten
table "tag__text_assignment" {
  schema = schema.main
  column "entity_type"  { type = text }
  column "company_code" {
    type    = text
    default = "*"
  }
  column "category"     { type = text }
  column "mandatory" {
    type    = boolean
    default = false
  }
  column "condition_field" {
    type = text
    null = true
  }
  column "condition_values" {
    type = text
    null = true
  }
  column "sort_order" {
    type    = bigint
    default = 0
  }
  primary_key { columns = [column.entity_type, column.company_code, column.category] }
  foreign_key "tag__text_assignment_category" {
    columns     = [column.category]
    ref_columns = [table.tag__text_category.column.code]
  }
}

# Text an einem Datensatz (Object + fachlicher Schlüssel)
table "tag__text" {
  schema = schema.main
  column "id"           { type = text }
  column "entity_type"  { type = text }
  column "entity_id"    { type = text }
  column "company_code" {
    type = text
    null = true
  }
  column "category"     { type = text }
  column "position"     { type = bigint }
  column "language"     { type = text }
  column "content"      { type = text }
  column "created_at"   { type = text }
  column "created_by" {
    type = text
    null = true
  }
  column "changed_at"   { type = text }
  column "changed_by" {
    type = text
    null = true
  }
  primary_key { columns = [column.id] }
  index "tag__text_target" {
    unique  = true
    columns = [column.entity_type, column.entity_id, column.category, column.position, column.language]
  }
  foreign_key "tag__text_category_fk" {
    columns     = [column.category]
    ref_columns = [table.tag__text_category.column.code]
  }
  foreign_key "tag__text_language_fk" {
    columns     = [column.language]
    ref_columns = [table.tag__text_language.column.code]
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

// Vorschlagswerte (DBSchema fügt nur fehlende ein). Zuordnungen zu Objects legt
// der Anwender an – das Modul kennt die Objects anderer Module nicht.
var seedCategories = []map[string]any{
	row("code", "NOTE", "name", "Anmerkung", "cardinality", "MULTIPLE", "sort_order", 10),
	row("code", "DESCRIPTION", "name", "Beschreibung", "cardinality", "SINGLE", "sort_order", 20),
}

var seedLanguages = []map[string]any{
	row("code", "de", "name", "Deutsch", "sort_order", 10),
	row("code", "en", "name", "Englisch", "sort_order", 20),
}
