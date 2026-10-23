# coremesh-setup

Richtet CoreMesh-Systeme ein und stellt Stände zusammen. Läuft nur lokal (Oberfläche auf
127.0.0.1), ein Programm je Plattform, ohne weitere Abhängigkeiten.

## Begriffe

| Begriff | Bedeutung | Wo |
|---|---|---|
| Stand | unveränderliche Liste zusammengehörender Bausteine, Name nach Datum (`2026.10.23`, mehrere am Tag: `2026.10.23-2`) | GitHub-Release mit `manifest.json` |
| Baustein | eine Datei: Programm (host, console), Plugin (Name + Version), Dateien (configs, migrations, Inhalte, Lizenzen als ein Archiv), coremesh-setup | `manifest.json` → `components` |
| Linie | Zeiger auf einen Stand: `release` (freigegeben), `preview` (zum Ausprobieren) | `dist/index.json` |
| Ausprägung | Auswahl beim Einrichten, z. B. `erp-vuv` (ERP Vermietung und Verwaltung), optional mit Testbestand | `dist/pack.yaml`, `manifest.json` |
| Repository | Ablage der Dateien eines Stands (Vorlage mit `{release}` und `{file}`, `https://…` oder `file://…`) | `dist/index.json` |

Nur `dist/index.json` auf `master` legt fest, welcher Stand Release bzw. Preview ist; jeder
Stand nennt seinen Vorgänger (`previous`, Update-Pfad; `via` = Pflicht-Zwischenstand).

## Bausteine statt Archive

Ein Stand ist kein großes Archiv, sondern eine Liste von Bausteinen mit Version und Prüfsumme
(`manifest.json`, Format 2). Das Werkzeug weiß daraus, welche Plugins zusammengehören, und lädt
nur, was in der Installation fehlt:

- **Plugins** liegen für alle Stände gemeinsam in `plugins/<xx>/<name>-<version>-<plattform>`.
  Ein Plugin in derselben Version wird nie erneut geladen.
- **Programme** (host, console) mit gleichem Inhalt übernimmt das Werkzeug aus dem bisherigen Stand.
- **Dateien** (configs, migrations, Inhalte, Lizenzen) sind ein kleines Archiv (gut 100 KB).

Ein Update, das nur den Webserver ändert, lädt so etwa 12 MB statt 250 MB. Beim Veröffentlichen
gilt dasselbe: Bausteine, die schon ein früherer Stand enthält (Plugin in derselben Version,
Programm oder Dateien mit gleicher Prüfsumme), verweisen im Manifest auf dessen Datei
(`"release": "<früherer Stand>"`) und werden nicht noch einmal hochgeladen.

## Installation (Verzeichnis)

```
releases/<stand>/     Programme, configs, migrations, Inhalte, Lizenzen, manifest.json eines Stands
plugins/<xx>/         Plugins aller Stände (je Version einmal)
local/90-local.yaml   Einstellungen dieser Installation (Port, Pfade) – mischt sich über configs/
data/                 Datenbank, Log; data/setup-backup/ Sicherungen vor Updates
current               aktiver Stand
setup.json            Linie, Ausprägung, Port, vorheriger Stand und Sicherung (Rückfall)
ZUGANG.txt            Adresse, Benutzer, Passwort (nur für den Besitzer lesbar)
coremesh-setup        das Werkzeug selbst
start.sh stop.sh status.sh   (Windows: .cmd) Starten, Beenden, Zustand
```

Die Skripte rufen das mitgelieferte `coremesh-setup` auf. Der Host läuft im Hintergrund weiter,
auch wenn das Terminal geschlossen wird; `stop.sh` beendet ihn sauber.

Der Host läuft im Installationsverzeichnis mit `-config releases/<stand>/configs,local`.
Eigene Einstellungen gehören in eine weitere Datei in `local/` (z. B. `95-eigen.yaml`); sie
überlebt jedes Update.

## Befehle

```sh
coremesh-setup                         # Oberfläche (Assistent): einrichten, aktualisieren, zurück, starten/beenden
coremesh-setup list                    # Linien, Stände, Ausprägungen des Servers
coremesh-setup install --dir=D [--release=release|preview|<stand>] [--package=erp-vuv] [--data] [--port=8080]
coremesh-setup plan --dir=D [--release=…]      # was ein Update lädt, übernimmt, behält
coremesh-setup update --dir=D [--release=…]    # Standard: die Linie der Installation
coremesh-setup rollback --dir=D                # zurück auf den Stand vor dem letzten Update
coremesh-setup start|stop|status --dir=D
coremesh-setup seed --dir=D --file=einrichtung.yaml
coremesh-setup pack --labs=.. --release=<stand> [--build] [--data] [--out=…]
```

`--index` (bzw. `COREMESH_SETUP_INDEX`) wählt den Server: Standard ist `dist/index.json` auf
`master` bei GitHub; für lokal gebaute Stände ein Pfad zu einer eigenen `index.json` mit einem
Repository `file:///…/{release}/{file}`.

**Oberfläche** (`http://127.0.0.1:9870`): Übersicht der Installationen mit Hinweis auf Updates
ihrer Linie; neue Installation in vier Schritten (Verzeichnis und Port, Stand, Ausprägung mit
oder ohne Testbestand, Übersicht mit allen Bausteinen und der Download-Größe); Update in zwei
Schritten (Stand, Übersicht: was sich ändert, was zu laden ist); Rückfall mit Rückfrage.

**install** lädt die fehlenden Bausteine (und den Testbestand), prüft die SHA-256-Prüfsummen
aus `manifest.json`, schreibt `local/90-local.yaml`, die Skripte und startet. Ohne Testbestand
legt der erste Start den Benutzer `admin` mit einem zufälligen Passwort an. Mit Testbestand
(nur in ein Verzeichnis ohne Datenbank) ersetzt das Werkzeug das bekannte Passwort des
Bestands sofort durch ein zufälliges.

**update** lädt die Bausteine des neuen Stands, während der alte weiterläuft; dann beendet es
den Host, sichert die Datenbank nach `data/setup-backup/`, schaltet um und startet (der Host
passt die Datenbank beim Start an). Startet der neue Stand nicht, geht es von selbst zurück
(Stand und Datenbank). **rollback** macht das letzte Update rückgängig – Erfasstes seit dem
Update geht dabei verloren. Alte Stände bleiben in `releases/`, alte Plugin-Versionen in
`plugins/`.

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
die eigene Plattform zusammen: je Baustein eine Datei (Programme, genau die konfigurierten
Plugin-Versionen, gzip) und ein Archiv mit Konfiguration ohne lokale Suchpfade, Migrationen,
Berichten, Agenten-Leitfaden und Lizenzen. Gleiche Bausteine des vorherigen Stands (`--previous`,
sonst die Linie `preview` des Servers) werden nicht erneut abgelegt. Mit `--data` richtet
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
   Ältere Stände bleiben im Index: spätere Stände verweisen auf ihre Dateien.

Plugin-Versionen sind Kennungen: Wer ein Plugin ändert, erhöht seine Version. Sonst bleibt die
schon veröffentlichte Datei (pack meldet „Inhalt anders … Version erhöhen!“).

Stände bis `2026.10.23-2` sind ganze Archive (Manifest-Format 1); dieses Werkzeug richtet sie
nicht mehr ein.

Heute: linux-amd64. Windows und macOS (arm64) folgen über weitere Runner im Workflow.
