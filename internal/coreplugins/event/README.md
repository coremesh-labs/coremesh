# Core-Plugin `event` – Event-Dispatcher

Der Event-Dispatcher verteilt Änderungen an Datensätzen zwischen Plugins, ohne dass sich
Sender und Empfänger kennen. Er läuft im Host-Prozess (`kind: internal`) und bietet das
Object **`SystemEvent`** an.

| Action | Payload | Wirkung |
|---|---|---|
| `Register` | `{object, action, company_code, callback}` | Abonnement: Events zu Object/Action/Buchungskreis (`*` = alle; fehlende Action und fehlender Buchungskreis = `*`) gehen an `<callback>.onEvent` |
| `Push` | `{object, action, company_code, entity_id, source, data}` | Änderung melden. Der Dispatcher ergänzt `id`, `occurred_at`, `tenant_id`, `user_id`, `request_id` und stellt an alle passenden Abonnements zu. Antwort `{event_id, subscribers}` |
| `List` | – | aktuelle Abonnements (Diagnose) |

Go-Schnittstelle für Plugins: [`pkg/sdk/events`](../../../pkg/sdk/events/events.go)
(`events.Register`, `events.Push`, `events.Decode`).

## Ablauf

```
Modul (z. B. ledger)                    event (Host)                        Empfänger (z. B. hello)
  Buchung schreiben, Commit
  events.Push ── SystemEvent.Push ──▶  passende Abonnements suchen
                 ◀── {event_id} ──────  in die Warteschlange stellen
                                        Worker: eigene Systemanfrage ──▶  EventLog.onEvent {event: {…}}
```

- **Asynchron:** `Push` kehrt sofort zurück. Ein Worker-Pool stellt die Events aus einer
  Warteschlange im Speicher zu. Der Sender bleibt dadurch unabhängig von Laufzeit und
  Fehlern der Empfänger.
- **Systemanfrage:** Jede Zustellung ist eine eigene Wurzelanfrage:
  - ohne Benutzer, deshalb braucht das Plugin `ingress: true`,
  - mit dem Mandanten des Auslösers und den Metadaten `ingress=event` und `event_id`,
  - der auslösende Benutzer steht als Information in `event.user_id`.
- **Fester Callback:** Zugestellt wird ausschließlich an die Action `onEvent`. Ein
  Abonnement kann so keine beliebigen Actions mit Systemrechten auslösen.
- **Abonnieren beim Start:** Plugins melden sich in `Configure` bzw. `Initialize` an. Ihre
  Routen kennt der Host erst danach. Fehlt die `onEvent`-Route noch, gilt das Abonnement
  trotzdem (`pending: true`). Gleiche Abonnements werden nicht doppelt gespeichert,
  wiederholtes Registrieren nach einem Neustart ist unschädlich.
- **Zuverlässigkeit:**
  - at most once, ohne Persistenz.
  - Ist der Empfänger gerade nicht verfügbar (Unavailable/Unimplemented, z. B. beim
    Neustart), wird mit wachsendem Abstand wiederholt, bis `max_attempts`.
  - Fachliche Fehler des Empfängers werden nicht wiederholt, nur protokolliert.
  - Ist die Warteschlange voll, wird das Event verworfen (Log-Warnung).
  - Beim Herunterfahren stellt der Dispatcher die Warteschlange zu Ende zu.
- **Rechte:**
  - `Push` und `Register` kommen von Plugins als verschachtelte Aufrufe und werden nicht
    erneut autorisiert.
  - Als Wurzelanfrage, z. B. per Console, braucht der Benutzer `SystemEvent.<Action>`.
  - `SystemEvent` ist ein reserviertes Object, kein Modul darf es belegen.

## Konfiguration

```yaml
# configs/02-event-plugin.yaml
plugins:
  event:
    kind: internal
    ingress: true
    settings:
      workers: 4          # parallele Zustellungen
      queue_size: 1000
      max_attempts: 3
      timeout_sec: 30
```

## Events senden

**Bewegungsdaten** melden jede Änderung.

- **Generisch über `pkg/sdk/crud`:** Eine Entity mit `Events: true` meldet nach `create`,
  `update`, `expire` und `deactivate` ein Event. Der Buchungskreis kommt aus
  `company_code_id` bzw. `company_code`, sonst aus `CompanyCodeField`. Eingeschaltet wird
  das im Modul mit `set.Events(env.Services, "<modul>")`.
- **Explizit:** fachliche Vorgänge mit `events.Push` **nach dem Commit**, zum Beispiel das
  Hauptbuch in coremesh-erp: `JournalEntry.post`, `JournalEntry.reverse`,
  `JournalDraft.post`.

Ist kein Event-Dispatcher konfiguriert, ist `events.Push` wirkungslos.

## Events empfangen

```go
// Route anmelden …
r.Object("RentContract").Handle(events.CallbackAction, m.onEvent)
// … und in Initialize abonnieren
events.Register(ctx, env.Services, events.Subscription{Object: "JournalEntry", Action: "post", CompanyCode: "*", Callback: "RentContract"})

func (m *Module) onEvent(ctx context.Context, req sdk.Request) (sdk.Response, error) {
    ev, err := events.Decode(req.Payload) // ev.Object, ev.Action, ev.EntityID, ev.Data …
    …
}
```

Beispiel im Repository: `examples/plugins/hello` abonniert alle Events (`EventLog`).
`console --object EventLog --action list` zeigt die zuletzt empfangenen.

## Grenzen und nächste Schritte

- **Outbox:** Events in der Transaktion des Senders speichern und danach zustellen. Damit
  geht bei einem Absturz zwischen Commit und Push nichts verloren (at least once).
- **Persistente Abonnements** und Dead-Letter-Liste für nicht zustellbare Events.
- Filter auf Inhalte (`data`) und Reihenfolgegarantie je Datensatz.
