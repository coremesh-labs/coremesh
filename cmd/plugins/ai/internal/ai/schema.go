package ai

// opt: Spalte darf leer sein.
const opt = `
    null = true`

// schemaHCL: Soll-Schema (Atlas-HCL) der KI-Anbindung. Tabellen mit dem Pflicht-Präfix ai__.
const schemaHCL = `
# KI-Dienst (OpenAI-kompatible Schnittstelle /chat/completions)
table "ai__provider" {
  schema = schema.main
  column "code" { type = text }
  column "name" { type = text }
  # z. B. https://generativelanguage.googleapis.com/v1beta/openai/
  column "base_url" { type = text }
  # BEARER (Authorization: Bearer …), HEADER (eigener Header), QUERY (Parameter), NONE
  column "auth_method" {
    type    = text
    default = "BEARER"
  }
  # Name des Headers bzw. Parameters bei HEADER/QUERY
  column "auth_name" {
    type = text` + opt + `
  }
  # Umgebungsvariable mit dem Schlüssel (statt des gespeicherten)
  column "api_key_env" {
    type = text` + opt + `
  }
  column "model" { type = text }
  # weitere Header, je Zeile "Name: Wert"
  column "extra_headers" {
    type = text` + opt + `
  }
  # ohne JSON-Schema (response_format) – für Dienste, die es nicht kennen
  column "plain_json" {
    type    = boolean
    default = false
  }
  column "timeout_seconds" {
    type    = bigint
    default = 90
  }
  column "is_active" {
    type    = boolean
    default = true
  }
  primary_key { columns = [column.code] }
}

# Schlüssel der Dienste – getrennt, ohne Leseschnittstelle (nur das Plugin liest ihn)
table "ai__provider_secret" {
  schema = schema.main
  column "provider" { type = text }
  column "api_key" { type = text }
  column "changed_at" { type = text }
  primary_key { columns = [column.provider] }
}

# Anfrage: OBJECT (Strukturen aus dem Metamodell füllen) oder TEXT
table "ai__request" {
  schema = schema.main
  column "code" { type = text }
  column "name" { type = text }
  column "kind" { type = text }
  # leer = erster aktiver Dienst
  column "provider" {
    type = text` + opt + `
  }
  column "model" {
    type = text` + opt + `
  }
  column "system_text" {
    type = text` + opt + `
  }
  # die eigentliche Aufgabe
  column "task_text" { type = text }
  # weitere Textzeilen (Möglichkeiten aus dem Kontext)
  column "extra_text" {
    type = text` + opt + `
  }
  # z. B. 0.2 (leer = Vorgabe des Dienstes)
  column "temperature" {
    type = text` + opt + `
  }
  # Aufbewahrung des Protokolls in Tagen
  column "retention_days" {
    type    = bigint
    default = 1
  }
  # Verweise mit höchstens so vielen Werten werden als Auswahl mitgegeben
  column "max_values" {
    type    = bigint
    default = 100
  }
  column "sample_input" {
    type = text` + opt + `
  }
  column "description" {
    type = text` + opt + `
  }
  column "is_active" {
    type    = boolean
    default = true
  }
  primary_key { columns = [column.code] }
}

# Objekte einer Objekt-Anfrage: Wurzel, Unter-Objekte (Relation der Wurzel) und
# eingebettete Objekte (Verweisfeld des übergeordneten, z. B. Adresse einer Partner-Adresse)
table "ai__request_object" {
  schema = schema.main
  column "request" { type = text }
  column "line_no" { type = bigint }
  # Name in der JSON-Struktur, z. B. partner, addresses
  column "name" { type = text }
  column "object" { type = text }
  column "parent" {
    type = text` + opt + `
  }
  # Abschnitt der Relation am übergeordneten Object (Aggregat), z. B. adressen
  column "relation" {
    type = text` + opt + `
  }
  # Verweisfeld des übergeordneten Objects (eingebettet), z. B. address_id
  column "via_field" {
    type = text` + opt + `
  }
  column "multiple" {
    type    = boolean
    default = false
  }
  # Felder (Komma-Liste; leer = alle bearbeitbaren)
  column "fields" {
    type = text` + opt + `
  }
  column "exclude" {
    type = text` + opt + `
  }
  # Hinweis an die KI zu diesem Objekt
  column "hint" {
    type = text` + opt + `
  }
  primary_key { columns = [column.request, column.line_no] }
}

# Anfrageprotokoll: Eingang, Anfrage, Antwort, Ergebnis; Verfall nach den Tagen der Anfrage
table "ai__log" {
  schema = schema.main
  column "log_id" { type = text }
  column "request" { type = text }
  column "kind" { type = text }
  column "company_code" {
    type = text` + opt + `
  }
  column "provider" {
    type = text` + opt + `
  }
  column "model" {
    type = text` + opt + `
  }
  # DONE, ERROR
  column "status" { type = text }
  column "message" {
    type = text` + opt + `
  }
  # Eingang (Text) bzw. Datei (Base64) mit Inhaltstyp
  column "input" {
    type = text` + opt + `
  }
  column "input_type" {
    type = text` + opt + `
  }
  # Zusatzzeilen des Aufrufers (Kontext), je Zeile eine
  column "context" {
    type = text` + opt + `
  }
  # gesendete Nachrichten (JSON, ohne Datei)
  column "prompt" {
    type = text` + opt + `
  }
  column "response" {
    type = text` + opt + `
  }
  # Ergebnis: JSON (OBJECT) bzw. Text
  column "result" {
    type = text` + opt + `
  }
  column "duration_ms" {
    type    = bigint
    default = 0
  }
  column "tokens" {
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
  primary_key { columns = [column.log_id] }
}
`
