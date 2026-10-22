# coremesh-setup

Richtet CoreMesh-Systeme ein und stellt Stände zusammen. Läuft nur lokal (Oberfläche auf
127.0.0.1), ein Programm je Plattform, ohne weitere Abhängigkeiten.

## Begriffe

| Begriff | Bedeutung | Wo |
|---|---|---|
| Stand | unveränderliche Zusammenstellung aller Programme und Plugins, Name nach Datum (`2026.10.23`, mehrere am Tag: `2026.10.23-2`) | GitHub-Release mit `manifest.json` |
| Linie | Zeiger auf einen Stand: `release` (freigegeben), `preview` (zum Ausprobieren) | `dist/index.json` |
| Ausprägung | Auswahl beim Einrichten, z. B. `erp-vuv` (ERP Vermietung und Verwaltung), optional mit Testbestand | `dist/pack.yaml`, `manifest.json` |
| Repository | Ablage der Dateien eines Stands (Vorlage mit `{release}` und `{file}`, `https://…` oder `file://…`) | `dist/index.json` |

Nur `dist/index.json` auf `master` legt fest, welcher Stand Release bzw. Preview ist; jeder
Stand nennt seinen Vorgänger (`previous`, Update-Pfad; `via` = Pflicht-Zwischenstand).

## Installation (Verzeichnis)

```
releases/<stand>/     Programme, Plugins, configs, migrations, Inhalte, Lizenzen (wird bei Updates ersetzt)
local/90-local.yaml   Einstellungen dieser Installation (Port, Pfade) – mischt sich über configs/
data/                 Datenbank, Log, Sicherungen
current               aktiver Stand
ZUGANG.txt            Adresse, Benutzer, Passwort (nur für den Besitzer lesbar)
```

Der Host läuft im Installationsverzeichnis mit `-config releases/<stand>/configs,local`.
Eigene Einstellungen gehören in eine weitere Datei in `local/` (z. B. `95-eigen.yaml`); sie
überlebt jedes Update.

## Befehle

```sh
coremesh-setup                         # Oberfläche: Installationen, Starten/Beenden, Einrichten
coremesh-setup list                    # Linien, Stände, Ausprägungen des Servers
coremesh-setup install --dir=D [--release=release|preview|<stand>] [--package=erp-vuv] [--data] [--port=8080]
coremesh-setup start|stop|status --dir=D
coremesh-setup seed --dir=D --file=einrichtung.yaml
coremesh-setup pack --labs=.. --release=<stand> [--build] [--data] [--out=…]
```

`--index` (bzw. `COREMESH_SETUP_INDEX`) wählt den Server: Standard ist `dist/index.json` auf
`master` bei GitHub; für lokal gebaute Stände ein Pfad zu einer eigenen `index.json` mit einem
Repository `file:///…/{release}/{file}`.

**install** lädt Archiv (und Testbestand), prüft die SHA-256-Prüfsummen aus `manifest.json`,
entpackt nach `releases/<stand>`, schreibt `local/90-local.yaml` und startet. Ohne Testbestand
legt der erste Start den Benutzer `admin` mit einem zufälligen Passwort an. Mit Testbestand
(nur in ein Verzeichnis ohne Datenbank) ersetzt das Werkzeug das bekannte Passwort des
Bestands sofort durch ein zufälliges.

**seed** spielt eine Einrichtungsdatei über die Konsole ein – dieselben Fachobjekte und
Aktionen wie in der Weboberfläche, nie direkt in die Datenbank:

```yaml
title: Testbestand …
steps:
  - title: Hauptbuch                      # mit title: Meldung ausgeben
    command: ledger:setup-company         # Konsolenbefehl eines Moduls
    args: {company: "1000", chart: SKR04}
  - handle: BusinessPartner.create        # Aktion eines Objects
    as: faust                             # Antwort merken …
    data: {type: PERSON, name1: Faust}
  - handle: PartnerRole.create
    unless: "…"                           # überspringen, wenn <Object>.get mit dieser ID gelingt
    data: {bp_id: "${faust.id}", role_code: OWNER}   # … und einsetzen
```

Werte mit `./` oder `../` sind Dateien relativ zur Einrichtungsdatei. Kurz belegte Datenbank
(„database is locked“) wiederholt den Schritt.

**pack** stellt aus den nebeneinander liegenden Repositories (`dist/pack.yaml`) einen Stand für
die eigene Plattform zusammen: Programme, genau die konfigurierten Plugin-Versionen, Konfiguration
ohne lokale Suchpfade, Migrationen, Berichte, Agenten-Leitfaden, Lizenzen. Mit `--data` richtet
es je Ausprägung mit Datenbestand eine frische Installation ein, spielt die Einrichtungsdatei ein
(z. B. `coremesh-erp/testdata/vuv-faust.yaml`) und legt die Datenbank komprimiert daneben.
Ein vorhandenes `manifest.json` wird um die Plattform ergänzt.

## Veröffentlichen

1. `feature` → `release` → `master` in allen drei Repositories (Release-Stand), Tests grün.
2. Gleiches Tag in allen drei Repositories, zuerst coremesh-erp und coremesh-erph pushen, dann
   coremesh: `git tag 2026.10.23 && git push origin 2026.10.23`.
3. Der Workflow `.github/workflows/release.yml` baut, testet, erzeugt die Testbestände und legt
   das GitHub-Release an.
4. `dist/index.json` auf `master`: Stand eintragen (`previous` = Vorgänger) und Linie(n) setzen.

Heute: linux-amd64. Windows und macOS (arm64) folgen über weitere Runner im Workflow.
