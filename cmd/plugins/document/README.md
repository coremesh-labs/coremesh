# Plugin `document` – Dokumentverweise

Beliebig viele Dokumente an jedem Datensatz jedes Objects: Vertrag, AGB, Nachtrag,
Widerrufsbelehrung, Bestätigung, Angebot, Rechnung, Bescheid, Abrechnung, Schriftverkehr, Personalausweis …

- **Schnittstelle:** nur das Object `Documents` aus [`pkg/sdk/docservice`](../../../pkg/sdk/docservice/docservice.go)
  (`list`, `attach`, `update`, `detach`, `types`). Andere Plugins und der WebServer kennen
  keine Tabellen dieses Plugins. **Ein Dokumentenmanagementsystem ersetzt das Plugin**, indem
  ein Adapter-Plugin dasselbe Object mit denselben Payloads anbietet.
- **Ziel:** Object und fachlicher Schlüssel des Datensatzes, bei Zeitscheiben ohne Beginndatum.
  Gültig von/bis sagt, für welchen Zeitraum ein Dokument gilt (z. B. AGB-Fassung, Nachtrag ab).
- **Ablage:** zunächst Dateiname, Ablageort oder Link (Links öffnet die Oberfläche). Das
  Hochladen und Speichern der Dateien selbst folgt.
- **Rechte folgen dem Datensatz:** lesen = `<Object>.get` und `<Object>.read` im Buchungskreis
  des Datensatzes; anhängen, ändern, entfernen = `<Object>.update`.
- **Entfernen** setzt `removed_at`/`removed_by`; der Verweis bleibt zur Nachvollziehbarkeit.
- **Umschlüsselung:** Das Plugin abonniert `*.rekey` und stellt Verweise auf den neuen Schlüssel um.
- **Dokumentarten:** Katalog `DocumentKind` (Vorschlagswerte als Seeds).

In der Oberfläche schaltet ein Modul den Abschnitt mit
`metamodel.SectionDefinition{Key: "dokumente", Title: "Dokumente", Documents: true}` ein.

Tests: `go test ./...`
