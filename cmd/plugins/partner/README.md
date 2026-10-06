# Fachmodul Geschäftspartner (`partner`)

Geschäftspartner nach dem Vorbild des SAP-Business-Partner-Modells:

- ein **Partner** (Person oder Organisation),
- **Rollen** mit Zeitscheiben,
- **wiederverwendbare Adressen** mit Adressrolle,
- **Kommunikation**, geprüft nach Kategorie,
- **Bankverbindungen**,
- **Finanzdaten je Buchungskreis** für Debitor- und Kreditorrollen.

Alle Typen und Rollen sind **Stammdaten-Kataloge**.

Das Plugin `partner` beherbergt das fachliche **Modul `businesspartner`**
(`internal/businesspartner`, gebaut mit [`pkg/sdk/module`](../../../pkg/sdk/module/README.md)).
Im WebServer erscheint es unter `/m/businesspartner`, die JSON-API liegt unter
`/api/v1/businesspartner`. Die Navigation gruppiert die Objects in **Partnerdaten** und
**Kataloge**.

Das Plugin hat ein eigenes Go-Modul und nutzt nur `pkg/sdk`. Es braucht
`databases: { main: { access: write } }`.

## Tabellen

Das Präfix `partner__` ist von DBSchema vorgegeben (`<modul>__`). Bei PostgreSQL mit
`isolation: schema` liegen die Tabellen im Schema `mod_partner`.

| Tabelle | Object (Oberfläche/API) | Schlüssel | Zeitscheibe |
|---|---|---|---|
| `partner__address_roles` | `PartnerAddressRole` | `code` | ✔ |
| `partner__comm_categories` | `PartnerCommCategory` | `code` | – |
| `partner__comm_types` | `PartnerCommType` | `code` | ✔ |
| `partner__role_types` | `PartnerRoleType` | `code` | ✔ |
| `partner__bp` | `BusinessPartner` | `id` (generiert) | – |
| `partner__roles` | `PartnerRole` | `bp_id, role_code, valid_from` | ✔ |
| `partner__addresses` | `PartnerAddressData` | `id` (generiert) | – |
| `partner__bp_addresses` | `PartnerAddress` | `id` (generiert) | ✔ |
| `partner__contacts` | `PartnerContact` | `id` (generiert) | ✔ |
| `partner__bank_details` | `PartnerBankDetail` | `id` (generiert) | ✔ |
| `partner__company_codes` | `PartnerCompanyCode` | `bp_id, company_code, role_code` | – |

**Zusammengesetzte Schlüssel** erscheinen in der API und in URLs als eine `id`: die
Teile URL-kodiert, getrennt durch `|`, zum Beispiel `9834…|DEBITOR|2026-01-01`.

**Seeds:** `DBSchema.Init` liefert neben dem Schema auch den Grundbestand der Kataloge
(`sdk.SchemaInitResponse.Seed`). DBSchema fügt ihn in der Transaktion der Migration ein,
und zwar **nur fehlende Zeilen**. Geänderte Beschreibungen im Bestand bleiben erhalten.

| Katalog | Einträge |
|---|---|
| Kommunikationskategorien | PHONE, EMAIL, WEB, FAX |
| Kommunikationstypen | EMAIL_WORK (Haupttyp EMAIL), PHONE_WORK (Haupttyp PHONE), MOBILE |
| Adressrollen | MAIN (Hauptanschrift), INVOICE |
| Rollentypen | DEBITOR (`is_debitor`), CREDITOR (`is_creditor`), TENANT, LANDLORD |

## Aufrufe

Jedes Object bietet `list`, `get`, `create`, `update` und `delete` nach den
Konventionen des WebServers: `{data}`, `{id, data}`, `{id}`, `{query}`.

- **Filter in `list`:** zum Beispiel `bp_id`, `role_code`, `company_code` oder
  `category_code`, je nach Object.
- **Suche:** `q` sucht bei `BusinessPartner` in Name 1, Name 2 und Suchbegriff.

```bash
console --object BusinessPartner --action create --param 'data={"type":"ORGANIZATION","name1":"Muster AG"}'
console --object PartnerRole --action create --param 'data={"bp_id":"…","role_code":"DEBITOR","company_codes":[{"company_code":"1000","reconciliation_account":"140000","payment_terms":"NT30"}]}'
console --object PartnerCompanyCode --action list --param bp_id=…
console --object PartnerContact --action create --param 'data={"bp_id":"…","comm_type_code":"EMAIL_WORK","value":"info@muster.ch"}'
```

In der Weboberfläche nimmt das Feld „Buchungskreise“ einer Rollenzuordnung eine Zeile
je Buchungskreis auf, im Format `1000;140000;NT30` (Buchungskreis;Abstimmkonto;Zahlungsbedingung).

## Regeln

**Zeitscheiben**

- `valid_from` ist standardmäßig heute, `valid_to` standardmäßig `9999-12-31`.
- Es muss immer `valid_from <= valid_to` gelten.
- Ein Verweis auf einen zeitabhängigen Katalog ist nur gültig, wenn der Katalogeintrag am
  `valid_from` des Datensatzes gilt.

**Kataloge**

- Der Code ist nach dem Anlegen fest.
- **Nur eine** Adressrolle darf `is_main` tragen, und **pro Kategorie** nur ein
  Kommunikationstyp.
- Ein Katalogeintrag, der noch verwendet wird, lässt sich nicht löschen (`409`).

**Kommunikation nach Kategorie**

Geprüft wird über die `category_code` des Kommunikationstyps:

| Kategorie | Prüfung |
|---|---|
| `EMAIL` | gültige E-Mail-Adresse |
| `PHONE`, `FAX` | nur Ziffern, Leerzeichen und `+ ( ) / . -`, mindestens 3 Ziffern |
| `WEB` | URL mit `http://` oder `https://` |
| andere | keine Prüfung |

**Finanzrollen und Buchungskreise**

- **Finanzrolle** ist eine Rolle, deren Typ `is_debitor` oder `is_creditor` trägt.
- **Buchungskreis-Zwang:** Wer eine Finanzrolle zuweist, muss im selben Aufruf mindestens
  einen Buchungskreis mitgeben (`company_codes`). Rolle und Buchungskreisdaten entstehen
  in einer Transaktion. Bei anderen Rollen sind Buchungskreise nicht erlaubt.
- **Buchungskreisdaten** gibt es nur für Finanzrollen, die der Partner hat.
- Der **letzte** Buchungskreis einer noch gültigen Finanzrolle lässt sich nicht löschen.
  Endet die letzte Zuordnung einer Rolle, werden ihre Buchungskreisdaten mit gelöscht.
- **Buchungskreise kommen aus `iam`:** Unbekannte Buchungskreise werden abgelehnt.
- **Berechtigungen je Buchungskreis:** Lesen und Schreiben ist auf die Buchungskreise
  beschränkt, die die Rollen des Benutzers für `PartnerCompanyCode.<action>` erlauben
  (`sdk.GrantedCompanyCodes`, `sdk.CheckAccess`). Ein Beispiel für eine Rolle:
  `PartnerCompanyCode.*@1000`.

**Weitere Prüfungen**

- **IBAN:** Format und Prüfziffer (ISO 13616, mod 97), gespeichert ohne Leerzeichen in
  Großbuchstaben.
- **BIC:** 8 oder 11 Zeichen.
- **Land:** ISO-2.
- **Löschen eines Partners:** entfernt seine Rollen, Adresszuordnungen, Kontakte,
  Bankverbindungen und Buchungskreisdaten. Die Adressen selbst bleiben, sie sind
  wiederverwendbar.

## Detailansicht, Lookups und Aggregat

**Detailansicht eines Partners** (`/m/businesspartner/BusinessPartner/{id}`) mit aufklappbaren
Abschnitten:

| Abschnitt | Inhalt | Lookups in der eingebetteten Tabelle |
|---|---|---|
| Stammdaten | Art, Name 1/2, Suchbegriff, Gesperrt, ID | – |
| Rollen | `PartnerRole` | Rolle → `PartnerRoleType` |
| Adressen | `PartnerAddress`, n:m über die Zuordnung mit Rolle und Zeitscheibe | Adressrolle → `PartnerAddressRole`, Adresse → `PartnerAddressData` (✎ bearbeitet die Adresse selbst) |
| Kommunikation | `PartnerContact` | Kommunikationstyp → `PartnerCommType` |
| Bankverbindungen (zugeklappt) | `PartnerBankDetail` | – |
| Buchungskreisdaten (zugeklappt) | `PartnerCompanyCode` | Buchungskreis → `CompanyCode` aus `iam` (über Modulgrenzen, nur lesend) |

**Lookups und Labels.** Jeder Verweis (`ref` in `entities.go`) mit `Object` und `LabelFields`
erscheint im Metamodell als Lookup. Die Engine liefert in jedem Datensatz `"_labels"` mit dem
lesbaren Text, je Feld mit einer einzigen Abfrage für alle Datensätze (`labels.go`). Die
Kataloge unterstützen die Suche `q` über Code und Beschreibung.

**Aggregat.** `BusinessPartner.getAggregate` und `BusinessPartner.saveAggregate` stellt
`pkg/sdk/module` automatisch aus den Relationen bereit. Beispiel: Partner mit Adresse und
E-Mail in einem atomaren Aufruf:

```bash
console --object BusinessPartner --action saveAggregate --param 'data={"type":"ORGANIZATION","name1":"Neu AG"}' \
  --param 'relations={"adressen":{"create":[{"address_id":"…","address_role_code":"MAIN"}]},"kommunikation":{"create":[{"comm_type_code":"EMAIL_WORK","value":"info@neu.ch"}]}}'
```

## Aufbau des Codes

| Datei | Inhalt |
|---|---|
| `main.go` | nur Verdrahtung: `module.NewPlugin(Info{partner, 0.2.0}, businesspartner.New())` |
| `internal/businesspartner/module.go` | das **BusinessPartnerModule**: Descriptor, RegisterRoutes, Initialize (DB, Services, Logger), Schema |
| `schema.go` | Atlas-HCL aller Tabellen und Seeds |
| `engine.go` | tabellengesteuerte CRUD-Engine: Typumwandlung, Schlüssel, Verweise, Zeitscheiben, Transaktionen, Metamodell |
| `labels.go` | lesbare Texte der Verweise (`_labels`) für Oberfläche und API |
| `entities.go` | Kataloge, Partner, Adressen, Kommunikation, Bank |
| `finance.go` | Rollenzuordnung, Buchungskreisdaten, Zugriff je Buchungskreis |
| `validate.go` | Zeitscheiben, E-Mail, Telefon, URL, IBAN, BIC, Land |

Alle Fachdateien liegen in `internal/businesspartner/` und sind damit von außen nicht importierbar.
Datenbankzugriffe laufen über das injizierte `module.DB`, Aufrufe an `iam` (Buchungskreise) über
`module.Services`. Es gibt keine globalen Variablen für Ressourcen.

Eine neue Entität ist eine Beschreibung in `entities.go` (Tabelle, Schlüssel, Felder,
optionale Hooks) plus die Tabelle in `schema.go`. Version erhöhen, dann migriert
DBSchema beim nächsten Start.

## Tests

`go test ./...` legt das Schema mit **Atlas aus dem Schema von `DBSchema.Init`** in SQLite an, spielt die
Seeds ein und prüft unter anderem:

- Seeds und die Eindeutigkeit von `is_main`,
- Zeitscheiben,
- Kommunikation nach Kategorie, IBAN, Land,
- Katalogeinträge in Verwendung,
- das Löschen eines Partners samt Beziehungen,
- Finanzrollen mit Buchungskreis-Zwang, Rollback bei Fehlern und Sicht je Buchungskreis,
- die Gültigkeit aller 11 Metamodelle und das Modul (Gruppen, ein `schema`-Block).
- Labels, Lookup-Metadaten und Suche in Katalogen,
- `saveAggregate` mit Adresse und Kommunikation, Rollback bei ungültiger E-Mail.

Die Tests laufen über `module.NewPlugin`, also über denselben Weg wie im Betrieb: Router,
Dependency Injection und DBSchema.Init.

## Grenzen und nächste Schritte

- **Standard-Kennzeichen:** Mehrere `is_default`-Einträge pro Partner und Typ in
  überlappenden Zeitscheiben werden noch nicht verhindert.
- **Mandanten:** Partner sind nicht nach Mandant getrennt (`tenant_id`). Das ließe sich
  bei Bedarf als Spalte mit Filter aus `CallFromContext(ctx).TenantID` ergänzen.
- **PostgreSQL:** Schema und SQL sind portabel geschrieben, getestet ist bisher nur SQLite.
