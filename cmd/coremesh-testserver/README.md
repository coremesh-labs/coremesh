# coremesh-testserver

Webserver für Tests im lokalen Netz – jetzt für die Ablage der PWA (Export), später für alles,
was das Handy über HTTPS braucht.

```
make testserver
bin/coremesh-testserver --dir=testserver
```

| Schalter | Vorgabe | Bedeutung |
|---|---|---|
| `--dir` | `testserver` | Verzeichnis für Serverzertifikat, Dateien, App |
| `--ca` | `~/.config/coremesh/ca` | Zertifizierungsstelle (gemeinsam) |
| `--http` | `:8080` | Einrichtung und CA-Zertifikat (HTTP), sonst Umleitung auf HTTPS |
| `--https` | `:8443` | Startseite, Dateien, App |
| `--name` | | weitere Namen bzw. Adressen fürs Zertifikat (Komma) |
| `--label` | `Testserver` | Name der CA: „CoreMesh <label> CA“ |
| `--open` | aus | Schreiben aus dem Netz ohne Upload-Schlüssel |
| `--max-upload` | `1024` | größter Upload in MB |

## Handy einrichten

1. Der Server zeigt beim Start einen QR-Code im Terminal; dieselben Codes stehen am Rechner auf
   `http://localhost:8080/`.
2. Scannen → Einrichtungsseite (HTTP) → „Zertifikat herunterladen“.
3. Android: Einstellungen › … › Verschlüsselung und Anmeldedaten › Zertifikat installieren ›
   CA-Zertifikat (Suche „CA-Zertifikat“), Fingerabdruck vergleichen.
4. „Weiter zur sicheren Seite“ – ohne Warnung, wenn es geklappt hat.
5. Zum Hochladen vom Handy: am Rechner den QR-Code „Handy mit Schreibrecht“ scannen (setzt den
   Upload-Schlüssel als Cookie).

## Zertifikate

- CA (`ca.crt`, `ca.key`) am festen Ort je Benutzer: `~/.config/coremesh/ca` (Schalter `--ca`,
  Umgebung `COREMESH_CA_DIR`). Alle Testserver – und später das Setup-Werkzeug – nutzen dieselbe;
  das Handy wird nur einmal eingerichtet. Eine CA aus `certs/` eines älteren Testservers wird beim
  Start dorthin übernommen. **Das Verzeichnis sichern** – ohne `ca.key` ist jedes Gerät neu
  einzurichten.
- Eigene CA (ECDSA P-256, 10 Jahre), per Name Constraints
  beschränkt auf `localhost`, `*.local`, den Rechnernamen, die Suchdomäne des Netzes (z. B.
  `fritz.box`), weitere Namen aus `--name` und private Adressbereiche. Sie kann keine fremde
  Seite ausstellen.
- `certs/server.crt`: Serverzertifikat (397 Tage) für Rechnername, `rechnername.local`,
  `rechnername.<suchdomäne>`, `localhost`, `127.0.0.1` und die privaten IPv4-Adressen.
- Alle 30 Sekunden prüft der Server das Netz. Neue Adresse (anderes WLAN) oder Ablauf in 30
  Tagen → neues Serverzertifikat mit derselben CA; am Handy ist nichts neu einzurichten.
- Paket `pkg/localca` – dasselbe soll später das Setup-Werkzeug für HTTPS der Installationen nutzen.

## Dateien: `/files/<pfad>`

| Anfrage | Wirkung |
|---|---|
| `GET` | Datei (Range, HEAD; `?download=1` als Anhang); Verzeichnis als Seite bzw. JSON (`?format=json`, `&recursive=1`) |
| `PUT` | Datei schreiben (atomar), Antwort `{path, size, sha256}` |
| `DELETE` | löschen (`?recursive=1` mit Inhalt) |
| `POST` (Verzeichnis) | Formular-Upload (Feld `file`, mehrfach, `unzip=1` entpackt zip), `op=mkdir`, `op=delete` |

- Lesen darf jeder im Netz; Schreiben am Rechner selbst immer, sonst mit Upload-Schlüssel
  (`Authorization: Bearer …`, `X-Upload-Token`, `?token=` oder Cookie).
- CORS offen; `o/…` und `m/…` mit `immutable`, alles andere `no-cache` (wie die Ablage der PWA).

## App: `/app/`

Gebaute Web-App als zip auf der Startseite hochladen (ein gemeinsamer Ordner wie `dist/` wird
zur Wurzel) oder nach `app/` legen. Pfade ohne Datei liefern `index.html`.
