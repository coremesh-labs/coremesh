# Core-Plugin `numrange` – Nummernkreise

Fortlaufende Nummern für alle Module: Verträge, Belege, Abrechnungen … – ohne
Doppelvergabe, auch bei gleichzeitigen Anfragen. Schnittstelle für Module:
[`pkg/sdk/numrange`](../../../pkg/sdk/numrange/numrange.go). Oberfläche:
**Nummernkreise** (`/m/numranges`).

## Begriffe

| Begriff | Bedeutung | Wer legt fest |
|---|---|---|
| **Objekt** (`NumberRangeObject`) | Nummernkreis eines Moduls, z. B. `Contract`: gilt je Buchungskreis und/oder je Jahr, mit Standardwerten | das Modul (`Define`, bei jedem Start) |
| **Intervall** (`NumberRange`) | je Objekt, Buchungskreis (`*` = alle), Intervallschlüssel und Jahr (`0` = ohne): von/bis, Stand, Stellenzahl, Format, Überlauf, Warnschwelle | der Administrator; fehlende legt der erste Abruf an |
| **Protokoll** (`NumberRangeLog`) | jede vergebene Nummer mit Zeitpunkt, Benutzer, Request und Referenz | automatisch |

**Welches Intervall ein Vorgang nutzt**, legt das Modul in seinen Katalogen fest –
z. B. die Vertragsart mit ihrem Intervallschlüssel `MV`. So bleibt `numrange` allgemein
(wie in SAP die Belegart auf ihren Nummernkreis verweist).

## Einstellungen eines Intervalls

| Feld | Bedeutung |
|---|---|
| von / bis | Bereich der laufenden Nummer (von ≥ 1, höchstens 15 Stellen) |
| Stand | zuletzt vergebene Nummer (`0` = noch keine); der Administrator kann ihn setzen, z. B. bei Übernahme aus einem Altsystem |
| Stellenzahl | `{N}` wird mit Nullen auf diese Länge aufgefüllt; `0` = ohne; `bis` muss hineinpassen |
| Format | Platzhalter `{KEY}` (Schlüssel), `{CC}` (Buchungskreis), `{YYYY}`, `{YY}` (Jahr), `{N}` (Nummer, genau einmal) – z. B. `{KEY}-{YYYY}-{N}` → `MV-2026-0001` |
| Bei Überlauf | `ERROR` Fehler, nichts wird vergeben · `RESTART` wieder bei *von* (nur, wenn alte Nummern frei sind) · `NEXT` im Folgeintervall (anderer Schlüssel) weiter |
| Warnschwelle | ab dieser Belegung in % liefert jeder Abruf eine Warnung; `0` = keine |

## Verhalten

- **Suche:** Intervall des Buchungskreises, sonst das für alle (`*`).
- **Fehlt das Intervall**, entsteht es beim ersten Abruf: bei Jahres-Nummernkreisen
  aus dem jüngsten Vorjahr desselben Schlüssels (alle Einstellungen, Stand 0), sonst aus
  den Standardwerten des Objekts. Ein neues Jahr braucht so keinen Handgriff.
- **Inaktive** Intervalle vergeben nichts (Fehler statt Neuanlage).
- **Gleichzeitige Abrufe:** Die Vergabe läuft serialisiert in einer eigenen
  Transaktion; der Stand ändert sich nur, wenn er noch der gelesene ist (bedingtes
  UPDATE) – auch über Prozesse hinweg keine Doppelvergabe.
- **Lücken:** Scheitert das Speichern im Modul nach der Vergabe, bleibt eine Lücke. Das
  Protokoll erklärt sie (wer, wann, Request, Referenz).

## Externe Vergabe (seit 0.3.0)

Je **Intervall** einstellbar (Gruppe „Vergabe“): Bei externer Vergabe bringt der Aufrufer die
Nummer mit (`Assign` mit `value`). numrange prüft sie
- am **erlaubten Muster** (regulärer Ausdruck, immer ganz, z. B. `[A-Z][A-Z0-9-]{2,11}`), sonst
- als **Zahl im Bereich** von–bis (formatiert mit Stellenzahl und Format),

und protokolliert sie. Ob sie frei ist, prüft das Modul an seinem Schlüssel. `Next` auf ein
externes Intervall scheitert; `Assign` ohne Wert auf ein internes vergibt wie `Next`.
Folgejahre übernehmen die Einstellung.

## Optionen des Objekts

| Option | Bedeutung |
|---|---|
| **lückenlos** (`gap_free`) | Die Nummer wird in der **Transaktion des Aufrufers** gezogen (`sdk.InTx`, gleiche Datenbank); ohne Transaktion lehnt `Next` ab. Rollt der Aufrufer zurück, ist die Nummer wieder frei – keine Lücke, z. B. für Belegnummern. Gleichzeitige Buchungen warten aufeinander |
| **Intervalle überschneidungsfrei** (`disjoint`) | Intervalle verschiedener Schlüssel desselben Objekts, Buchungskreises (bzw. `*`) und Jahres dürfen sich nicht überlappen – beim Anlegen, Ändern und bei der automatischen Neuanlage. Eine Nummer ist dann im Buchungskreis und Jahr eindeutig, gleich aus welchem Intervall |

## Aufrufe

| Aufruf | Payload → Antwort | Wer |
|---|---|---|
| `NumberRange.Define` | `numrange.Definition` → `{object}` | Module beim Start |
| `NumberRange.Next` | `{object, company_code, key, year, reference}` → `{number, value, interval, warning}` | Module (verschachtelt ohne Rechteprüfung); direkt nur mit Recht `NumberRange.Next` |
| `NumberRange.Assign` | `{object, company_code, key, year, reference, value}` → wie `Next`, dazu `external` | Module: intern (Wert leer) die nächste Nummer, extern die eingegebene, geprüft |
| `NumberRange.Info` | wie `Next` → `{interval, exists, external, external_pattern, from, to, active}` | Masken (Nummer eingeben oder nicht) |
| `NumberRange.list/get/create/update/deactivate` | Pflege der Intervalle | Administration |
| `NumberRangeObject.list/get`, `NumberRangeLog.list/get` | Übersicht | Administration |

```go
numrange.Define(ctx, env.Services, numrange.Definition{Object: "Contract", Owner: "contract",
	Description: "Vertragsnummern", PerCompanyCode: true, PerYear: true,
	Pattern: "{KEY}-{YYYY}-{N}", Width: 4})

// lückenlos: Next nur innerhalb von sdk.InTx
numrange.Define(ctx, env.Services, numrange.Definition{Object: "JournalEntry", Owner: "ledger",
	PerCompanyCode: true, PerYear: true, Width: 10, GapFree: true, Disjoint: true})

res, err := numrange.Next(ctx, env.Services, numrange.Request{Object: "Contract",
	CompanyCode: "1000", Key: "MV", Year: 2026, Reference: "Mietvertrag Müller"})
```

Tabellen: `numrange__object`, `numrange__interval`, `numrange__log`.
Konfiguration: `configs/02-numrange-plugin.yaml`.
