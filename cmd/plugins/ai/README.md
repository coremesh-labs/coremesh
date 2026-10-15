# KI (`ai`)

Anfragen an KI-Dienste mit OpenAI-kompatibler Schnittstelle (`/chat/completions`), z. B.
Google Gemini (`https://generativelanguage.googleapis.com/v1beta/openai`). Menü **KI**.

## KI-Dienste (`AiProvider`)

URL, Modell, Anmeldung (`BEARER`, eigener `HEADER`, `QUERY`-Parameter, `NONE`), weitere Header
(je Zeile `Name: Wert`), Zeitlimit. Der **API-Schlüssel** ist ein reines Schreibfeld: Er liegt in
einer eigenen Tabelle ohne Leseschnittstelle und erscheint weder in der Oberfläche noch in Listen,
Datenströmen oder Protokollen. Alternativ nennt das Feld **Umgebungsvariable** eine Variable aus
der Plugin-Konfiguration (`plugins.ai.env`); sie geht dem gespeicherten Schlüssel vor.
„Verbindung prüfen“ sendet eine kurze Testanfrage. Der Schlüssel lässt sich auch beim Ändern
neu eintragen (seit 0.3.2, Textfeld).

`Ai.run` nimmt Text und Datei zusammen (seit 0.3.0): Der Text steht im Auftrag, die Datei geht als
Anhang mit. Wie, steht am Dienst unter **Dateien senden als** (seit 0.3.3): `image_url` (Gemini,
auch PDF), `file` (OpenAI für PDF) oder automatisch (erst `image_url`, lehnt der Dienst die Art
ab, einmal `file`). Bilder gehen immer als `image_url`.

Bei Überlast (HTTP 429, 5xx) wiederholt das Plugin die Anfrage zweimal. Bleibt der Dienst gestört
(auch nicht erreichbar oder Zeitlimit), springt der **Ausweich-Dienst** ein (Feld am Dienst, mit
dessen eigenem Modell; höchstens drei Dienste hintereinander). Das Protokoll nennt den Dienst, der
geantwortet hat, die Meldung den Wechsel. Fehler der Anfrage selbst (z. B. falscher Schlüssel)
lösen keinen Wechsel aus. Die Schnittstelle nimmt ihre Felder auch unter `data` an (Konsole).

## Anfragen (`AiRequest`, `AiRequestObject`)

- **TEXT:** Aufgabe, weitere Textzeilen, Zusatzzeilen des Aufrufers und der Eingang – die Antwort
  ist Text.
- **OBJECT:** Die Zeilen nennen die Objects – die Wurzel, Unter-Objekte über eine Relation der
  Wurzel (Abschnitt, z. B. `adressen`) und eingebettete Objekte über ein Verweisfeld (z. B.
  `address_id` → Adressdaten). Aus dem Metamodell entstehen die leere Struktur, das JSON-Schema
  (`response_format`, mit `enum` für feste Werte) und die Liste der festen Werte („`addresses.role`
  kann diese Werte annehmen: Schlüssel = Beschreibung“); Verweise auf kleine Kataloge (höchstens
  `max_values` Einträge) werden zur Auswahl, große (z. B. Geschäftspartner) nennt die KI beim Namen.
  Die Antwort wird geprüft: unbekannte Felder und Werte außerhalb der Auswahl entfallen (mit Hinweis).

Jede Anfrage läuft ohne Gedächtnis (neuer Kontext). „Probeanfrage …“ testet mit der Beispieleingabe,
„Struktur anzeigen“ zeigt leere Struktur, Werte und Schema.

## Protokoll (`AiLog`)

Eingang, Zusatzzeilen, gesendete Nachrichten (ohne Dateiinhalt), Antwort und Ergebnis. Verfall:
Tagesdatum plus die Tage der Anfrage (Standard 1), außer „Nicht löschen“. „Wiederholen“ sendet
denselben Eingang mit der aktuellen Fassung der Anfrage.

## Schnittstelle (`Ai`)

| Action | Payload | Ergebnis |
|---|---|---|
| `run` | `request`, `input` (Base64) mit `content_type` oder `text`, `context` (Zeilen), `company_code`, `keep` | `log_id`, `status` (`DONE`/`ERROR`), `message`, `result` (OBJECT) bzw. `text` |
| `form` | `request`, `result`, `defaults` (`"name.feld": Wert`) | `fields` (Felder einer Eingabemaske, `f__…`), `values` |
| `collect` | `request`, `values` (Werte der Maske) | `result` (Struktur, leere Zeilen entfallen) |
| `create` | `request`, `result`, `defaults` | `object`, `id`, `record` – eingebettete Objekte zuerst, dann `saveAggregate` der Wurzel mit den Relationen |
| `schema` | `request` | `schema`, `empty`, `values` |

Recht: `AiRequest.run` (Rollenpflege). Ein Fehler des Dienstes ist kein Fehler des Aufrufs:
`status = ERROR` mit Meldung – der Aufrufer kann dann ohne Vorbelegung weitermachen.
Eingang: Text (z. B. die Texterkennung des DMS) oder Bilder; PDF besser als erkannter Text.
