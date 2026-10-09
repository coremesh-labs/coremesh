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

| Tabelle | Object (Oberfläche/API) | Primärschlüssel | Zeitscheibe |
|---|---|---|---|
| `partner__address_roles` | `PartnerAddressRole` | `code, valid_from` | ✔ |
| `partner__comm_categories` | `PartnerCommCategory` | `code` | – |
| `partner__comm_types` | `PartnerCommType` | `code, valid_from` | ✔ |
| `partner__role_types` | `PartnerRoleType` | `code, valid_from` | ✔ |
| `partner__bp` | `BusinessPartner` | `id` (BP-Nummer)`, valid_from` | ✔ (seit 0.5.0) |
| `partner__groups` | `PartnerGroup` | `code` | – |
| `partner__roles` | `PartnerRole` | `bp_id, role_code, valid_from` | ✔ |
| `partner__addresses` | `PartnerAddressData` | `id` (generiert) | – |
| `partner__bp_addresses` | `PartnerAddress` | `id` (generiert)`, valid_from` | ✔ |
| `partner__contacts` | `PartnerContact` | `id` (generiert)`, valid_from` | ✔ |
| `partner__bank_details` | `PartnerBankDetail` | `id` (generiert)`, valid_from` | ✔ |
| `partner__company_codes` | `PartnerCompanyCode` | `bp_id, company_code, role_code` | – |

**Regel: Das Beginndatum ist immer Teil des Primärschlüssels.** Jede Tabelle mit Zeitscheibe
hat `valid_from` als letzten Teil des Primärschlüssels; ein Test prüft das für Code und Schema.

- **Mehrere Zeitscheiben je fachlichem Schlüssel** sind möglich, zum Beispiel ein Rollentyp
  `DEBITOR` bis 2029-12-31 und ein neuer ab 2030-01-01. Sie dürfen sich **nicht überschneiden**.
  Das prüfen `create`, `update` und `expire`.
- **IDs:**
  - `_id` identifiziert den Datensatz, also die Zeitscheibe. Die Teile sind URL-kodiert und
    durch `|` getrennt, zum Beispiel `9834…|1900-01-01` oder `9834…|DEBITOR|2026-01-01`.
  - `id` bleibt der **fachliche Schlüssel**, wo es eine Spalte `id` gibt (Partner, Zuordnungen,
    Kontakte, Bank). Darauf verweisen andere Datensätze, etwa `bp_id`. Ohne Spalte `id` ist
    `id` der fachliche Schlüssel ohne Beginndatum, zum Beispiel `DEBITOR`.
  - Ein Aufruf **nur mit dem fachlichen Schlüssel**, ohne Beginndatum, trifft die heute
    gültige Zeitscheibe, sonst die jüngste. So arbeiten Verweise und das Aggregat. Mit `_id`
    wird genau eine Zeitscheibe angesprochen.
  - Das Beginndatum ist nach dem Anlegen fest, weil es Schlüssel ist. Eine neue Zeitscheibe ist
    eine Neuanlage.
- **Verweise auf Tabellen mit Zeitscheibe sind keine Fremdschlüssel der Datenbank.** Sie würden
  auf keine eindeutige Zeile zeigen. Die Engine prüft sie zeitbezogen: Das Ziel muss am
  `valid_from` des verweisenden Datensatzes gültig sein. Ein beendeter Partner nimmt also keine
  neuen Kontakte ab seinem Enddatum an. Fremdschlüssel bleiben nur auf Tabellen ohne
  Zeitscheibe (`comm_types` → `comm_categories`, `bp_addresses` → `addresses`).

**Migration auf 0.5.0:** DBSchema baut die Tabellen mit geändertem Primärschlüssel neu auf
(Atlas: neue Tabelle, Daten kopieren, umbenennen) und ohne Datenverlust. Bestehende Partner
gelten ab `1900-01-01` bis `9999-12-31`. Die Spalte `is_active` aus 0.4.0 bleibt ungenutzt
erhalten, weil DBSchema `DROP COLUMN` ablehnt. Den Weg prüft `TestMigrationFrom040` mit dem
Schema von 0.4.0 (`testdata/schema-0.4.0.hcl`).

**Seeds:** `DBSchema.Init` liefert neben dem Schema auch den Grundbestand der Kataloge
(`sdk.SchemaInitResponse.Seed`). DBSchema fügt ihn in der Transaktion der Migration ein,
und zwar **nur fehlende Zeilen**. Geänderte Beschreibungen im Bestand bleiben erhalten.

| Katalog | Einträge |
|---|---|
| Kommunikationskategorien | PHONE, EMAIL, WEB, FAX |
| Kommunikationstypen | EMAIL_WORK (Haupttyp EMAIL), PHONE_WORK (Haupttyp PHONE), MOBILE |
| Adressrollen | MAIN (Hauptanschrift), INVOICE |
| Rollentypen | DEBITOR und TENANT (Mieter) mit `is_debitor`, CREDITOR und LANDLORD (Vermieter) mit `is_creditor` |

Seit 0.9.0 sind Mieter und Vermieter in den Vorschlagswerten Finanzrollen. Bestehende Kataloge
bleiben unverändert (Seeds fügen nur fehlende Zeilen ein); umgestellt wird im Katalog
„Rollentypen“ über die Häkchen Debitor bzw. Kreditor.

## Art, Geschlecht und Anrede (seit 0.11.0)

- **Art:** natürliche Person (`PERSON`) oder juristische Person bzw. Organisation (`ORGANIZATION`).
- **Geschlecht** (nur natürliche Personen): weiblich, männlich, divers, unbekannt.
- **Anrede** aus dem Katalog `PartnerSalutation` (Kataloge → Anreden, pflegbar): Bezeichnung,
  Vorlage der Briefanrede (`{name1}` = Nachname/Firma, `{name2}` = Vorname), gültig für Art und
  Geschlecht. Vorbelegt: Frau, Herr, Guten Tag (divers), Firma. Ohne Anrede schlägt die Pflege
  die erste passende vor (Art, Geschlecht); die **Briefanrede** zeigt der Partner berechnet an.
- Rollentyp **Behörde / Amt** (`AUTHORITY`, Kreditor) für Ämter und Behörden.

## BP-Nummer, Partnergruppe und Kurzname (seit 0.10.0)

- **BP-Nummer** ist der Schlüssel des Partners (`id`) und nach dem Anlegen fest. Sie kommt aus
  dem Nummernkreis **`BusinessPartner`** (Core-Plugin `numrange`); das Intervall bestimmt die
  **Partnergruppe** (Katalog `PartnerGroup`: Code, Intervallschlüssel, Standardgruppe).
- **Intern oder extern** entscheidet das Intervall (Nummernkreise → Intervall → „Externe
  Vergabe“, optional mit erlaubtem Muster, z. B. `[A-Z][A-Z0-9-]{2,11}` für sprechende
  Schlüssel). Extern gibt der Sachbearbeiter die Nummer ein; ist sie vergeben oder passt sie
  nicht, lehnt das Anlegen ab und er wählt eine andere. Intern blendet die Maske das Feld aus.
- Vorschlag: Standardgruppe `STD`, intern 100000–999999. Weitere Gruppen (Mieter, Eigentümer,
  Dienstleister, Behörden …) mit eigenen Intervallen legt man im Katalog an.
- **Kurzname (Matchcode)** (`search_term`): änderbar, für Suche, Auswahl und Texte. Leer =
  Vorschlag aus Name 1 (Großbuchstaben, Umlaute ausgeschrieben, bei Dubletten mit Zähler:
  `MUELLER`, `MUELLER2`). Eindeutigkeit über die Einstellung
  `settings.modules.businesspartner.short_name_unique` (Standard `true`).
- **Übernahme (Migration):** Partner bis 0.9.0 hatten eine GUID. Beim ersten Aufruf nach dem
  Update bekommen sie eine BP-Nummer der Standardgruppe; die GUID bleibt in `legacy_id`
  („Frühere ID“). Je Partner geht das SystemEvent `BusinessPartner.rekey` (`old_id`, `new_id`)
  hinaus (Tags folgen); andere Module stellen ihre Verweise mit `pkg/sdk/bpref` über
  **`BusinessPartnerService.resolve`** (`{ids}` → `{ids: {alt: neu}}`) um – contract,
  realestate und ledger tun das in ihrem `Migrate`.

## Aufrufe

Jedes Object bietet `list`, `get`, `create` und `update`, dazu je nach Lebenszyklus `expire`
oder `deactivate` (siehe unten). Konventionen des WebServers: `{data}`, `{id, data}`, `{id}`, `{query}`.

- **Filter in `list`:** zum Beispiel `bp_id`, `role_code`, `company_code` oder
  `category_code`, je nach Object.
- **Suche:** `q` sucht bei `BusinessPartner` in Name 1, Name 2 und Suchbegriff.
- **Filter `role`** (seit 0.8.0): `BusinessPartner` mit `role=OWNER` liefert nur Partner, die die
  Rolle heute haben. Andere Module nutzen ihn für die Auswahl, z. B. die Immobilien
  (`Lookup.Filters: {"role": "role_code"}` – nur Eigentümer bei Rolle Eigentümer).

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
- Katalogeinträge werden nicht gelöscht. Einträge mit Gültigkeit enden über ein Enddatum;
  Verweise prüfen die Gültigkeit am Stichtag.

**Kommunikation nach Kategorie**

Geprüft wird über die `category_code` des Kommunikationstyps:

| Kategorie | Prüfung |
|---|---|
| `EMAIL` | gültige E-Mail-Adresse |
| `PHONE`, `FAX` | nur Ziffern, Leerzeichen und `+ ( ) / . -`, mindestens 3 Ziffern |
| `WEB` | URL mit `http://` oder `https://` |
| andere | keine Prüfung |

**Finanzrollen und Buchungskreise**

- **Finanzrolle** ist eine Rolle, deren Typ `is_debitor` oder `is_creditor` trägt – das ist
  Konfiguration im Katalog „Rollentypen“, nicht im Code. **Jede Finanzrolle hat ihre eigenen
  Buchungskreisdaten mit eigenem Abstimmkonto** (z. B. Mieter und Debitor getrennt).
- **Buchungskreis-Zwang:** Wer eine Finanzrolle zuweist, muss im selben Aufruf mindestens
  einen Buchungskreis mitgeben (`company_codes`). Rolle und Buchungskreisdaten entstehen
  in einer Transaktion. Bei anderen Rollen sind Buchungskreise nicht erlaubt.
- **Buchungskreisdaten** gibt es nur für Finanzrollen, die der Partner hat. Das
  **Abstimmkonto ist Pflicht** (seit 0.9.0).
- **Kontenplanwechsel** (seit 0.12.0): Abonnent des Hooks `ledger.chart_change` des Hauptbuchs –
  Abstimmkonten ohne Zuordnung verhindern den Wechsel (check), sonst werden sie im Buchungskreis
  umgestellt (commit).
- **Speichern nur vollständig:** Hat ein Partner eine heute gültige Finanzrolle ohne
  Buchungskreisdaten (etwa weil der Rollentyp nachträglich zur Finanzrolle wurde), lehnt
  `BusinessPartner.update` ab und nennt die Rolle. Ergänzen über „Buchungskreisdaten“.
- Buchungskreisdaten sind immutable: Sie bleiben, auch wenn die Finanzrolle endet.
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
- **Beenden eines Partners:** Er bekommt ein Enddatum (`expire`). Rollen, Adresszuordnungen,
  Kontakte, Bankverbindungen und Buchungskreisdaten bleiben erhalten (Historie). Ab dem
  Enddatum läuft die gesetzliche Aufbewahrungsfrist; das spätere Löschen nach Fristablauf ist
  vorbereitet, aber noch nicht umgesetzt.

## Lebenszyklus: Beenden und Inaktivieren statt Löschen

Physisch gelöscht wird nichts. Der Typ folgt aus der Entität (`lifecycle.go`):

| Typ | Objects | Action |
|---|---|---|
| **A** Zeitscheibe | `BusinessPartner` (seit 0.5.0), `PartnerRole`, `PartnerAddress` (Zuordnung mit Adressrolle), `PartnerContact`, `PartnerBankDetail`, Kataloge mit Gültigkeit: `PartnerAddressRole`, `PartnerCommType`, `PartnerRoleType` | `expire {id, valid_to}`: Enddatum Pflicht, nicht vor `valid_from`, rückwirkend erlaubt |
| **B** Status-Flag | derzeit keines (der Partner war es in 0.4.0) | `deactivate {id}` |
| **C** immutable | `PartnerAddressData` (Adressdetails), `PartnerCommCategory`, `PartnerCompanyCode` | keine; `delete` gibt es nicht |

Die Kataloge **Adressrollen, Kommunikationstypen und Rollentypen haben eine Zeitscheibe** und
sind deshalb Typ A, nicht C. Ein Katalog ohne Gültigkeit wie die Kommunikationskategorien ist
Typ C. Die Regel richtet sich allein nach den Metadaten.

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
| Merkmale (zugeklappt) | Tags aus dem [TagManagement](../tag/README.md) (`Tags: true`), je Buchungskreis | Auswahlwerte der Tags |

**Lookups und Labels.** Jeder Verweis (`ref` in `entities.go`) mit `Object` und `LabelFields`
erscheint im Metamodell als Lookup. Die Engine liefert in jedem Datensatz `"_labels"` mit dem
lesbaren Text, je Feld mit einer einzigen Abfrage für alle Datensätze. Die
Kataloge unterstützen die Suche `q` über Code und Beschreibung.

**Aggregat.** `BusinessPartner.getAggregate` und `BusinessPartner.saveAggregate` stellt
`pkg/sdk/module` automatisch aus den Relationen bereit. Beispiel: Partner mit Adresse und
E-Mail in einem atomaren Aufruf:

```bash
console --object BusinessPartner --action saveAggregate --param 'data={"type":"ORGANIZATION","name1":"Neu AG"}' \
  --param 'relations={"adressen":{"create":[{"address_id":"…","address_role_code":"MAIN"}]},"kommunikation":{"create":[{"comm_type_code":"EMAIL_WORK","value":"info@neu.ch"}]}}'
```

## Sprachen und Historie

- **Übersetzungen:** `internal/businesspartner/i18n/de.json`, `en.json`, `zh-CN.json` enthalten
  alle Titel, Feld-Labels, Auswahlwerte, Abschnitte und Navigationsgruppen. Die chinesischen
  Fachbegriffe folgen der SAP-Terminologie: 业务伙伴, 公司代码, 统驭科目, 催款冻结.
  `TestTranslationsComplete` stellt sicher, dass jeder Text in allen drei Sprachen vorliegt und
  keine Datei verwaiste Schlüssel enthält. Katalogbeschreibungen wie „Debitor“ sind Daten und
  bleiben einsprachig.
- **Historie:** `list` liefert standardmäßig nur Datensätze, die heute gültig sind
  (`valid_from <= heute <= valid_to`) bzw. aktiv. Mit `includeHistory=true` in der Query kommen
  auch beendete und künftige Einträge. Das betrifft auch die Lookup-Dialoge und `getAggregate`.

## Aufbau des Codes

| Datei | Inhalt |
|---|---|
| `main.go` | nur Verdrahtung: `module.NewPlugin(Info{partner, 0.8.0}, businesspartner.New())` |
| `internal/businesspartner/module.go` | das **BusinessPartnerModule**: Descriptor, RegisterRoutes, Initialize (DB, Services, Logger), Schema |
| `schema.go` | Atlas-HCL aller Tabellen und Seeds |
| `crud.go` | Aliase auf die gemeinsame CRUD-Engine [`pkg/sdk/crud`](../../../pkg/sdk/crud) (Typumwandlung, Schlüssel, Verweise und `_labels`, Zeitscheiben, Lebenszyklus, Transaktionen, Metamodell). Sie stammt aus diesem Plugin und wird mit `tag` geteilt. |
| `i18n/*.json` | Übersetzungen (de, en, zh-CN), eingebettet; `module.go` implementiert `module.Translator` |
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
- Lebenszyklus: Enddatum (Pflicht, nicht vor Beginn), Inaktivieren, kein `delete`,
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
