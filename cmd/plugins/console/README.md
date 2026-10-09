# CoreMesh Console

Kommandozeilen-Zugang zu CoreMesh, bestehend aus zwei Teilen:

| Teil | Ort | Aufgabe |
|---|---|---|
| **Console-Plugin** | `cmd/plugins/console` (eigenes Go-Modul, externes Plugin) | gRPC-Server `ConsoleService` auf Unix-Socket oder TCP, Anmeldung über `iam`, Aufrufe an Module, Beispieldateien, ZIP-Entpacken |
| **CLI** | `cmd/console` | Client: meldet sich an, setzt Befehle ab, schreibt Ergebnisse |

Der gemeinsame Vertrag ist `proto/console/v1/console.proto`, generiert nach
`pkg/consoleapi/console/v1`. Er ist öffentlich, damit auch andere Clients ihn nutzen
können.

```
console (CLI) ──gRPC (Unix-Socket / TCP+TLS)──▶ Console-Plugin ──Host.Handle──▶ Dispatcher ──▶ Modul
     │  Token                                       │ iam: Authenticate, Me, Granted
```

## CLI

### Verben: handle, read, list, details

```bash
console list                                   # alle Objects
console list Contract*                         # Objects nach Muster (* ?; ohne Platzhalter: Teil des Namens)
console list Contract.*                        # Actions eines Objects
console list hooks ledger.*                    # Hooks (Besitzer, Phasen, Zahl der Abos)
console list events JournalDraft               # Event-Abonnements zu einem Object
console list commands setup                    # Konsolenbefehle der Module
console details BankAccount                    # Felder (Typ, Pflicht, Verweis), Actions, Filter
console details Contract.activate              # Route (Plugin, Version), Art, Parameter
console details hook ledger.posting            # Hook mit Phasen, Daten und Abos
console details event JournalDraft.post        # Empfänger eines Events
console handle CompanyCode.get --id=2000       # Action aufrufen
console handle Contract.list --query='{"company_code":"2000"}'
console handle BankAccount.importFile --id='1000|COBA' --file=./auszug.csv
console read JournalEntryItem.list --company_code_id=2000 --format jsonl --out items.jsonl
```

- Verben ohne Rücksicht auf Groß-/Kleinschreibung (`Handle`, `List` …).
- **Parameter** von `handle` und `read`: `--name=wert` (oder wie bisher `--param name=wert`).
  Die CLI liest das Metamodell des Objects und wandelt nach dem Feldtyp um: Textfelder
  bleiben Text (`--company_code=2000`), Zahl- und Schalterfelder werden gelesen, `data` und
  `query` sind JSON, `id` ist immer Text. Datei-Felder (Typ `file`) liest die CLI lokal ein –
  Inhalt und `<feld>_name` wie das Formular des WebServers. Nicht deklarierte Parameter:
  gültiges JSON als Wert, sonst Text.
- `list` und `details` geben eine Tabelle aus, mit `--format json` JSON.
- Quellen: Catalog (`ListObjects`, `ListActions`, `GetDefinition`, `ListModules`), Hook
  (`Hook.list/get`, `HookSubscription.list`), SystemEvent (`List`). Es gilt das Recht des
  angemeldeten Benutzers auf diese Objects.

### Bisherige Form

```bash
console --object Greeting --action list
console --object Greeting --action say --param name=Christof
console --object BusinessPartner --action SampleFile --out ./partner_template.yaml
console --object BusinessPartner --sample --format csv
console --object JournalEntryItem --action list --read --param company_code_id=1000 --out items.csv
console --object AssetsModule --action ExportBundle --param theme=dark --target-dir /var/www/coremesh/static
console --logout
```

| Option | Bedeutung |
|---|---|
| `--addr` | `unix://<pfad>` (Standard `unix://data/console.sock`) oder `tcp://<host>:<port>`, auch über `COREMESH_CONSOLE` |
| `--tls-ca` | bei `tcp://`: TLS mit dieser CA-Datei. Ohne Angabe ist die Verbindung unverschlüsselt. |
| `--user` | Benutzer (`iam`), auch über `COREMESH_USER`, sonst Abfrage |
| `--object`, `--action` | Ziel des Aufrufs |
| `--param key=value` | mehrfach möglich. Gültiges JSON wird als Zahl, `true` oder Objekt übernommen, alles andere als Text. |
| `--target-dir` | absolutes Verzeichnis **auf dem Server**. Dort wird `zip_content` aus der Antwort entpackt. |
| `--sample` | wie `--action SampleFile` |
| `--read` | Ergebnis als Datenstrom (`sdk.Reader`), z. B. `list` mit allen Treffern. Ausgabe fortlaufend, Zusammenfassung auf stderr; mit `--param limit=…` endet sie mit der Marke für `--param after=…` |
| `--format` | Beispieldatei: `yaml` (Standard), `json` oder `csv`; mit `--read`: `csv` (Standard) oder `jsonl` |
| `--out` | Ergebnis in eine Datei statt auf die Standardausgabe |
| `--logout` | abmelden, gespeichertes Token löschen |

**Anmeldung:** Das Passwort kommt aus `COREMESH_PASSWORD` oder wird ohne Echo abgefragt.
Das Token liegt danach je Adresse im Benutzer-Konfigurationsverzeichnis
(`…/coremesh/console-<hash>.token`, Rechte `0600`). Ist es abgelaufen, meldet die CLI
sich einmal neu an.

**Ausgabe:** Meldungen gehen auf stderr, Daten auf stdout. Bei einem Fehler ist der
Exit-Code 1, mit lesbarer Meldung wie „keine Berechtigung: …“ oder „nicht gefunden: …“.

### Konsolenbefehle der Module

Module melden Befehle im Metamodell an (`ModuleDefinition.Commands`, im Modul per
`r.Command(…)`). Die CLI ruft sie als `<modul>:<befehl>` mit `--parameter=wert` auf:

```bash
console ledger:help                                   # Befehle des Moduls auflisten
console ledger:load-coa --chart=SKR04                 # mitgelieferten Kontenrahmen laden
console ledger:load-coa --chart=SKR25 --file=./skr25.csv
console ledger:setup-company --company=1000 --chart=SKR25 --currency=EUR --year=2026
```

- Das **Console-Plugin** löst den Befehl über `Catalog.GetModule` zu `Object.Action` auf.
  Es prüft Pflichtparameter und lehnt unbekannte Parameter ab. Danach läuft der Aufruf
  wie `--object/--action` mit den Rechten des angemeldeten Benutzers.
- **Typ der Parameter** (`CommandParam.Type`): `text` (Standard – `--company=2000` bleibt der
  Text "2000"), `number`, `boolean`, `json`. Die CLI wandelt die Werte nach dem Typ um; nicht
  deklarierte Parameter werden wie bisher gelesen (gültiges JSON als Wert, sonst Text).
- **Datei-Parameter** (`CommandParam.File`) liest die CLI lokal ein und sendet den Inhalt:
  - `*.json` geparst,
  - `*.csv` als Liste von Objekten (Kopfzeile = Feldnamen, Trenner `;` oder `,`,
    Excel-BOM wird entfernt),
  - sonst Text.
- CLI-Optionen (`--addr`, `--user`, `--out`, `--tls-ca`, `--timeout`) dürfen hinter dem
  Befehl stehen. Alle anderen `--name=wert` gehen als Parameter an den Befehl.

## Console-Plugin

```yaml
# configs/05-console.yaml
plugins:
  console:
    version: 0.1.0
    ingress: true                          # Pflicht: startet eigene Wurzelanfragen
    settings:
      listen: unix://./data/console.sock   # oder tcp://0.0.0.0:7443
      token_ttl: 8h
      max_extract_bytes: 1073741824        # 1 GiB entpackt
      max_extract_files: 10000
      blocked_directories: []              # zusätzlich gesperrte Ziele
      tls_cert: ./certs/console.crt        # nur tcp://, beide gesetzt = TLS
      tls_key: ./certs/console.key
```

**Unix-Socket (Standard):**
- Nur lokal erreichbar. Unter Linux und macOS hat die Socket-Datei die Rechte `0600`,
  nur der Benutzer des Host-Prozesses kann sich verbinden.
- Unter Windows 10 und neuer funktionieren Unix-Sockets ebenfalls (AF_UNIX).
- Ein verwaister Socket von einem früheren Lauf wird beim Start entfernt, eine normale
  Datei an dieser Stelle dagegen nie.

**TCP:** Für Zugriffe von anderen Rechnern. Ohne TLS warnt das Host-Log, weil Passwörter
und Tokens dann unverschlüsselt übertragen werden.

### Ablauf eines Aufrufs

1. **Anmelden:** `Login` prüft die Daten über `iam` (`Account.Authenticate`) und liefert
   ein zufälliges Token. Im Speicher steht nur dessen SHA-256. Nach 5 Fehlversuchen je
   Benutzer und Absender wird gesperrt, mit wachsender Dauer.
2. **Jeder weitere Aufruf** sendet `authorization: Bearer <token>`. Das Plugin fragt
   jedes Mal `Account.Me`, ein deaktivierter Benutzer ist also sofort draußen.
3. **Weiterreichen:** Der Aufruf geht über den Host, mit Benutzer und Mandant im
   `CallContext`. Der **Dispatcher prüft die Berechtigung** (`Object.Action`), Module können
   zusätzlich Buchungskreise prüfen (`sdk.CheckAccess`).

### SampleFile: Beispieldateien aus dem Metamodell

`SampleFile` holt `Catalog.GetDefinition` und erzeugt daraus eine Vorlage zum Anlegen
von Datensätzen. Sie enthält nur **bearbeitbare** Felder, in der Reihenfolge des
Metamodells.

| Typ | Beispielwert |
|---|---|
| `text`, `textarea` | `Beispiel <Label>` |
| `email` | `max.mustermann@example.com` |
| `number` | `100` |
| `date` | `"2026-01-01"` (in YAML in Anführungszeichen, damit es Text bleibt) |
| `select` | erster Wert aus `Options` |
| `boolean` | `true` |
| `password` | `Bitte-aendern-123` |

**YAML** ist das Standardformat und enthält Kommentare zu jedem Feld:

```yaml
# Beispieldatei für Geschäftspartner (BusinessPartner)
# Erzeugt aus dem Metamodell im Catalog; enthält alle bearbeitbaren Felder.
# Pflichtfelder: company_name, kind
# Firmenname
#   Typ: Text · Pflichtfeld
company_name: Beispiel Firmenname
# Art
#   Typ: Auswahl · Pflichtfeld
#   Erlaubte Werte: customer (Kunde), supplier (Lieferant)
kind: customer
```

- **JSON:** ein Objekt, die Felder in der Reihenfolge des Metamodells.
- **CSV:** Zeile 1 enthält die Feld-Keys, Zeile 2 die Beispielwerte.
- **Dateiname:** Die Antwort schlägt einen vor, z. B. `business_partner_sample.yaml`.

### Execute mit ZIP: Dateien auf dem Server entpacken

Liefert ein Modul in seiner Antwort `zip_content` und ist `target_directory` gesetzt
(Feld oder Parameter), entpackt das Console-Plugin das ZIP dort. `target_directory`
wird nie an das Modul weitergegeben, und `zip_content` fehlt in der Antwort an die CLI.

**Im Modul** genügt ein `[]byte`. Über das Plugin-Protokoll kommt es Base64-kodiert an,
das Console-Plugin dekodiert es:

```go
return sdk.Response{Payload: map[string]any{"zip_content": zipBytes, "files": n}}, nil
```

**Berechtigungen:**
- die Berechtigung für die aufgerufene Action (Dispatcher),
- zusätzlich **`Console.ExtractZip`** in `iam`, weil das Entpacken Dateien auf den Server
  schreibt. In `*.*` ist sie enthalten.

**Zielverzeichnis:**

| Regel | Umsetzung |
|---|---|
| absolut | relative Pfade werden abgelehnt (`InvalidArgument`) |
| keine Systemverzeichnisse | inklusive aller Unterverzeichnisse, siehe unten, plus `blocked_directories` (`PermissionDenied`) |
| kein Wurzelverzeichnis | `/`, `C:\` |
| Symlinks aufgelöst | vor der Prüfung, ein Link `/tmp/x → /etc` hilft also nicht |
| Schreibrechte | entscheidet das Betriebssystem für den Benutzer des Host-Prozesses |

**Gesperrte Systemverzeichnisse:**

| System | Verzeichnisse |
|---|---|
| Linux | `/bin`, `/sbin`, `/usr`, `/lib`, `/lib32`, `/lib64`, `/libx32`, `/boot`, `/dev`, `/proc`, `/sys`, `/etc`, `/run`, `/snap`, `/var/lib`, `/var/run` |
| macOS | `/System`, `/Library`, `/bin`, `/sbin`, `/usr`, `/etc`, `/private/etc`, `/private/var/db`, `/dev`, `/Applications`, `/cores`, `/Volumes` |
| Windows | `%SystemRoot%`, `%ProgramFiles%`, `%ProgramFiles(x86)%`, `%ProgramData%`, `\Boot`, `\Recovery`, `\System Volume Information`, `\$Recycle.Bin` (ohne Beachtung der Groß-/Kleinschreibung) |

**Einträge im ZIP** (Schutz vor Zip-Slip und ZIP-Bomben):

| Regel | Umsetzung |
|---|---|
| nur lokale Namen | `filepath.IsLocal`: keine absoluten Pfade, kein `..`, keine Laufwerke, keine Windows-Gerätenamen (`NUL`, `CON` …) |
| bleibt im Ziel | auch über Symlinks, die im Ziel schon existieren |
| nur Dateien und Verzeichnisse | Symlinks und Sonderdateien im ZIP werden abgelehnt, bestehende Symlinks nie überschrieben |
| Grenzen | `max_extract_files`, `max_extract_bytes`. Gezählt werden die tatsächlich entpackten Bytes, nicht die Angaben im ZIP. |
| erst prüfen, dann schreiben | Unsichere Einträge brechen ab, bevor die erste Datei geschrieben ist |

Die Antwort meldet Verzeichnis, Anzahl Dateien und Verzeichnisse sowie die Bytes. Das
Host-Log erhält einen Eintrag „ZIP entpackt“ mit Benutzer und Ziel.

## Bauen und testen

```bash
cd cmd/plugins/console
go test ./...
go build -o ../../../bin/plugins/co/console-0.1.0-windows-amd64.exe .
cd ../../..
go build -o bin/console.exe ./cmd/console
```

Die Tests decken ab:
- Anmeldung, Token und Abmelden,
- SampleFile in allen drei Formaten,
- ZIP: Zip-Slip-Varianten, Windows-Gerätenamen, Symlinks, gesperrte Systemverzeichnisse,
  Grenzen,
- `Execute` mit `zip_content` über eine echte gRPC-Verbindung.

Die Symlink-Tests brauchen unter Windows Administratorrechte oder den Entwicklermodus und
werden sonst übersprungen. Unter Linux laufen sie immer.

## Grenzen

- **Nachrichtengröße:** Zwischen CLI und Console-Plugin sind bis 64 MiB erlaubt. Auf dem
  Weg Modul → Host → Console gilt die gRPC-Grenze des Plugin-Protokolls von 4 MiB, ein ZIP
  darf dort also höchstens etwa 3 MiB groß sein (Base64). Für größere Bündel müsste
  das Plugin-Protokoll höhere Grenzen setzen oder Streaming nutzen.
- **Tokens im Speicher:** Nach einem Neustart des Plugins meldet sich die CLI automatisch
  neu an.
- **Teilweise entpackt:** Wird `max_extract_bytes` erst beim Schreiben überschritten, weil
  die Größenangabe im ZIP gelogen hat, bleiben die bis dahin geschriebenen Dateien liegen.
