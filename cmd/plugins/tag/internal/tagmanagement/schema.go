package tagmanagement

// schemaHCL: Datenmodell des TagManagements (Plugin tag, Präfix tag__, bei
// PostgreSQL-Schema-Isolation im Schema mod_tag). Nur table-Blöcke; den
// schema-Block ergänzt module.Plugin.
//
//	tag_types ──< value_options                    (Auswahlwerte, Zeitscheibe)
//	    │
//	    ├──< tag_set_items >── tag_sets            (Zusammensetzung, Zeitscheibe)
//	    ├──< tag_set_rules  >── tag_sets           (REQUIRES / EXCLUDES / SHOW_IF, Zeitscheibe)
//	    │                       tag_sets ──< tag_set_assignments (Objekttyp + Buchungskreis ↔ Tag Set, Zeitscheibe)
//	    └──< tag_assignments                       (Wert je Objekt und Buchungskreis, polymorph, Zeitscheibe)
//
// Regeln wie im ganzen System: valid_from ist bei jeder Zeitscheibe Teil des
// Primärschlüssels; Fremdschlüssel nur auf Tabellen ohne Zeitscheibe
// (tag_types). Verweise auf tag_sets prüft die Engine zeitbezogen.
//
// Typen sind portabel (SQLite/PostgreSQL): text, bigint, boolean, date.
// Beträge stehen als Dezimaltext in value_amount (exakt, ohne Rundung durch
// Gleitkommazahlen), Zeitstempel als RFC 3339 in UTC.
const schemaHCL = `
table "tag__tag_types" {
  schema = schema.main
  column "code"            { type = text }
  column "name"            { type = text }
  column "translation_key" {
    type = text
    null = true
  }
  column "description" {
    type = text
    null = true
  }
  column "data_type"  { type = text }
  column "value_mode" { type = text }
  # REFERENCE: Object des Ziels (z. B. RentalObject); sonst leer
  column "ref_object" {
    type = text
    null = true
  }
  column "status" {
    type    = text
    default = "ACTIVE"
  }
  # seit 0.4.0: Prüfmuster (regulärer Ausdruck, ganzer Wert) mit Hinweis; geschützt =
  # Werte nur mit TagType.readValue/changeValue (einschränkbar nach Tag-Code)
  column "pattern" {
    type = text
    null = true
  }
  column "pattern_hint" {
    type = text
    null = true
  }
  column "protected" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.code] }
}

table "tag__value_options" {
  schema = schema.main
  column "tag_type_code"   { type = text }
  column "code"            { type = text }
  column "label"           { type = text }
  column "translation_key" {
    type = text
    null = true
  }
  column "sort_order" {
    type    = bigint
    default = 0
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.tag_type_code, column.code, column.valid_from] }
  foreign_key "tag__value_options_type" {
    columns     = [column.tag_type_code]
    ref_columns = [table.tag__tag_types.column.code]
  }
}

table "tag__tag_sets" {
  schema = schema.main
  column "code"            { type = text }
  column "name"            { type = text }
  column "translation_key" {
    type = text
    null = true
  }
  column "description" {
    type = text
    null = true
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code, column.valid_from] }
}

table "tag__tag_set_items" {
  schema = schema.main
  column "tag_set_code"  { type = text }
  column "tag_type_code" { type = text }
  column "mandatory" {
    type    = boolean
    default = false
  }
  column "sort_order" {
    type    = bigint
    default = 0
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.tag_set_code, column.tag_type_code, column.valid_from] }
  foreign_key "tag__tag_set_items_type" {
    columns     = [column.tag_type_code]
    ref_columns = [table.tag__tag_types.column.code]
  }
}

table "tag__tag_set_rules" {
  schema = schema.main
  column "id"           { type = text }
  column "tag_set_code" { type = text }
  column "rule_type"    { type = text }
  column "source_tag"   { type = text }
  column "target_tag"   { type = text }
  column "condition_value" {
    type = text
    null = true
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.id, column.valid_from] }
  index "tag__tag_set_rules_set" { columns = [column.tag_set_code] }
  foreign_key "tag__tag_set_rules_source" {
    columns     = [column.source_tag]
    ref_columns = [table.tag__tag_types.column.code]
  }
  foreign_key "tag__tag_set_rules_target" {
    columns     = [column.target_tag]
    ref_columns = [table.tag__tag_types.column.code]
  }
}

table "tag__tag_set_assignments" {
  schema = schema.main
  column "entity_type"  { type = text }
  # Buchungskreis (iam CompanyCode) oder "*" = alle Buchungskreise
  column "company_code" {
    type    = text
    default = "*"
  }
  column "tag_set_code" { type = text }
  # Bedingung: Feld des Objekts und erlaubte Werte (kommagetrennt), z. B.
  # contract_type = "RENT,LEASE". Leer = alle Datensätze des Objekttyps.
  column "condition_field" {
    type = text
    null = true
  }
  column "condition_values" {
    type = text
    null = true
  }
  # seit 0.4.0: zweite Bedingung (UND)
  column "condition_field_2" {
    type = text
    null = true
  }
  column "condition_values_2" {
    type = text
    null = true
  }
  column "valid_from"   { type = date }
  column "valid_to"     { type = date }
  primary_key { columns = [column.entity_type, column.company_code, column.tag_set_code, column.valid_from] }
}

table "tag__tag_assignments" {
  schema = schema.main
  column "id"                 { type = text }
  column "target_entity_type" { type = text }
  column "target_entity_id"   { type = text }
  # Buchungskreis des Werts; "*" = gilt in allen Buchungskreisen (Tag aus einem
  # Tag Set, das dem Objekttyp für alle Buchungskreise zugewiesen ist)
  column "company_code" {
    type    = text
    default = "*"
  }
  column "tag_type_code"      { type = text }
  column "value_string" {
    type = text
    null = true
  }
  column "value_integer" {
    type = bigint
    null = true
  }
  column "value_amount" {
    type = text
    null = true
  }
  column "value_currency" {
    type = text
    null = true
  }
  column "value_date" {
    type = date
    null = true
  }
  column "value_timestamp" {
    type = text
    null = true
  }
  column "option_code" {
    type = text
    null = true
  }
  column "value_ref" {
    type = text
    null = true
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  column "changed_at" { type = text }
  column "changed_by" {
    type = text
    null = true
  }
  primary_key { columns = [column.id, column.valid_from] }
  index "tag__tag_assignments_target" { columns = [column.target_entity_type, column.target_entity_id, column.company_code] }
  index "tag__tag_assignments_tag"    { columns = [column.tag_type_code] }
  foreign_key "tag__tag_assignments_type" {
    columns     = [column.tag_type_code]
    ref_columns = [table.tag__tag_types.column.code]
  }
}
`
