# Core-Plugin `backup` – Datensicherung

Snapshots der Datenbank in einstellbarem Abstand, im laufenden Betrieb. Oberfläche:
**Datensicherung** (`/m/backup`), dort auch **Jetzt sichern** und **Datenbank verdichten**.

| Datenbank | Verfahren | Datei | Wiederherstellen |
|---|---|---|---|
| SQLite | `VACUUM INTO` – konsistente Kopie, Schreiber warten nicht | `coremesh-JJJJMMTT-HHMMSS.db` | Server stoppen, Datei als `coremesh.db` einsetzen (`-wal`/`-shm` entfernen) |
| PostgreSQL | `pg_dump --format=custom`; Passwort als `PGPASSWORD`, nicht auf der Kommandozeile | `coremesh-JJJJMMTT-HHMMSS.dump` | `pg_restore --clean --dbname=…` |

Die Datei entsteht zuerst als `….partial` und wird erst nach Erfolg umbenannt.

## Einstellungen (BackupSetting)

Vorgaben beim ersten Start aus `configs/02-backup-plugin.yaml`, danach gilt die Oberfläche.

| Feld | Bedeutung |
|---|---|
| Verzeichnis | Ziel der Snapshots, relativ zum Arbeitsverzeichnis des Servers oder absolut; wird angelegt |
| Abstand (Stunden) | frühestens so lange nach dem letzten Snapshot der nächste |
| Nur bei Änderungen | nur sichern, wenn seit dem letzten Snapshot etwas geändert wurde – so entsteht ein Snapshot je **Arbeitstag**, an freien Tagen keiner |
| Ohne Tabellen | Tabellen, die sich auch ohne Arbeit ändern (eigenes Protokoll, Sitzungen): Muster mit `*`, Komma-Liste; Vorgabe `backup__*, webserver__sessions` |
| Verweildauer (Tage) | ältere Snapshots werden gelöscht; `0` = unbegrenzt |
| Höchstens | nie mehr Snapshots als diese Zahl; `0` = beliebig viele |
| Automatisch sichern | Zeitgeber an/aus (Jetzt sichern geht immer) |

Gelöscht werden nur eigene, erfolgreiche Snapshots aus dem Protokoll; **der jüngste
bleibt immer**, auch nach langer Pause. Fremde Dateien im Verzeichnis bleiben unberührt.

## Datenbank verdichten (seit 0.2.0)

SQLite gibt Platz gelöschter Daten (z. B. abgelaufene Druckaufträge) nicht an die Platte
zurück, sondern verwendet ihn wieder. **Datenbank verdichten** baut die Datei neu (`VACUUM`)
und leert danach den WAL; die Meldung nennt die Größe vorher und nachher. Die Datenbank ist
dabei kurz gesperrt (Sekunden). PostgreSQL verwaltet freien Platz selbst (autovacuum) – dort
meldet die Aktion nur einen Hinweis.

## Änderungen erkennen

- **SQLite:** Prüfsumme über alle Zeilen außer den Tabellen unter „Ohne Tabellen“. Neu
  gerechnet wird nur, wenn Datei oder WAL jünger sind als beim letzten Vergleich.
- **PostgreSQL:** Zahl der eingefügten, geänderten und gelöschten Zeilen aus
  `pg_stat_user_tables` (ohne „Ohne Tabellen“) und Startzeit des Servers. Nach einem
  Neustart des PostgreSQL-Servers gilt die Datenbank einmal als geändert.

## Zeitgeber

Zwei Minuten nach dem Start, dann jede Minute: fällig, wenn aktiv, der Abstand erreicht
ist und (falls eingestellt) sich etwas geändert hat. Ein Snapshot zur Zeit.

## Protokoll (BackupSnapshot)

Jeder Snapshot mit Zeitpunkt, Datei, Größe, Dauer, Auslöser (automatisch / von Hand),
Benutzer, Ergebnis und Meldung; gelöschte stehen mit Grund (Verweildauer, Höchstanzahl).
