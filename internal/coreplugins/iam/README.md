# iam – Benutzer, Rollen und Berechtigungen

Das Core-Plugin `iam` (Identity & Access Management) läuft im Host-Prozess und hat
drei Aufgaben:

1. **Benutzer und Rollen verwalten.** Das geschieht über die generische Oberfläche:
   Die Objects `User`, `Role`, `RoleAuth`, `RoleAuthValue` und `CompanyCode` haben ein Metamodell und bilden das
   Modul **`admin`** („Administration“, `/m/admin`). Der WebServer baut Listen und
   Formulare daraus.
2. **Anmeldedaten prüfen.** `Account.Authenticate` ist nur für Ingress-Plugins wie den
   WebServer erreichbar.
3. **Berechtigungen durchsetzen.** `iam` ist der *Authorizer* des Dispatchers. Jede
   Anfrage eines angemeldeten Benutzers wird gegen seine Rollen geprüft, bevor sie ein
   Modul erreicht.

## Modell

```
Benutzer ──n:m── Rolle ──1:n── Berechtigung (Object.Action, Buchungskreise)
                                     └──1:n── Feldwert (Feld, von, bis)
```

| Tabelle | Inhalt |
|---|---|
| `iam__users` | `id`, `username` (eindeutig, klein geschrieben), `password_hash` (bcrypt), `display_name`, `tenant_id`, `active`, Zeitstempel |
| `iam__roles` | `id`, `name` (eindeutig), `description` |
| `iam__role_auth` | je Rolle und `Object.Action` eine Zeile: `object`, `action`, `company_codes` (kommagetrennt, `*` = alle), `active` (seit 0.5.0) |
| `iam__role_auth_value` | erlaubte Werte eines Berechtigungsfelds: `auth_id`, `field`, `low`, `high`, `active` (seit 0.5.0) |
| `iam__role_permissions` | bis 0.4.0 (eine Zeile je Buchungskreis); wird beim ersten Start von 0.5.0 einmalig übernommen und danach nicht mehr verwendet |
| `iam__company_codes` | `id` (Buchungskreis), `description` |
| `iam__user_roles` | `user_id`, `role_id` |

Die Tabellen entstehen über `DBSchema.Init`, isoliert über das Präfix `iam__`. Das
Plugin braucht dafür in der Konfiguration `databases: { main: { access: write } }`.

## Berechtigungen

Eine Berechtigung erlaubt einer Rolle `Object.Action` in Buchungskreisen und –
optional – nur für bestimmte **Werte der Berechtigungsfelder** des Objects:

| Object | Action | Buchungskreise | Feldwerte | erlaubt |
|---|---|---|---|---|
| `*` | `*` | `*` | | alles (Rolle **Administrator**) |
| `Partner` | `*` | `1000` | | alle Actions auf `Partner` im Buchungskreis 1000 |
| `*` | `list` | `1000, 2000` | | `list` auf allen Objects in 1000 und 2000 |
| `FiscalPeriod` | `post` | `*` | `posting_period` 1 – 12 | buchen nur in den normalen Perioden |
| `FiscalPeriod` | `post` | `1000` | `posting_period` 13 – 16, `ledger` 0L | Sonderperioden, nur Ledger 0L in 1000 |
| `DocumentType` | `post` | `*` | `code` KR, `code` KG | nur Kreditorenrechnungen und -gutschriften buchen |

**Regeln:**

- Object und Action dürfen Platzhalter enthalten (Go `path.Match`, `*` und `?`).
  Groß- und Kleinschreibung zählt, entsprechend den Action-Namen im Metamodell.
- Es gibt **nur Erlaubnisse**. Was keine Rolle erlaubt, ist verboten. Berechtigungen
  ergänzen sich (ODER), die Felder einer Berechtigung müssen alle passen (UND), mehrere
  Werte desselben Felds ergänzen sich (ODER). Ein Feld ohne Werte ist frei.
- Ein **Feldwert** ist ein Einzelwert (`SA`), ein Muster (`4*`) oder ein Bereich
  von – bis (`13` – `16`). Bereiche vergleichen numerisch, wenn Grenzen und Wert ganze
  Zahlen sind, sonst als Zeichenfolge.
- Pro Rolle gibt es je `Object.Action` höchstens **eine aktive Zeile**. Entzogen wird
  über „Entziehen“ (`active = 0`), nicht durch Löschen.

### Rollenpflege

Gepflegt wird in der Rolle im Abschnitt **Berechtigungen**, je `Object.Action` eine
Zeile, darunter die **Feldwerte**. Die Auswahl kommt aus dem **Catalog**
(`RoleAuth.formState`, `RoleAuthValue.formState`):

- **Object:** alle Objects mit Route oder Metamodell (Titel und Name), dazu `*`.
- **Action:** die Routen des Objects und die reinen Berechtigungs-Actions aus
  `metamodel.Authorization.Actions` (z. B. `FiscalPeriod.post`), dazu `*`.
- **Feld:** die Berechtigungsfelder aus `metamodel.Authorization.Fields` – bei einem
  Muster wie `Fiscal*` die aller passenden Objects. Andere Felder lehnt das Speichern ab.

Werte, die der Catalog nicht (mehr) kennt, etwa bei gestopptem Plugin, bleiben
erhalten und wählbar. Das Feld `permissions` der Rolle zeigt alles zusammen in
**Textform**; die Rollen-API (`Role.create/update` mit `permissions`) und das Profil
(`Account.Me`) verwenden sie ebenfalls:

```
FiscalPeriod.post@1000,2000 posting_period=1..12,13 ledger=0L
```

Wer `permissions` mitschickt, ersetzt alle Berechtigungen der Rolle; das Formular
schickt das Feld nicht.

### Berechtigungsfelder deklarieren

Ein Modul macht seine Objects über das Metamodell berechtigungsfähig:

```go
metamodel.ObjectDefinition{
	Name: "FiscalPeriod", …,
	Authorization: &metamodel.Authorization{
		Fields:  []string{"ledger", "fiscal_year", "posting_period"}, // Feld-Keys des Objects
		Actions: []metamodel.AuthAction{{Name: "post", Label: "In der Periode buchen"}},
	},
}
// crud: Entity.Authorization; Übersetzung der Action: <modul>.<Object>.auth.<action>
```

Der Buchungskreis ist immer eine eigene Dimension (`company_code`) und kein
Berechtigungsfeld.

### Was wird wann geprüft?

| Situation | Prüfung |
|---|---|
| Anfrage eines angemeldeten Benutzers (Wurzelanfrage mit `UserID`) | **ja**, gegen seine Rollen |
| Modul ruft innerhalb dieser Anfrage ein anderes Modul auf | nein. Wer `Order.create` darf, braucht `Stock.reserve` nicht zusätzlich. |
| System-Anfragen ohne Benutzer (Host-Start, `host -call`, Login-Vorgang) | nein |
| Katalog lesen (inkl. Übersetzungen), `Account.Me`, `Account.UpdateProfile`, `Account.ChangePassword` | für jeden angemeldeten Benutzer erlaubt |

- **Inaktive Benutzer** haben keine Berechtigungen, und ihre Sessions im WebServer enden
  beim nächsten Aufruf.
- **Cache:** Berechtigungen werden 30 Sekunden zwischengespeichert. Jede Änderung an
  Benutzern oder Rollen leert den Cache sofort.
- **Fehler bei der Prüfung** (z. B. Datenbank nicht erreichbar) führen zur Ablehnung.

Der WebServer blendet zusätzlich Navigation und Buttons aus, die der Benutzer nicht
nutzen darf. Ruft er sie trotzdem auf, antwortet er mit 403. Verbindlich entscheidet
aber immer der Dispatcher, unabhängig davon, über welchen Weg eine Anfrage kommt.

## Actions

| Object.Action | Zweck | Erreichbar |
|---|---|---|
| `Account.Authenticate {username, password}` | Anmeldedaten prüfen → Profil | nur als Wurzelanfrage (Ingress), nie aus einem Plugin heraus |
| `Account.Me` | eigenes Profil inkl. `roles`, `permissions` und `locale` | jeder angemeldete Benutzer |
| `Account.UpdateProfile {locale}` | eigene Sprache (`de`, `en`, `zh-CN`, leer = automatisch); Spalte `iam__users.locale` seit 0.4.0 | jeder angemeldete Benutzer |
| `Account.ChangePassword {current, new}` | eigenes Passwort | jeder angemeldete Benutzer |
| `Account.Check {object, action, attrs}` | → `{allowed}`: darf der Benutzer das mit diesen Werten? (`company_code` als Kurzform für `attrs.company_code`) | jeder angemeldete Benutzer, v. a. Module (`sdk.Authorize`, `sdk.CheckAccess`) |
| `Account.Granted {object, action}` | → `{all, company_codes, rules}` (`sdk.GrantSet`): alle Erlaubnisse mit Feldwerten | jeder angemeldete Benutzer, v. a. Module (`sdk.Grants`, `sdk.GrantedCompanyCodes`) |
| `Account.Display {object}` | → `{rules}`: Darstellungsregeln des Benutzers für das Object | jeder angemeldete Benutzer, v. a. WebServer |
| `User.list/get/create/update/deactivate` | Benutzerverwaltung. Lebenszyklus **status** (`active`): inaktivieren statt löschen; `list` nur aktive, mit `includeHistory=true` alle | mit Berechtigung, z. B. `User.*` |
| `Role.list/get/create/update` | Rollenverwaltung. **immutable**: kein Löschen | mit Berechtigung, z. B. `Role.*` |
| `RoleAuth.list/get/create/update/deactivate/formState` | Berechtigung je Rolle und `Object.Action`. Lebenszyklus **status**; `list` filtert nach `role_id`, `object` | mit Berechtigung, z. B. `RoleAuth.*` |
| `RoleAuthValue.list/get/create/update/deactivate/formState` | Feldwerte einer Berechtigung. Lebenszyklus **status**; `list` filtert nach `auth_id` | mit Berechtigung, z. B. `RoleAuthValue.*` |
| `CompanyCode.list/get/create/update` | Buchungskreise (`code`, `description`). **immutable** | mit Berechtigung, z. B. `CompanyCode.*` |

Die Payloads folgen den Konventionen des WebServers (`{data}`, `{id, data}` …):

- **Rollen eines Benutzers:** Feld `roles`, ein Rollenname pro Zeile.
- **Berechtigungen einer Rolle:** zeilenweise über `RoleAuth`/`RoleAuthValue`; das Feld `permissions` (Textform, eine Berechtigung pro Zeile) ersetzt über die API alle auf einmal.
- **Passwort:** Bei der Neuanlage Pflicht. Beim Bearbeiten heißt leer: unverändert. Es wird
  nie zurückgegeben.

## Berechtigungen im Fachmodul prüfen

Der Dispatcher kennt den Inhalt einer Anfrage nicht. Er prüft deshalb nur, ob der Benutzer
`Object.Action` **überhaupt** darf – in irgendeinem Buchungskreis, mit irgendwelchen
Werten. Zu welchem Buchungskreis und welchen Werten ein Vorgang gehört, weiß nur das
Fachmodul, und es prüft das mit dem SDK (`pkg/sdk/access.go`):

```go
// Einzelner Vorgang: Feldwerte unter den Schlüsseln der Berechtigungsfelder,
// der Buchungskreis unter "company_code".
ok, err := sdk.Authorize(ctx, "FiscalPeriod", "post", sdk.Attrs{
	"company_code": "1000", "ledger": "0L", "fiscal_year": "2026", "posting_period": "13"})
if err != nil {
	return sdk.Response{}, err
}
if !ok {
	return sdk.Response{}, fmt.Errorf("%w: Periode 13", sdk.ErrPermissionDenied)
}

// Viele Prüfungen (Positionen, Auswahlwerte, Listen): Regeln einmal holen.
g, err := sdk.Grants(ctx, "DocumentType", "post")
g.Allows(sdk.Attrs{"company_code": "1000", "code": "KR"})          // lokal prüfen
where, args := g.SQL(map[string]string{"company_code": "company_code_id", "code": "code"})
// → "((company_code_id IN (?) AND (code = ?)))" – Platzhalter ?; "1=0" ohne Recht

// Nur Buchungskreis (bisherige API, unverändert nutzbar):
ok, err = sdk.CheckAccess(ctx, "Partner", "update", partner.CompanyCode)
cc, err := sdk.GrantedCompanyCodes(ctx, "Partner", "list") // cc.All, cc.None(), cc.CompanyCodes
```

- **Fail closed:** Schränkt eine Berechtigung ein Feld ein, das der Aufruf nicht
  mitgibt, passt sie nicht. `CheckAccess` und `GrantedCompanyCodes` berücksichtigen
  deshalb nur Berechtigungen ohne Feldwerte.
- **Benutzer aus dem Kontext:** Alle Funktionen nehmen ihn aus `ctx`. Der Dispatcher
  setzt ihn aus der ursprünglichen Anfrage. Trägt ein Modul einen anderen Benutzer ein,
  wird das überschrieben (Test `TestCheckAccessUsesTrustedUser`).
- **System-Anfragen** ohne Benutzer dürfen alles, wie im Dispatcher.
- **Buchungskreise verwalten:** Buchungskreise sind immutable und werden nie gelöscht. Die
  Nummer ist nicht änderbar, nur die Beschreibung.
- **Upgrade:** Von 0.1.0 erhalten bestehende Berechtigungen `company_code = *`. Von 0.4.0
  übernimmt der erste Start die Zeilen aus `iam__role_permissions` nach
  `iam__role_auth` (je `Object.Action` eine Zeile mit allen Buchungskreisen).

## Datensätze und Feldgruppen (crud.Access)

Neben dem Vorgang („darf er buchen?“) gibt es zwei Ebenen der Sichtbarkeit. Für Module
auf Basis von `pkg/sdk/crud` setzt **crud** sie selbst durch – in Liste, Detail,
Lookup, Kopfdaten-Vorschau, API, beim Ändern und Beenden und bei eigenen
Datensatz-Actions:

| Ebene | Berechtigung | Wirkung |
|---|---|---|
| Datensatz | `<Object>.read` mit Buchungskreisen und Feldwerten | nicht abgedeckte Datensätze fehlen (Liste in der Datenbank gefiltert, sonst „nicht gefunden“); anlegen und ändern nur innerhalb des eigenen Bereichs |
| Feldgruppe | `<Object>.readFields` / `<Object>.changeFields`, Feld `field_group` | ohne Leserecht fehlen die Felder in der Antwort; ohne Änderungsrecht werden Änderungen abgelehnt |

```go
&crud.Entity{
	Object: "RentContract", …,
	Access: &crud.Access{
		Records:     true,
		CompanyCode: "company_code_id",     // Attr company_code
		Fields:      []string{"property_id"}, // weitere Berechtigungsfelder
		FieldGroups: []metamodel.FieldGroup{
			{Key: "bank", Label: "Bankverbindung", Fields: []string{"iban", "bic"}},
		},
	},
}
// Positionen folgen dem Beleg: Access{Object: "JournalEntry", Records: true, CompanyCode: …}
```

- `read` ersetzt für die Sichtbarkeit `list` und `get`; diese prüft der Dispatcher
  weiter, ob die Seite überhaupt aufrufbar ist.
- **Nur Erlaubnisse:** Eine Feldgruppe ist ausgeblendet, bis eine Rolle sie erlaubt
  (z. B. `RentContract.readFields`, `field_group` = bank, Buchungskreis 1000).
  Ungruppierte Felder sind normal sichtbar.
- Die Rollenpflege bietet `read`, `readFields`, `changeFields`, das Feld `field_group`
  und die Feldgruppen aus dem Metamodell an (Texte `admin.auth.*`).
- **Oberfläche (WebServer):** crud markiert Datensätze mit `_hidden_fields` und
  `_readonly_fields`. Detail und Formular blenden aus bzw. sperren; Listen zeigen „—“
  und lassen Spalten weg, die in keiner Zeile sichtbar sind. Beim Speichern gehen beide
  nie mit, die Werte bleiben erhalten.
- Module ohne crud prüfen selbst mit `sdk.Authorize`/`sdk.Grants` (Aktionen
  `read`, `readFields`, `changeFields`).

## Darstellungsregeln (Administration → Darstellung)

Unabhängig von den Plugins legt der Administrator fest, welche Felder eines Objects in
der Oberfläche **ausgeblendet** oder **unänderbar** sind – abhängig von Feldwerten und
optional nur für bestimmte Rollen:

| Teil | Inhalt |
|---|---|
| Regel (`DisplayRule`) | Bezeichnung, Object (Auswahl aus dem Catalog), Rollen (leer = alle), aktiv |
| Bedingungen (`DisplayRuleCondition`) | Feld und Werte (kommagetrennt, einer muss zutreffen); alle Bedingungen müssen zutreffen, keine = immer |
| Felder (`DisplayRuleField`) | Feld und Darstellung `ausblenden` oder `unänderbar` |

Beispiel: `JournalEntryItem`, Bedingung *Herkunft* = RENT → *Kundenauftrag (SD)* und
*Verkaufsorganisation (SD)* ausblenden (den Kunden nicht – dort steht bei RENT der Mieter).

- **Wirkung (WebServer):** Liste, Detail und Unterzeilen zeigen ausgeblendete Felder
  nicht („—“; Spalten fehlen, wenn sie in keiner Zeile sichtbar sind). Im Formular
  fehlen sie bzw. sind schreibgeschützt und werden nicht mitgeschickt – ihr Wert bleibt.
  Felder aus Bedingungen werten die Maske bei Änderung neu aus: Ändert sich der Typ,
  ändert sich die Darstellung sofort.
- **Nur einschränken:** Regeln wirken nach Metamodell, FormState und
  Feldberechtigungen und können nichts einblenden. Pflichtfelder lassen sich nicht
  ausblenden (Pflege lehnt ab, der WebServer zeigt sie trotzdem), nur „unänderbar“.
- **Kein Schutz:** Über die API bleiben die Felder sichtbar. Schützen über
  Feldgruppen (`readFields`/`changeFields`).
- `Account.Display {object}` liefert die Regeln des aufrufenden Benutzers (für jeden
  angemeldeten Benutzer erlaubt); Tabellen `iam__display_rule`, `iam__display_rule_cond`,
  `iam__display_rule_field` (seit 0.6.0).

## Schutz vor Aussperren

Jede Änderung läuft in einer Transaktion. Danach muss **mindestens ein aktiver Benutzer
mit `*.*`** übrig bleiben, sonst wird die Änderung zurückgerollt (`409`). Abgelehnt
werden damit:

- der Rolle Administrator `*.*` entziehen oder es mit Feldwerten einschränken,
- dem letzten Administrator die Rolle nehmen oder ihn deaktivieren,
- sich selbst inaktivieren (`User.deactivate` oder Feld „Aktiv“).

## Erster Start

Gibt es noch keinen Benutzer, legt `iam` nach dem Start an:
- die Rolle **Administrator** (`*.*`),
- den Benutzer `admin_user` mit dieser Rolle.

Das Passwort kommt aus `admin_password`, am besten über
`${env:COREMESH_ADMIN_PASSWORD}`. Fehlt es, wird ein zufälliges erzeugt und **einmalig**
im Host-Log ausgegeben (`WARN … Initialpasswort bitte sofort ändern`).

```yaml
# configs/02-iam-plugin.yaml
plugins:
  iam:
    kind: internal
    databases:
      main: { access: write }
    settings:
      database: main
      admin_user: admin
      tenant: demo
      # admin_password: ${env:COREMESH_ADMIN_PASSWORD}
```

## Sicherheit

| Maßnahme | Umsetzung |
|---|---|
| Passwörter | bcrypt (Kosten 12), mindestens 10 Zeichen |
| Ausforschen von Benutzernamen | gleiche Meldung und Laufzeit für „unbekannt“, „falsches Passwort“ und „deaktiviert“ |
| Durchprobieren von Passwörtern | `Account.Authenticate` nur als Wurzelanfrage. Die Sperre nach Fehlversuchen liegt im WebServer, der die Client-IP kennt. |
| Eindeutigkeit | Benutzer- und Rollennamen haben eindeutige Indizes |

## Grenzen und nächste Schritte

- **Keine Mandanten-Trennung bei Rollen:** Rollen gelten global. Ein Benutzer hat einen
  Mandanten, ob ein Modul Daten danach filtert, entscheidet das Modul über
  `CallFromContext(ctx).TenantID`.
- **Keine Verbote:** Ausnahmen wie „alles außer `User.deactivate`“ brauchen eine
  passende Rolle ohne diese Action.
- **Auswahlfelder:** Rollen eines Benutzers und Buchungskreise einer Berechtigung werden
  als Text bearbeitet. Ein Feldtyp für Mehrfachauswahl im Metamodell wäre komfortabler.
- **Feldwerte ohne Nachschlagehilfe:** Bei Feldern mit festen Werten nennt die Maske die
  Werte im Hinweis; ein Lookup auf die Stammdaten (z. B. Belegarten) fehlt noch.
- **SSO:** Für OIDC oder SAML würde `Account.Authenticate` um einen externen
  Identitätsanbieter ergänzt. Rollen und Berechtigungen blieben in `iam`.

## Übersetzungen

Das Modul `admin` bringt eigene Texte in Deutsch, Englisch und Chinesisch mit
(`i18n/*.json`, Schlüssel `admin.…`). Die Metamodelle erhalten ihre Schlüssel über
`metamodel.WithKeys`.
