package script

// opt: Spalte darf leer sein.
const opt = `
    null = true`

// schemaHCL: Soll-Schema (Atlas-HCL) der Skripte und Berichte. Tabellen mit dem Pflicht-Präfix script__.
const schemaHCL = `
# Skript (JavaScript, goja)
table "script__script" {
  schema = schema.main
  column "code" { type = text }
  column "name" { type = text }
  column "description" {
    type = text` + opt + `
  }
  column "source" { type = text }
  # erlaubte Aufrufe Object.action, je Zeile ein Muster mit * (z. B. *.list)
  column "allowed_calls" {
    type = text` + opt + `
  }
  column "timeout_sec" {
    type    = bigint
    default = 30
  }
  # Protokoll: ALL, ERRORS, NONE
  column "log_runs" {
    type    = text
    default = "ALL"
  }
  column "retention_days" {
    type    = bigint
    default = 30
  }
  column "is_active" {
    type    = boolean
    default = true
  }
  primary_key { columns = [column.code] }
}

# Eingabeparameter eines Skripts (Maske „Ausführen …“)
table "script__parameter" {
  schema = schema.main
  column "script" { type = text }
  column "name" { type = text }
  column "position" {
    type    = bigint
    default = 10
  }
  column "label" { type = text }
  # TEXT, NUMBER, DATE, BOOLEAN, SELECT
  column "type" {
    type    = text
    default = "TEXT"
  }
  column "required" {
    type    = boolean
    default = false
  }
  column "default_value" {
    type = text` + opt + `
  }
  # Auswahlwerte (SELECT), je Zeile "WERT=Text"
  column "options" {
    type = text` + opt + `
  }
  # Auswahl aus einem Object (z. B. CompanyCode, Feld code)
  column "lookup_object" {
    type = text` + opt + `
  }
  column "lookup_value" {
    type = text` + opt + `
  }
  column "lookup_label" {
    type = text` + opt + `
  }
  primary_key { columns = [column.script, column.name] }
  foreign_key "script__parameter_script_fk" {
    columns     = [column.script]
    ref_columns = [table.script__script.column.code]
  }
}

# Bindung: Skript als Event-Handler (EVENT) oder Hook-Abonnent (HOOK)
table "script__binding" {
  schema = schema.main
  column "id" { type = text }
  column "script" { type = text }
  column "kind" { type = text }
  column "event_object" {
    type = text` + opt + `
  }
  column "event_action" {
    type = text` + opt + `
  }
  column "company_code" {
    type = text` + opt + `
  }
  column "hook" {
    type = text` + opt + `
  }
  column "phase" {
    type = text` + opt + `
  }
  column "priority" {
    type    = bigint
    default = 100
  }
  column "description" {
    type = text` + opt + `
  }
  column "is_active" {
    type    = boolean
    default = true
  }
  primary_key { columns = [column.id] }
  foreign_key "script__binding_script_fk" {
    columns     = [column.script]
    ref_columns = [table.script__script.column.code]
  }
  index "script__binding_script" { columns = [column.script] }
}

# Bericht: Datenquelle (Object oder Skript), Spalten, Gruppen, Summen, Vorlage
table "script__report" {
  schema = schema.main
  column "code" { type = text }
  column "name" { type = text }
  column "description" {
    type = text` + opt + `
  }
  # OBJECT oder SCRIPT
  column "source_kind" {
    type    = text
    default = "OBJECT"
  }
  column "source_object" {
    type = text` + opt + `
  }
  column "source_action" {
    type = text` + opt + `
  }
  # Payload als JSON mit Platzhaltern {parameter}
  column "source_payload" {
    type = text` + opt + `
  }
  # als Datenstrom lesen (Read) statt Call
  column "stream" {
    type    = boolean
    default = false
  }
  # Pfad der Zeilen in der Antwort (z. B. items); leer = items bzw. rows
  column "result_path" {
    type = text` + opt + `
  }
  column "source_script" {
    type = text` + opt + `
  }
  # Felder, Komma-Liste
  column "group_by" {
    type = text` + opt + `
  }
  # Felder, Komma-Liste; -feld = absteigend
  column "sort_by" {
    type = text` + opt + `
  }
  column "row_limit" {
    type = bigint` + opt + `
  }
  column "totals" {
    type    = boolean
    default = false
  }
  column "template" {
    type = text` + opt + `
  }
  column "template_language" {
    type = text` + opt + `
  }
  # Parameter mit dem Buchungskreis für die Vorlage
  column "company_param" {
    type = text` + opt + `
  }
  # Feld der Abschnitte (Zwischenüberschrift und -summe je Wert), seit 0.2.0
  column "section_by" {
    type = text` + opt + `
  }
  # Untertitel mit Platzhaltern {parameter}, seit 0.2.0
  column "subtitle" {
    type = text` + opt + `
  }
  column "is_active" {
    type    = boolean
    default = true
  }
  primary_key { columns = [column.code] }
}

table "script__report_column" {
  schema = schema.main
  column "report" { type = text }
  column "position" { type = bigint }
  column "field" { type = text }
  column "label" {
    type = text` + opt + `
  }
  # NONE, SUM, COUNT, MIN, MAX, AVG
  column "aggregate" {
    type    = text
    default = "NONE"
  }
  # TEXT, NUMBER, INTEGER, MONEY, MONEY_MINOR, DATE
  column "format" {
    type    = text
    default = "TEXT"
  }
  # relative Breite (PDF), seit 0.2.0
  column "width" {
    type = double` + opt + `
  }
  primary_key { columns = [column.report, column.position] }
  foreign_key "script__report_column_report_fk" {
    columns     = [column.report]
    ref_columns = [table.script__report.column.code]
  }
}

table "script__report_parameter" {
  schema = schema.main
  column "report" { type = text }
  column "name" { type = text }
  column "position" {
    type    = bigint
    default = 10
  }
  column "label" { type = text }
  column "type" {
    type    = text
    default = "TEXT"
  }
  column "required" {
    type    = boolean
    default = false
  }
  column "default_value" {
    type = text` + opt + `
  }
  column "options" {
    type = text` + opt + `
  }
  column "lookup_object" {
    type = text` + opt + `
  }
  column "lookup_value" {
    type = text` + opt + `
  }
  column "lookup_label" {
    type = text` + opt + `
  }
  primary_key { columns = [column.report, column.name] }
  foreign_key "script__report_parameter_report_fk" {
    columns     = [column.report]
    ref_columns = [table.script__report.column.code]
  }
}

# Protokoll der Läufe (Maske, Event, Hook, Bericht)
table "script__run" {
  schema = schema.main
  column "run_id" { type = text }
  column "script" {
    type = text` + opt + `
  }
  column "report" {
    type = text` + opt + `
  }
  # RUN, EVENT, HOOK, REPORT
  column "trigger" { type = text }
  column "detail" {
    type = text` + opt + `
  }
  # DONE, ERROR
  column "status" { type = text }
  column "message" {
    type = text` + opt + `
  }
  column "log" {
    type = text` + opt + `
  }
  column "result" {
    type = text` + opt + `
  }
  column "duration_ms" {
    type    = bigint
    default = 0
  }
  column "keep" {
    type    = boolean
    default = false
  }
  column "created_at" { type = text }
  column "created_by" {
    type = text` + opt + `
  }
  column "expires_on" { type = text }
  primary_key { columns = [column.run_id] }
  index "script__run_script" { columns = [column.script, column.created_at] }
}
`
