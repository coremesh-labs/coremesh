# Skripte und Berichte (`script`)

Das Plugin `script` (Modul `script`, `/m/script`) erweitert das System ohne neues Programm:

- **Skripte** in JavaScript ([goja](https://github.com/dop251/goja), ECMAScript 5.1 mit vielen Teilen
  von ES6+) greifen auf die Objects aller Module zu. Sie laufen
  - über die Maske **„Ausführen …“**, mit eigenen Eingabeparametern,
  - als **Event-Handler** (SystemEvent, nach dem Speichern, asynchron),
  - als **Hook-Abonnent** (modify, check, commit; synchron im Vorgang),
  - als **Datenquelle eines Berichts**.
- **Berichte** werden in der Oberfläche zusammengestellt, ohne Code:
  - Datenquelle: Action eines Objects oder ein Skript,
  - Spalten mit Format und Verdichtung, Gruppen, Sortierung, Summenzeile,
  - Ausgabe als Tabelle mit CSV oder über eine Vorlage (Modul Vorlagen, z. B. PDF).
- **Protokoll** jedes Laufs mit Meldung, Ausgaben von `log()`, Ergebnis und Dauer.
- **Export und Import** als YAML (`format: coremesh.scripts`) über die Fachobjekte, damit die
  Skripte jeden Releasewechsel überstehen.

Gepflegt wird alles in der Weboberfläche unter **Skripte und Berichte**:

| Menü | Object | Inhalt |
|---|---|---|
| Skripte → Skripte | `Script` | Quelltext, erlaubte Aufrufe, Zeitlimit, Protokoll; Abschnitte Parameter, Bindungen, Protokoll; Aktionen „Ausführen …“, „Exportieren …“, „Importieren …“ |
| Skripte → Bindungen | `ScriptBinding` | Skript als Event-Handler oder Hook-Abonnent |
| Skripte → Protokoll | `ScriptRun` | Läufe (Verfall nach den Tagen des Skripts, „Nicht löschen“) |
| Berichte → Berichte | `Report` | Datenquelle, Aufbau, Vorlage; Abschnitte Spalten, Parameter, Protokoll; Aktion „Ausführen …“ |
| Einstellungen | `ScriptParameter`, `ReportParameter`, `ReportColumn` | Unter-Objekte auch als eigene Listen |

## Skripte

Das Skript ist der **Rumpf einer Funktion**: `return` liefert das Ergebnis, Fehler mit Zeilenangabe
kommen als Meldung zurück. Syntaxfehler weist schon das Speichern ab.

| Global | Bedeutung |
|---|---|
| `params` | Werte der Parameter. Typen: Text, Zahl, Datum `JJJJ-MM-TT`, Ja/Nein, Auswahl. Ein Datum ohne Vorbelegung erhält den Stichtag. |
| `event` | das SystemEvent `{object, action, company_code, entity_id, user_id, data, …}` (Bindung EVENT) |
| `hook` | `{name, phase, data, messages}` mit `hook.error(id, text, field)`, `hook.warning(…)`, `hook.info(…)` (Bindung HOOK) |
| `context` | `{script, trigger, detail, user_id, key_date}` |
| `AccessObject(object, action, payload)` | Action eines Moduls; liefert die Antwort. Fehler sind Ausnahmen (`try/catch`). |
| `ReadObject(object, action, payload[, limit])` | Datenstrom (z. B. `JournalEntryItem.list`) als Liste von Objekten |
| `ReadObject(object, action, payload, fn)` | dasselbe, `fn(row, index)` je Zeile; `return false` bricht ab |
| `CallTemplate(template, data, {company_code, language, wait})` | Vorlage rechnen → `{message, job_number, download}` |
| `log(…)` | Ausgabe ins Protokoll und unter die Meldung der Maske |
| `keyDate()` | Stichtag des Benutzers |
| `table(rows[, columns])` | `{columns, rows}` aus Objekten; `columns` als Feldnamen oder `{field, label}` |
| `download(text, file_name[, content_type])` | Datei zum Speichern (Base64) |
| `formatNumber(v[, decimals])`, `formatDate(iso)` | deutsch: `1.234,56`, `31.12.2026` |

**Ergebnis** der Maske „Ausführen …“:

| `return` | Anzeige |
|---|---|
| Text | Meldung |
| Liste von Objekten | Tabelle |
| `{message, rows, columns, table, download}` | Meldung, Tabelle (`rows` + `columns` oder `table`), Datei |
| sonstiges Objekt | JSON |

Bei einem **Hook** gilt:

- Meldungen aus `hook.error/warning/info` bzw. `return {messages: [{type, id, text, field}]}`.
- `modify`: Die (geänderten) Daten sind `hook.data` oder `return {data}`. Das nächste Skript erhält
  den geänderten Stand.
- Ein Laufzeitfehler wird zur Meldung `E` (`SCRIPT-001`). In `commit` macht der Dispatcher daraus
  eine Warnung.

```js
// Hook ledger.posting, Phase check: Positionen über 10.000 € nur mit Text
// (Daten: {request: {company_code, posting_date, items: [{account_number, shkzg, amount, item_text}]}, …})
(hook.data.request.items || []).forEach(function (it, i) {
  if (Number(it.amount) > 10000 && !it.item_text) {   // Betrag als Text mit Dezimalpunkt
    hook.error("BIG-1", "Position " + (i + 1) + ": über 10.000 € nur mit Text", "items");
  }
});
```

```js
// Maske: Salden eines Buchungskreises (Parameter company_code, year)
var res = AccessObject("AccountBalance", "list", {company_code: params.company_code, fiscal_year: params.year});
return {
  message: res.items.length + " Konten",
  rows: res.items.map(function (r) { return {konto: r.account_number, name: r.name, saldo: formatNumber(r.balance_minor / 100)}; })
};
```

### Erlaubte Aufrufe und Berechtigungen

Ein Skript darf nur die Objects aufrufen, die in **„Erlaubte Aufrufe“** stehen:

- je Zeile ein Muster `Object.action`, `*` steht für beliebig, `#` leitet einen Kommentar ein;
- leer = keine Aufrufe;
- neue Skripte bekommen den Vorschlag „nur lesen“: `*.list`, `*.get`, `*.getAggregate`,
  `Template.render`, `Template.get`.

Schreibende Aufrufe (z. B. `Contract.update`) muss man also ausdrücklich eintragen.

Zusätzlich gelten die **Berechtigungen des angemeldeten Benutzers** je `Object.action` (über
`Account.Granted`). Das muss das Plugin selbst prüfen, weil der Dispatcher verschachtelte Aufrufe
aus Plugins nicht noch einmal prüft. Die Fachmodule prüfen Buchungskreise und Feldwerte wie immer
selbst.

**Event-Handler** laufen als Systemanfrage ohne Benutzer. Für sie gelten nur die erlaubten Aufrufe,
darum Skripte und Bindungen nur Administratoren erlauben (`Script.*`, `ScriptBinding.*`).

**Grenzen** stehen in der Plugin-Konfiguration ([`configs/09-script.yaml`](../../../configs/09-script.yaml)):

| Einstellung | Wert |
|---|---|
| Zeitlimit je Skript | Standard 30 s, höchstens `max_timeout_sec` |
| `max_calls` | Aufrufe je Lauf |
| `max_read_rows` | Zeilen je Datenstrom bzw. Bericht |
| `log_bytes` | Umfang der Ausgaben von `log()` |

Ein Skript hat keinen Zugriff auf Dateien oder Netz.

## Bindungen

| Art | Felder | Abo |
|---|---|---|
| EVENT | Object (`*` = alle), Action (leer = alle), Buchungskreis (leer = alle) | `SystemEvent.Register` je Object/Action/Buchungskreis an `ScriptEvent.onEvent` |
| HOOK | Hook (Auswahl aus `Hook.list`), Phase, Priorität | `Hook.Subscribe` je Hook/Phase an `ScriptHook.onHook`, mit der kleinsten Priorität der Bindungen |

Die Abos meldet das Plugin an, und zwar
- beim Start, weil Event-Abos beim Dispatcher nur im Speicher liegen,
- kurz nach jeder Änderung einer Bindung, im Hintergrund als eigene Systemanfrage (`ingress: true`)
  und außerhalb der Transaktion, in der die Bindung gespeichert wurde.

Welche Skripte bei einem Event oder Hook laufen, steht in den aktiven Bindungen aktiver Skripte,
geordnet nach Priorität. Eine inaktive Bindung läuft nicht, ihr Abo läuft dann ins Leere.

## Berichte

| Teil | Felder |
|---|---|
| Datenquelle Object | Object, Action (leer = `list`), Anfrage als JSON mit Platzhaltern `{parameter}` (ein Platzhalter allein behält den Typ; `{key_date}` = Stichtag), „Als Datenstrom lesen“, Pfad der Zeilen (leer = `items` bzw. `rows`). Die Datenquelle ist zugleich die Erlaubnis; die Berechtigung des Benutzers gilt trotzdem. |
| Datenquelle Skript | Skript, das Zeilen liefert (`return [{…}]` oder `{rows: […]}`), mit den Parametern des Berichts |
| Spalten | Feld (Punkte für eingebettete Werte), Überschrift, Verdichtung (keine, Summe, Anzahl, Minimum, Maximum, Durchschnitt), Format (Text, Zahl, ganze Zahl, Betrag, Betrag aus Cent, Datum). Ohne Spalten: alle Felder. |
| Aufbau | Gruppieren nach, Sortieren nach (`-feld` absteigend), höchstens Zeilen, Summenzeile (über alle Zeilen der Quelle) |
| Ausgabe | ohne Vorlage: Tabelle und CSV (Semikolon, deutsche Zahlen, UTF-8 mit BOM für Excel). Mit Vorlage: `Template.render` im Buchungskreis aus dem Parameter `company_code` (bzw. „Parameter mit dem Buchungskreis“) mit den Daten `{title, description, report, params, columns: [{field, label, format, aggregate}], rows (formatiert), values (roh), totals}` |

Beispiel „Salden je Konto“: Object `AccountBalance`, Anfrage
`{"company_code": "{company_code}", "fiscal_year": "{year}"}` und diese Spalten:

| Feld | Verdichtung | Format |
|---|---|---|
| `account_number` | – | – |
| `name` | – | – |
| `balance_minor` | Summe | Betrag aus Cent |

Dazu eine Summenzeile.

## Export und Import

„Exportieren …“ in der Übersicht der Skripte liefert eine YAML-Datei mit
- Skripten samt Parametern und Bindungen,
- Berichten samt Spalten und Parametern.

Auf Wunsch lassen sich nur bestimmte Skripte und Berichte auswählen (Schlüssel als Komma-Liste).

„Importieren …“ legt neu an oder ändert:
- Skripte und Berichte nach Schlüssel,
- Parameter und Spalten nach Name bzw. Reihenfolge,
- Bindungen nach Art, Event bzw. Hook und Phase.

Der Import ist eine Transaktion. „Nur prüfen“ nimmt alles zurück. Er ist wiederholbar, ein zweiter
Lauf ändert nichts. Nicht in der Datei stehende Parameter und Bindungen bleiben erhalten (kein
Löschen).

```bash
console script:export --out=skripte.yaml [--codes=SALDEN,PRUEF]
console script:import --file=skripte.yaml [--dry-run=true]
```

Ausführen von der Konsole – Parameter direkt mit Präfix `p_`:

```bash
console handle Script.run --id=SALDEN --p_kontotyp=BALANCE_SHEET
console handle Report.run --id=SALDEN_KONTO --p_kontotyp=REVENUE
```

```yaml
format: coremesh.scripts
version: 1
scripts:
  - code: PRUEF
    name: Prüfung großer Belege
    source: |
      …
    allowed_calls: |
      *.list
    active: true
    parameters: []
    bindings:
      - {kind: HOOK, hook: ledger.posting, phase: check, priority: 50, active: true}
reports: []
```

## Tabellen

`script__script`, `script__parameter`, `script__binding`, `script__run`, `script__report`,
`script__report_column`, `script__report_parameter`. Andere Plugins lesen sie nicht. Der Zugang führt
über die Objects.
