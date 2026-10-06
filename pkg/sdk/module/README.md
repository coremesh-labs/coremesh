# Modul-Schicht (`pkg/sdk/module`)

Ein **Modul** bündelt fachlich zusammengehörige Business-Objects, ihre Logik und ihre
Endpunkte unter einem eigenen Namensraum. Es ist die Einheit, die der WebServer
registriert: Jedes Modul erscheint unter `/m/<modul>` (Oberfläche) und
`/api/v1/<modul>` (JSON).

```
Plugin (Prozess, DB-Präfix)          partner              → Host-Config, DBSchema: partner__*
└── Modul (Namensraum, Navigation)   businesspartner      → /m/businesspartner, /api/v1/businesspartner
    └── Business-Objects             BusinessPartner, PartnerRole, … (Gruppen „Partnerdaten“, „Kataloge“)
        └── Actions                  list, get, create, update, delete
```

## Die Schnittstelle

```go
type Module interface {
    Descriptor() Descriptor                         // Name (Namensraum), Title, Icon, Description
    RegisterRoutes(r *Router)                       // Objects, Actions, Metamodelle (gekapseltes Sub-Routing)
    Initialize(ctx context.Context, env Env) error  // Ressourcen übernehmen (Dependency Injection)
    Shutdown(ctx context.Context) error             // Ressourcen freigeben
}

type SchemaProvider interface { Schema() Schema }   // optional: Tabellen (Atlas-HCL) und Seeds
type Base struct{}                                  // einbetten: Initialize/Shutdown ohne Wirkung
```

`module.NewPlugin(info, modules...)` macht daraus ein `sdk.Plugin`. Das Plugin übernimmt
alles, was keine Fachlogik ist:

| Aufgabe | Woraus |
|---|---|
| Manifest (Capabilities) | registrierte Routen aller Module |
| Routing `(Object, Action)` → Handler | Router |
| `DBSchema.Init` | `SchemaProvider` der Module, zu einem Schema zusammengefasst |
| `Catalog.Describe` | Metamodelle (`Describe`) und Module (`Descriptor` + Reihenfolge, `Section`) |
| `getAggregate`, `saveAggregate` | Relationen im Metamodell, siehe [Aggregate](#aggregate-master-detail) |
| Dependency Injection | `Env` je Modul in `Configure` |
| Lebenszyklus | `Initialize` in Registrierungsreihenfolge, `Shutdown` rückwärts |

## Lebenszyklus

| Schritt | Wann | Was das Modul tut |
|---|---|---|
| `RegisterRoutes` | in `NewPlugin`, vor dem Start | rein deklarativ: Objects und Actions anmelden |
| `Initialize` | beim Start (`Configure`), vor der ersten Anfrage | `Env` speichern, Konfiguration lesen, prüfen |
| Handler | nebenläufig, eine Goroutine je Anfrage | Fachlogik |
| `Shutdown` | der Host beendet das Plugin | Hintergrundarbeit stoppen (Zeitlimit ca. 1,5 s) |

`RegisterRoutes` läuft **vor** `Initialize`, weil der Host die Routen schon für das
Manifest braucht, also bevor er dem Plugin Ressourcen gibt. Handler sind deshalb
Methoden des Moduls und greifen erst zur Laufzeit auf die in `Initialize` gesetzten
Felder zu.

Scheitert ein `Initialize`, beendet das Plugin die bereits initialisierten Module wieder,
und der Host startet das Plugin nicht. Fehler in `RegisterRoutes` (doppeltes Object,
ungültiger Name, reserviertes Object `Catalog`/`DBSchema`) meldet `Plugin.Err()`; dann
scheitert schon das Manifest.

## Dependency Injection: `Env`

| Feld | Inhalt |
|---|---|
| `Log` | `*slog.Logger` in das Log des Hosts, mit dem Feld `module`. `InfoContext(ctx, …)` ergänzt die `request_id`. |
| `DB` | `Query`, `Exec`, `InTx` auf der Datenbank des Moduls (Setting `database`, Standard `main`) |
| `Services` | `Call(ctx, object, action, payload)`: Actions anderer Module über den Dispatcher |
| `Config(&v)` | `settings.modules.<modul>` aus der Host-Config |
| `Plugin`, `Version`, `Module` | Identität |

```yaml
plugins:
  partner:
    version: 0.2.0
    databases:
      main: { access: write }        # Rechte vergibt weiterhin der Host
    settings:
      database: main                 # Standard für alle Module dieses Plugins
      modules:
        businesspartner:
          database: main             # optional pro Modul
          default_country: CH        # → env.Config(&cfg)
```

`DB` und `Services` funktionieren auch in eigenen Goroutinen mit frischem `ctx`, weil
dann der Host aus `Configure` einspringt. Transaktionen (`InTx`) gibt es nur innerhalb
einer Anfrage.

## Aggregate (Master-Detail)

Enthält das Metamodell eines Objects Relationen (`SectionDefinition.Relation`), registriert
`NewPlugin` automatisch zwei weitere Actions. Ein Modul kann sie auch selbst implementieren;
dann bleibt seine eigene Implementierung.

| Action | Payload | Antwort |
|---|---|---|
| `getAggregate` | `{"id": "…"}` | `{"record": {…}, "relations": {"<section>": [{…}], …}}` |
| `saveAggregate` | `{"id"?, "data"?, "relations": {"<section>": {"create": [{…}], "update": [{"id", "data"}], "expire": [{"id", "valid_to"}], "deactivate": ["id"]}}}` | wie `getAggregate`, Stand nach dem Speichern |

So verarbeitet `saveAggregate` die Änderungen:

```
InTx(env.DB)                                   ← eine Transaktion der Moduldatenbank
├── Master: create (ohne id) oder update (mit data)
└── je Relation, in der Reihenfolge der Abschnitte:
    ├── expire / deactivate → Ende nach Lifecycle (gehört das Unter-Object zum Master?)
    ├── update  → Fremdschlüssel bleibt auf den Master gesetzt
    └── create  → Fremdschlüssel = id des Masters (auch bei Neuanlage)
Fehler irgendwo → Rollback von allem; Erfolg → Commit, dann getAggregate
```

**Jeder Teilschritt ist ein normaler Aufruf über den Dispatcher** (`env.Services.Call`), zum
Beispiel `PartnerContact.create`. Daraus folgt dreierlei:

- Berechtigungen gelten je `Object.Action`. Wer `PartnerContact.create` nicht darf, kann es
  auch über das Aggregat nicht.
- Die Fachregeln der Actions greifen unverändert, etwa die E-Mail-Prüfung oder der
  Buchungskreis-Zwang.
- Die Transaktion geht automatisch auf die aufgerufenen Actions über (Tx-Weitergabe des Hosts).

**Kein Löschen:** `delete` im Aggregat wird abgelehnt. Unter-Objects enden nach ihrem
Lebenszyklus (`metamodel.Lifecycle`): Typ timeslice mit `expire` und Enddatum, Typ status mit
`deactivate`. Ein immutable Unter-Object endet gar nicht.

**Voraussetzungen:** Unter-Objects gehören zum selben Modul (`NewPlugin` prüft das). Sie
bieten Actions der Kinds `list` (Filter auf den Fremdschlüssel), `item`, `create`, `update`
sowie je nach Lifecycle `expire` oder `deactivate`. Der Master bietet `item`, `create` und `update`.

## Kapselung

| Grenze | Wodurch |
|---|---|
| **Code** | Fachcode liegt unter `cmd/plugins/<plugin>/internal/<modul>`. Kein anderes Go-Modul kann ihn importieren. |
| **Datenbank zwischen Plugins** | DBSchema: nur Tabellen mit `sdk.TablePrefix(<plugin>)`, keine Fremdschlüssel auf fremde Tabellen, kein `DROP`. |
| **Datenbank zwischen Modulen eines Plugins** | `NewPlugin`: Haben mehrere Module ein Schema, darf jedes nur Tabellen mit `module.TablePrefix(<plugin>, <modul>)` nutzen, z. B. `erp__sales_`. |
| **Zugriff auf fremde Daten** | nur `env.Services.Call(…)`, also über die Actions des anderen Moduls, mit Berechtigungsprüfung des Dispatchers |
| **Routen** | Ein Object gehört genau einem Modul. Der WebServer erreicht es nur in dessen Namensraum. |
| **Oberfläche** | Ein Object ohne Modul erscheint nicht im WebServer. |

Beispiel: `businesspartner` prüft Buchungskreise über
`env.Services.Call(ctx, "CompanyCode", "get", …)`, also über das Modul `admin` (Plugin `iam`),
und nicht über dessen Tabelle `iam__company_codes`.

## Ein neues Modul hinzufügen

**1. Paket anlegen**, zum Beispiel `cmd/plugins/sales/internal/order/module.go`:

```go
package order

type Module struct {
    db  module.DB
    log *slog.Logger
    svc module.Services
}

func New() *Module { return &Module{} }

func (m *Module) Descriptor() module.Descriptor {
    return module.Descriptor{Name: "sales", Title: "Verkauf", Icon: "icon-cart"}
}

func (m *Module) RegisterRoutes(r *module.Router) {
    r.Object("SalesOrder").Section("Belege").Describe(orderDef).
        Handle("list", m.list).
        Handle("get", m.get).
        Handle("create", m.create)
    r.Object("SalesOrderType").Section("Kataloge").Describe(typeDef).
        Handle("list", m.listTypes)
}

func (m *Module) Initialize(ctx context.Context, env module.Env) error {
    m.db, m.log, m.svc = env.DB, env.Log, env.Services
    return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }

// Optional: eigene Tabellen (nur table-Blöcke, Präfix sales__).
func (m *Module) Schema() module.Schema { return module.Schema{HCL: schemaHCL} }

func (m *Module) list(ctx context.Context, req sdk.Request) (sdk.Response, error) {
    res, err := m.db.Query(ctx, "SELECT id, customer_id, total FROM sales__orders")
    // Kunde prüfen? Nur über das Partner-Modul:
    // m.svc.Call(ctx, "BusinessPartner", "get", map[string]any{"id": id})
    …
}
```

**2. `main.go`** des Plugins verdrahtet nur:

```go
func main() {
    plugin.Main(module.NewPlugin(module.Info{Name: "sales", Version: "0.1.0"}, order.New()))
}
```

**3. Host-Config** `configs/07-sales.yaml` mit `version`, `databases` und optional
`settings.modules.sales`.

**4. Bauen** nach der Resolver-Konvention, zum Beispiel
`bin/plugins/sa/sales-0.1.0-windows-amd64.exe`, und den Host starten. Das Modul erscheint
sofort in der Navigation, unter `/m/sales` und unter `/api/v1/sales`. Am WebServer ändert
sich nichts.

**5. Berechtigungen** vergibt `iam` wie bisher pro `Object.Action`, zum Beispiel
`SalesOrder.*`. Ein Modul ist sichtbar, sobald der Benutzer mindestens ein Object darin
nutzen darf.

**Regeln für Namen:**

- Modulnamen bestehen aus Kleinbuchstaben, Ziffern und `-` (2–40 Zeichen) und sind
  systemweit eindeutig; der Catalog lehnt Doppelte ab.
- Object-Namen sind PascalCase und global eindeutig, da der Dispatcher nach Object routet.

## Ohne Modul-Framework

Ein Plugin kann `sdk.Plugin` weiterhin direkt implementieren. Dann meldet es seine
Module selbst in `Catalog.Describe` (`metamodel.DescribeResponse.Modules`), so wie
`examples/plugins/hello` (Modul `demo`) und das Core-Plugin `iam` (Modul `admin`).
