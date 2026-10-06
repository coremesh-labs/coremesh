# iam – Benutzer, Rollen und Berechtigungen

Das Core-Plugin `iam` (Identity & Access Management) läuft im Host-Prozess und hat
drei Aufgaben:

1. **Benutzer und Rollen verwalten.** Das geschieht über die generische Oberfläche:
   Die Objects `User` und `Role` haben ein Metamodell, der WebServer baut
   Listen und Formulare daraus.
2. **Anmeldedaten prüfen.** `Account.Authenticate` ist nur für Ingress-Plugins wie den
   WebServer erreichbar.
3. **Berechtigungen durchsetzen.** `iam` ist der *Authorizer* des Dispatchers. Jede
   Anfrage eines angemeldeten Benutzers wird gegen seine Rollen geprüft, bevor sie ein
   Modul erreicht.

## Modell

```
Benutzer ──n:m── Rolle ──1:n── Berechtigung (Object.Action, mit Platzhaltern)
                                     └── je Buchungskreis (oder alle)
```

| Tabelle | Inhalt |
|---|---|
| `iam__users` | `id`, `username` (eindeutig, klein geschrieben), `password_hash` (bcrypt), `display_name`, `tenant_id`, `active`, Zeitstempel |
| `iam__roles` | `id`, `name` (eindeutig), `description` |
| `iam__role_permissions` | `role_id`, `object`, `action`, `company_code` (`*` = alle Buchungskreise) |
| `iam__company_codes` | `id` (Buchungskreis), `description` |
| `iam__user_roles` | `user_id`, `role_id` |

Die Tabellen entstehen über `DBSchema.Init`, isoliert über das Präfix `iam__`. Das
Plugin braucht dafür in der Konfiguration `databases: { main: { access: write } }`.

## Berechtigungen

Eine Berechtigung ist `Object.Action[@Buchungskreis,…]`. Object und Action dürfen
Platzhalter enthalten (Go `path.Match`). Buchungskreise sind feste IDs aus der
Tabelle der Buchungskreise. Ohne `@` (oder mit `@*`) gilt die Berechtigung in
**allen** Buchungskreisen:

| Berechtigung | erlaubt |
|---|---|
| `*.*` | alles (Rolle **Administrator**) |
| `Partner.*` | alle Actions auf `Partner` |
| `*.list` | `list` auf allen Objects |
| `Greeting.l*` | `Greeting.list`, `Greeting.load`, … |
| `Contract?.get` | `Contracts.get`, `ContractX.get` (`?` = ein Zeichen) |
| `Partner.*@1000` | alle Actions auf `Partner`, nur im Buchungskreis 1000 |
| `*.list@1000,2000` | `list` auf allen Objects in 1000 und 2000 |

**Regeln:**

- Groß- und Kleinschreibung zählt, entsprechend den Action-Namen im Metamodell.
- Es gibt **nur Erlaubnisse**. Was keine Rolle erlaubt, ist verboten.
- Im Rollenformular steht eine Berechtigung pro Zeile, `#` leitet einen Kommentar ein.
  Ungültige Einträge und unbekannte Buchungskreise lehnt das Formular mit Meldung ab.

### Was wird wann geprüft?

| Situation | Prüfung |
|---|---|
| Anfrage eines angemeldeten Benutzers (Wurzelanfrage mit `UserID`) | **ja**, gegen seine Rollen |
| Modul ruft innerhalb dieser Anfrage ein anderes Modul auf | nein. Wer `Order.create` darf, braucht `Stock.reserve` nicht zusätzlich. |
| System-Anfragen ohne Benutzer (Host-Start, `host -call`, Login-Vorgang) | nein |
| Katalog lesen, `Account.Me`, `Account.ChangePassword` | für jeden angemeldeten Benutzer erlaubt |

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
| `Account.Me` | eigenes Profil inkl. `roles` und `permissions` | jeder angemeldete Benutzer |
| `Account.ChangePassword {current, new}` | eigenes Passwort | jeder angemeldete Benutzer |
| `Account.Check {object, action, company_code}` | → `{allowed}`: darf der Benutzer das im Buchungskreis? | jeder angemeldete Benutzer, v. a. Module (`sdk.CheckAccess`) |
| `Account.Granted {object, action}` | → `{all, company_codes}`: in welchen Buchungskreisen? | jeder angemeldete Benutzer, v. a. Module (`sdk.GrantedCompanyCodes`) |
| `User.list/get/create/update/delete` | Benutzerverwaltung | mit Berechtigung, z. B. `User.*` |
| `Role.list/get/create/update/delete` | Rollenverwaltung | mit Berechtigung, z. B. `Role.*` |
| `CompanyCode.list/get/create/update/delete` | Buchungskreise (`code`, `description`) | mit Berechtigung, z. B. `CompanyCode.*` |

Die Payloads folgen den Konventionen des WebServers (`{data}`, `{id, data}` …):

- **Rollen eines Benutzers:** Feld `roles`, ein Rollenname pro Zeile.
- **Berechtigungen einer Rolle:** Feld `permissions`, eine Berechtigung pro Zeile.
- **Passwort:** Bei der Neuanlage Pflicht. Beim Bearbeiten heißt leer: unverändert. Es wird
  nie zurückgegeben.

## Buchungskreise im Fachmodul prüfen

Der Dispatcher kennt den Inhalt einer Anfrage nicht. Er prüft deshalb nur, ob der Benutzer
`Object.Action` **in irgendeinem** Buchungskreis darf. Zu welchem Buchungskreis ein
Datensatz gehört, weiß nur das Fachmodul, und es prüft das mit dem SDK:

```go
// Einzelner Datensatz: darf der Benutzer ihn im Buchungskreis ändern?
ok, err := sdk.CheckAccess(ctx, "Partner", "update", partner.CompanyCode)
if err != nil {
	return sdk.Response{}, err
}
if !ok {
	return sdk.Response{}, fmt.Errorf("%w: Buchungskreis %s", sdk.ErrPermissionDenied, partner.CompanyCode)
}

// Liste: nur Datensätze aus erlaubten Buchungskreisen laden.
g, err := sdk.GrantedCompanyCodes(ctx, "Partner", "list")
switch {
case g.All:    // ohne Filter
case g.None(): // leere Liste
default:       // … WHERE company_code IN (g.CompanyCodes…)
}
```

- **Benutzer aus dem Kontext:** Beide Funktionen nehmen ihn aus `ctx`. Der Dispatcher
  setzt ihn aus der ursprünglichen Anfrage. Trägt ein Modul einen anderen Benutzer ein,
  wird das überschrieben (Test `TestCheckAccessUsesTrustedUser`).
- **System-Anfragen** ohne Benutzer dürfen alles, wie im Dispatcher.
- **Buchungskreise verwalten:** Ein Buchungskreis, der in einer Rolle vorkommt, lässt sich
  nicht löschen (`409`, mit den betroffenen Rollen). Seine Nummer ist nicht änderbar,
  nur die Beschreibung.
- **Upgrade von 0.1.0:** Bestehende Berechtigungen erhalten `company_code = *` und
  gelten weiter in allen Buchungskreisen.

## Schutz vor Aussperren

Jede Änderung läuft in einer Transaktion. Danach muss **mindestens ein aktiver Benutzer
mit `*.*`** übrig bleiben, sonst wird die Änderung zurückgerollt (`409`). Abgelehnt
werden damit:

- die Rolle Administrator löschen oder ihr `*.*` entziehen,
- dem letzten Administrator die Rolle nehmen oder ihn deaktivieren,
- sich selbst löschen oder deaktivieren.

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
- **Keine Verbote:** Ausnahmen wie „alles außer `User.delete`“ brauchen eine
  passende Rolle ohne diese Action.
- **Auswahlfelder:** Rollen und Berechtigungen werden als Textfeld bearbeitet. Ein Feldtyp
  für Mehrfachauswahl mit dynamischen Optionen im Metamodell wäre komfortabler.
- **SSO:** Für OIDC oder SAML würde `Account.Authenticate` um einen externen
  Identitätsanbieter ergänzt. Rollen und Berechtigungen blieben in `iam`.
