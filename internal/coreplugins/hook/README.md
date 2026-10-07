# hook – Hook-Dispatcher (synchrone Erweiterungspunkte)

Das Core-Plugin `hook` verbindet Module **synchron**: Ein Modul definiert einen Hook und
ruft ihn an festen Punkten seiner Verarbeitung auf. Andere Module abonnieren ihn und
prüfen, ändern oder ergänzen – ohne dass der Besitzer sie kennt.

| | SystemEvent (`event`) | Hook (`hook`) |
|---|---|---|
| Zeitpunkt | nach dem Speichern | währenddessen |
| Ablauf | asynchron, nur Benachrichtigung | synchron, mit Antwort |
| Wirkung | Folgeaktionen | prüfen (Veto), Daten ändern, Meldungen |

## Phasen

| Phase | Zeitpunkt | Wirkung |
|---|---|---|
| `modify` | vor dem Speichern | Abonnenten liefern geänderte Daten (`ReturnData`); der nächste erhält den geänderten Stand |
| `check` | vor dem Speichern | Meldungen `E` brechen ab, `W` und `S` werden angezeigt |
| `commit` | nach dem Speichern | Folgeaktionen; Fehler können nichts mehr zurücknehmen und kommen als Warnung zurück |

Abonnenten werden nach **Priorität** (aufsteigend, Standard 100) aufgerufen – im Kontext
der auslösenden Anfrage (gleicher Benutzer, keine erneute Rechteprüfung), mit Zeitlimit
(`timeout_sec`, Standard 10 s). Ist ein Abonnent nicht erreichbar oder wirft er einen
Fehler, gilt `on_failure` des Hooks: `block` (wie eine Meldung E) oder `skip` (Warnung).

Zwischen Plugins gibt es keine gemeinsame Transaktion: `check` läuft beim Besitzer
typischerweise innerhalb seiner Transaktion – Abonnenten sollen dort nur lesen.

## Datenstruktur

```
Aufruf an <Callback>.onHook          Antwort
  hook    string   ledger.posting      return_data  JSON (nur modify; leer = unverändert)
  action  string   modify|check|commit messages     [{type E|W|S, id, text, field, source}]
  data    JSON (google.protobuf.Value)
```

`source` setzt der Dispatcher (`<plugin>/<Callback>`), `field` benennt optional das
betroffene Feld.

## SDK (`pkg/sdk/hook`)

```go
// Besitzer: beim Start anmelden, beim Verarbeiten aufrufen
hook.Define(ctx, env.Services, hook.Definition{Name: "ledger.posting", Owner: "ledger",
	Description: "Buchen eines Belegs", Phases: hook.AllPhases, Data: "…Aufbau…", OnFailure: hook.FailBlock})

res, err := hook.Call(ctx, env.Services, "ledger.posting", hook.PhaseCheck, data)
if err != nil { return err }
if err := res.Err(); err != nil { return err }  // Meldungen E → sdk.ErrInvalidArgument
res.DecodeData(&changed)                         // nach modify
res.Filter(hook.TypeWarning)                     // Warnungen anzeigen

// Abonnent: Route anmelden und abonnieren (wiederholbar, Sperre bleibt)
hook.Handle(r, "TaxCheck", m.onHook)             // module.Router; ohne Router: hook.Func
hook.Subscribe(ctx, env.Services, hook.Subscription{Hook: "ledger.posting",
	Phase: hook.PhaseCheck, Callback: "TaxCheck", Priority: 100})

func (m *Module) onHook(ctx context.Context, req hook.Request) (hook.Response, error) {
	var d MyData
	if err := req.DecodeData(&d); err != nil { return hook.Response{}, err }
	return hook.Reply(nil, hook.Error("TAX-1", "Steuerkennzeichen fehlt").OnField("tax_code")), nil
}
```

Ohne konfigurierten Hook-Dispatcher liefert `hook.Call` die Daten unverändert zurück.

## Actions

| Object.Action | Zweck |
|---|---|
| `Hook.Define {name, owner, description, phases, data, on_failure}` | Hook anmelden; Name `<modul>.<punkt>`, gehört genau einem Modul |
| `Hook.Subscribe {hook, phase, callback, priority, description}` | Abo anmelden; aufgerufen wird `<callback>.onHook` (feste Action) |
| `Hook.Call {hook, action, data}` | Abonnenten der Phase aufrufen → `{data, messages, subscribers}` |
| `Hook.list/get` | Übersicht (auch Hooks, die nur abonniert, aber nicht definiert sind) |
| `HookSubscription.list/get` | Abos mit Status aktiv / gesperrt / nicht erreichbar |
| `HookSubscription.lock/unlock` | Abo sperren bzw. entsperren – die Sperre bleibt über Neustarts erhalten |

## Oberfläche

**Erweiterungen → Hooks:** Hooks mit Beschreibung, Phasen, Aufbau der Daten und ihren
Abos; **Hook-Abos:** alle Abos, filterbar nach Hook und Phase, mit „Sperren“ /
„Entsperren“. Abos werden nicht gelöscht: Die Plugins melden sie bei jedem Start neu an.

## Tabellen

`hook__hooks` (Name, Besitzer, Beschreibung, Phasen, Aufbau, on_failure) und
`hook__subscriptions` (Hook, Phase, Callback, Plugin, Priorität, gesperrt von/am,
Zeitpunkte); Konfiguration `configs/02-hook-plugin.yaml`.

## Beispiel

Der Ledger definiert `ledger.posting` (siehe coremesh-erp). Das Beispiel-Plugin `hello`
abonniert `check` (Warnung ohne Referenz, Fehler bei „STOP“ im Kopftext) und `commit`
(Hinweis mit der Belegnummer).
