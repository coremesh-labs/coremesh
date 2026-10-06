package main

// schemaHCL ist das Soll-Schema des WebServers (Atlas-HCL), geliefert über
// die Lebenszyklus-Capability DBSchema.Init. Alle Namen tragen das Präfix
// sdk.TablePrefix("webserver") == "webserver__".
//
// Seit 0.3.0 verwaltet das Core-Plugin iam die Benutzer. webserver__users
// bleibt trotzdem im Schema: DBSchema lehnt DROP TABLE ab (Schutz vor
// Datenverlust). Die Tabelle wird nicht mehr benutzt; die Session-Tabelle
// verweist nicht mehr auf sie (Fremdschlüssel entfernt – das ist erlaubt).
//
// Zeitstempel sind UTC im RFC-3339-Format (Text): portabel zwischen SQLite und
// PostgreSQL und lexikografisch vergleichbar.
const schemaHCL = `
schema "main" {}

table "webserver__users" {
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
  column "created_at" {
    type = text
  }
  column "updated_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "webserver__users_username" {
    unique  = true
    columns = [column.username]
  }
}

table "webserver__sessions" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "user_id" {
    type = text
  }
  column "created_at" {
    type = text
  }
  column "expires_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "webserver__sessions_user" {
    columns = [column.user_id]
  }
}
`
