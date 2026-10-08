# DBSchema – Datenbankschema für Module

Das Core-Plugin `dbschema` verwaltet die Tabellen aller Module. Ein Modul
beschreibt sein **Soll-Schema** in Atlas-HCL. Der Host gleicht es mit
[Atlas](https://atlasgo.io) gegen die vorhandenen Tabellen ab und wendet nur die
Differenz an.

Dabei gilt immer:

- **Isolation:** Ein Modul sieht und ändert nur seine eigenen Objekte – per
  Namenspräfix oder (PostgreSQL) im eigenen DB-Schema.
- **Keine Datenverluste:** `DROP TABLE`, `DROP COLUMN` und Umbenennungen werden abgelehnt.
- **Einmal pro Version:** Pro Modulversion läuft die Migration genau einmal,
  transaktional und zusammen mit dem Eintrag in `coremesh_schema_migrations`.

## Ablauf beim Modulstart

```
Host                                   DBSchema (Core-Plugin)          Modul
 │ CheckVersion(module, version) ─────▶ migriert?
 │ ◀──────────── {migrated: true} ────  → Init wird übersprungen
 │
 │ ◀──────────── {migrated: false}
 │ Init(module, version, installed) ─────────────────────────────────▶ liefert
 │ ◀──────────────────────────────── {module, schema (HCL)} ─────────  Soll-Schema
 │ Prüfung: module == Manifest.Name, access: write
 │ Activate(module, version, schema) ─▶ 1. HCL auswerten (ohne SQL)
 │                                      2. Isolation prüfen
 │                                      3. TX: Version registrieren,
 │                                         Ist-Zustand <prefix>* lesen,
 │                                         Atlas-Diff, Änderungen prüfen,
 │                                         anwenden, COMMIT
 │ Modul registrieren → erhält Anfragen
```

Erst danach registriert der Host die Routen des Moduls. Anfragen erreichen ein
Modul also nie, bevor seine Tabellen in der passenden Version existieren.

## Aufruf: `DBSchema` / `Init`

### 1. Capability im Manifest melden

```go
{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}}   // "DBSchema", "Init"
```

`DBSchema.Init` ist eine **reservierte Lebenszyklus-Capability**:

- Der Dispatcher routet sie nicht.
- Mehrere Module dürfen sie melden.
- Von außen und von anderen Plugins ist sie nicht aufrufbar.

### 2. Request (vom Host an das Modul): `sdk.SchemaInitRequest`

| Feld        | Typ        | Beispiel            | Bedeutung                                  |
|-------------|------------|---------------------|--------------------------------------------|
| `module`    | `string`   | `"partner-service"` | Name des Moduls (`Manifest.Name`)          |
| `version`   | `string`   | `"1.1.0"`           | Version, die migriert wird                 |
| `installed` | `[]string` | `["1.0.0"]`         | Bereits migrierte Versionen, älteste zuerst |

```json
{ "module": "partner-service", "version": "1.1.0", "installed": ["1.0.0"] }
```

### 3. Response (vom Modul an den Host): `sdk.SchemaInitResponse`

Die Antwort hat **zwei Parameter**:

| Feld     | Typ      | Pflicht | Bedeutung                                                    |
|----------|----------|---------|--------------------------------------------------------------|
| `module` | `string` | ja      | Modulname. Muss `Manifest.Name` entsprechen, sonst lehnt der Host ab. |
| `schema` | `string` | ja      | **Vollständiges** Soll-Schema des Moduls als Atlas-HCL        |

```json
{
  "module": "partner-service",
  "schema": "schema \"main\" {}\ntable \"partner_service__business_partner\" { ... }"
}
```

`schema` beschreibt immer den **ganzen Zielzustand** und keine Änderungsskripte. Atlas
berechnet die Differenz zum Ist-Zustand selbst. Eine neue Spalte in Version
1.1.0 heißt also: Das Schema von 1.0.0 nehmen und die Spalte ergänzen.

### 4. Implementierung im Modul (Go)

```go
const schemaHCL = `
schema "main" {}

table "partner_service__business_partner" {
  schema = schema.main
  column "id"   { type = text }
  column "name" { type = text }
  column "email" {          # neu in 1.1.0
    type = text
    null = true
  }
  primary_key { columns = [column.id] }
  index "partner_service__bp_name" { columns = [column.name] }
}

table "partner_service__address" {
  schema = schema.main
  column "id"         { type = integer }
  column "partner_id" { type = text }
  primary_key { columns = [column.id] }
  foreign_key "partner_service__address_partner" {
    columns     = [column.partner_id]
    ref_columns = [table.partner_service__business_partner.column.id]
  }
}
`

func (p *partner) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	if req.Object == sdk.ObjectDBSchema && req.Action == sdk.ActionInit {
		var in sdk.SchemaInitRequest
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: sdk.SchemaInitResponse{
			Module: in.Module,
			Schema: schemaHCL,
		}}, nil
	}
	// … fachliche Actions
}
```

Ein lauffähiges Beispiel steht in [`examples/plugins/hello`](../../../examples/plugins/hello/main.go).

### 5. Konfiguration

Das Modul braucht Schreibrecht auf die Schema-Datenbank (`plugins.dbschema.settings.database`, Standard `main`):

```yaml
plugins:
  partner-service:
    version: 1.1.0
    databases:
      main: { access: write }
```

## Isolationsmodi

Eingestellt wird der Modus in `plugins.dbschema.settings.isolation`:

| | `prefix` (Standard) | `schema` |
|---|---|---|
| Datenbanken | SQLite, PostgreSQL | nur PostgreSQL |
| Ablage der Modul-Tabellen | Standard-Schema (`main` / `public`) | eigenes DB-Schema `mod_<modul>` (`sdk.SchemaName`) |
| Namensregel | Präfix `<modul>__` für Tabellen, Indizes, FKs | keine – Namen sind pro Schema eindeutig |
| Atlas liest | `Include = ["<prefix>*"]` + exakter Präfix-Filter | nur `mod_<modul>` |
| Laufzeit (Query/Exec des Moduls) | Standard-Pool | eigener Pool mit `search_path=mod_<modul>` |

Im Schema-Modus legt `Activate` das Schema mit `CREATE SCHEMA IF NOT EXISTS`
an, in derselben Transaktion wie die Migration. Der Schema-Name im HCL
(`schema "main" {}`) spielt keine Rolle, der Host ordnet alle Tabellen dem
Modul-Schema zu.

**Empfehlung für portable Module:** Verwende die Präfix-Namen (`<modul>__…`)
immer, auch im Schema-Modus. Sie sind dort erlaubt, und dasselbe Modul läuft
dann unverändert mit SQLite und mit PostgreSQL in beiden Modi. Verwende im SQL
des Moduls immer `?`-Platzhalter: Für PostgreSQL wandelt der Host sie in
`$1, $2, …` um.

**Sicherheitsgrenze:** Für Migrationen setzt DBSchema die Isolation in beiden
Modi vollständig durch. **Zur Laufzeit** prüft der Host jede Anweisung, die ein
Plugin über `HostService.Query`/`Exec` schickt (`internal/database/guard.go`):

- genau eine Anweisung; nur `SELECT`, `WITH`, `VALUES`, `INSERT`, `UPDATE`, `DELETE`,
  `REPLACE` – kein DDL, kein `PRAGMA`, `ATTACH`, `SET`, keine Transaktionssteuerung
  (`BEGIN`/`COMMIT` …, dafür `BeginTx`), kein `SELECT … INTO`;
- **geändert werden nur eigene Tabellen:** Jedes Ziel von `INSERT`, `UPDATE`, `DELETE`,
  `REPLACE`, `MERGE` – auch in CTEs und Unterabfragen – trägt das Präfix des Plugins
  bzw. liegt im eigenen Schema `mod_<modul>`; ohne Freigabe `access: write` ändert ein
  Plugin gar nichts, auch nicht über `Query`;
- Lesen fremder Tabellen bleibt möglich; fremde Daten gehören aber über die Actions des
  anderen Plugins gelesen (dort greifen dessen Rechte).

Die Prüfung arbeitet auf Tokens, nicht als vollständiger SQL-Parser. Funktionen mit
Seiteneffekten (PostgreSQL) erkennt sie nicht; eine harte Grenze ziehen erst eigene
DB-Rollen pro Modul (siehe „Grenzen“).

## Namensregeln (Präfix-Modus)

Das Präfix ist der Modulname mit `_` statt `-`, gefolgt von **zwei** Unterstrichen.
Im Code liefert es `sdk.TablePrefix(module)`:

| Modul             | Präfix               | Beispiel-Tabelle                       |
|-------------------|----------------------|----------------------------------------|
| `partner`         | `partner__`          | `partner__business_partner`            |
| `partner-service` | `partner_service__`  | `partner_service__business_partner`    |

Das doppelte `__` sorgt dafür, dass sich Präfixe nicht überschneiden: `partner__`
ist kein Präfix von `partner_service__contact`. Modulnamen dürfen deshalb keine
doppelten Bindestriche enthalten. Der Name `coremesh` ist für den Host reserviert.

Das Präfix ist Pflicht für:

- alle **Tabellen**,
- alle **Indizes**: In SQLite und PostgreSQL liegen Indexnamen in einem gemeinsamen Namensraum,
- alle **benannten Fremdschlüssel**.

Nicht erlaubt sind:

- Fremdschlüssel auf Tabellen **anderer Module**,
- Views, Sequenzen, Typen und andere Schema-Objekte.

## Erlaubte und verbotene Änderungen

| Änderung                                        | Ergebnis      |
|-------------------------------------------------|---------------|
| Neue Tabelle, neue Spalte                       | ✅ erlaubt     |
| Spalte ändern (Typ, NULL, Default)              | ✅ erlaubt ¹   |
| Index, Primärschlüssel, Check, eigener Fremdschlüssel anlegen/ändern/entfernen | ✅ erlaubt |
| `DROP TABLE` (Tabelle fehlt im Soll-Schema)     | ❌ abgelehnt   |
| `DROP COLUMN` (Spalte fehlt im Soll-Schema)     | ❌ abgelehnt   |
| Tabelle/Spalte umbenennen                       | ❌ abgelehnt   |
| Objekt eines anderen Moduls anfassen            | ❌ abgelehnt   |
| Jede andere Änderungsart                        | ❌ abgelehnt (fail closed) |

¹ Eine Typänderung kann je nach Datenbank Daten verlieren, zum Beispiel beim Verkürzen
eines Textfelds. Wer das ausschließen will, ergänzt in `checkTableChanges` eine
Prüfung auf `schema.ModifyColumn` mit `ChangeType`.

**Spalte loswerden:** Eine Spalte bleibt im Schema stehen, wird aber nicht mehr
benutzt (bei Bedarf `null = true`). Ein echtes Löschen ist eine bewusste
Admin-Operation außerhalb des Modul-Lebenszyklus.

## Fehler

Eine Ablehnung beendet den Host-Start mit einer Meldung, die jede Verletzung
einzeln nennt. Es wird nichts verändert, und die Version wird nicht registriert:

```
Migration partner 2.0.0: permission denied: Migration von Modul partner abgelehnt:
permission denied: destruktive Schema-Änderung: DROP COLUMN partner__business_partner.email
permission denied: destruktive Schema-Änderung: DROP TABLE partner__address
```

| Fehler (`errors.Is`)      | Ursache                                                   |
|---------------------------|-----------------------------------------------------------|
| `sdk.ErrPermissionDenied` | Isolation verletzt, destruktive Änderung, fremder Modulname, fehlendes `access: write` |
| `sdk.ErrInvalidArgument`  | HCL nicht auswertbar, `module`/`version` fehlt            |
| `sdk.ErrUnimplemented`    | Datenbanktreiber wird für Modul-Migrationen noch nicht unterstützt |

## Warum HCL und nicht SQL?

Atlas wertet HCL **rein im Speicher** aus. Kein einziges SQL-Statement des Moduls
wird ausgeführt, und die Statements, die tatsächlich auf der Datenbank laufen,
erzeugt Atlas selbst aus dem geprüften Diff.

Bei SQL müsste der Host das DDL zuerst in einer Hilfsdatenbank ausführen, um
den Zielzustand zu ermitteln. Dort könnten Anweisungen wie `ATTACH DATABASE`
oder `VACUUM INTO` Dateien schreiben. Die HCL-Funktionen von Atlas haben in
der Standardkonfiguration keinen Datei- oder Umgebungszugriff.

## Grenzen und nächste Schritte

- **Datenbanken:** SQLite und PostgreSQL (Treiber `pgx`). Die PostgreSQL-Tests
  laufen nur mit `COREMESH_TEST_POSTGRES=postgres://…` (Benutzer mit
  `CREATEDB`). Jeder Test legt eine eigene Datenbank an und löscht sie wieder.
- **DB-Rollen pro Modul** (nächster Schritt für `isolation: schema`): Jedes
  Modul bekommt eine eigene Rolle mit `USAGE`/`CREATE` nur auf `mod_<modul>`,
  und der Modul-Pool verbindet sich mit dieser Rolle. Dann verhindert die
  Datenbank selbst Zugriffe auf fremde Schemata, auch voll qualifiziert.
- **Portable HCL-Typen:** HCL wird dialektspezifisch ausgewertet. Für Module, die
  auf beiden Datenbanken laufen sollen, nur gemeinsame Typen verwenden (`text`,
  `integer`, `bigint`, `boolean`, …). Auto-Increment-Schlüssel unterscheiden
  sich: SQLite zählt `INTEGER PRIMARY KEY` hoch, PostgreSQL braucht `identity`.
  Am portabelsten sind Text-IDs aus dem Modul.
- **Datenmigrationen** wie Backfill oder Umrechnung decken deklarative Schemata nicht
  ab. Dafür bietet sich eine eigene Lebenszyklus-Capability an (z. B.
  `DBSchema.Migrate`), die nach dem Schema-Abgleich in derselben Transaktion läuft.
- **MySQL/MariaDB** committen DDL sofort, dort wäre eine fehlgeschlagene
  Migration nicht vollständig zurückrollbar.
