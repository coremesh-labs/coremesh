# CoreMesh WebServer

Das generische Web-Frontend von CoreMesh, implementiert als **externes Plugin**.

Es nimmt HTTP-Anfragen an und übersetzt sie in `(object, action)`-Aufrufe an die
Fachmodule. Dabei registriert es **nur Module**: fachliche Namensräume, die Objects bündeln
(`/m/businesspartner/…`, `/api/v1/businesspartner/…`). Tabellen, Formulare und Detailansichten entstehen allein aus dem
**Metamodell**, das die Module beim Catalog hinterlegen. Der WebServer enthält
keinen Code für einzelne Objects. Ein neues Fachmodul erscheint in der Oberfläche,
sobald es läuft und ein Metamodell liefert.

```
Browser ──HTTP──▶ WebServer (Plugin, Ingress) ──Host.Handle──▶ Dispatcher ──▶ Fachmodul
   ▲                 │ html/template + HTMX                    │
   └──── HTML ───────┘                         Catalog.ListModules / GetModule / GetDefinition
```

## Inhalt

1. [Grundprinzip: Wie aus einer URL ein Aufruf wird](#1-grundprinzip)
2. [URL-Schema](#2-url-schema)
3. [Object und Action bestimmen](#3-object-und-action-bestimmen)
4. [Parameter und Payloads](#4-parameter-und-payloads)
5. [Formulardaten → Payload](#5-formulardaten--payload)
6. [Aufrufkontext](#6-aufrufkontext)
7. [Rendering: HTMX-Fragment oder ganze Seite](#7-rendering-htmx-fragment-oder-ganze-seite)
8. [HTMX-Swaps im Detail](#8-htmx-swaps-im-detail)
8b. [Master-Detail und Lookups](#master-detail-und-lookups)
8c. [Lebenszyklus statt Löschen](#lebenszyklus-statt-löschen)
8d. [Mehrsprachigkeit](#mehrsprachigkeit-de-en-zh-cn) und [historische Einträge](#historische-einträge)
8e. [Tags (TagEditor) und Services](#tags-tageditor-und-services)
9. [Fehlerbehandlung](#9-fehlerbehandlung)
10. [Templates und Blöcke überschreiben](#10-templates-und-blöcke-überschreiben)
11. [Konfiguration](#11-konfiguration) und [Anmeldung und Sicherheit](#anmeldung-und-sicherheit)
12. [Bauen, testen, austauschen](#12-bauen-testen-austauschen)
13. [Grenzen und nächste Schritte](#13-grenzen-und-nächste-schritte)

---

## 1. Grundprinzip

### Module statt einzelner Objects

Der WebServer registriert **ausschließlich Module** (`Catalog.ListModules`). Ein Modul
ist ein fachlicher Namensraum, der zusammengehörige Business-Objects bündelt, zum
Beispiel `businesspartner` mit `BusinessPartner`, `PartnerRole`, … und den Katalogen.
Module beschreibt jedes Plugin selbst (`metamodel.ModuleDefinition`, am einfachsten
über `pkg/sdk/module`). Ein Object ohne Modul ist über den WebServer nicht erreichbar.

Jedes Modul bekommt drei **gekapselte Sub-Router** (`modules.go`):

| Präfix | Sub-Router | Inhalt |
|---|---|---|
| `/m/{module}` | `uiRoutes` | Oberfläche (HTMX) |
| `/action/{module}` | `actionRoutes` | custom-Actions |
| `/api/v1/{module}` | `apiRoutes` | JSON-API |

`mountModule` löst das Modul **einmal je Anfrage** auf (`Catalog.GetModule`). Danach
beschränkt es das Modul auf die Objects, die laufen und für die der Benutzer eine
Berechtigung hat, und reicht die Anfrage mit abgeschnittenem Präfix an den Sub-Router
weiter. Was das für die Kapselung bedeutet:

- `/m/businesspartner/User` → **404**: `User` gehört zum Modul `admin`.
- Ein unbekanntes Modul → **404**.
- Ein Modul ohne ein einziges berechtigtes Object → **403**, und es fehlt in der Navigation.

### Von der URL zum Aufruf

Jede Anfrage im Modul wird in vier Schritten verarbeitet:

1. **Object aus der URL:** `/m/businesspartner/BusinessPartner/…` → Object
   `BusinessPartner`. Es muss PascalCase haben **und zum Modul gehören**, sonst 404.
2. **Metamodell holen:** `Catalog.GetDefinition {"object": "BusinessPartner"}`
   liefert Felder, Titel und die angebotenen Actions mit ihrem **Kind**.
3. **Action über das Kind wählen:** Die HTTP-Methode und der Pfad bestimmen ein Kind
   (z. B. `GET /m/{module}/X` → `list`). Der WebServer nimmt die Action, die im Metamodell
   dieses Kind hat. Wie sie heißt (`List`, `list`, `search`), ist egal.
4. **Aufruf:** `Host.Handle(ctx, {Object, Action, Payload})`. Das Payload folgt den
   Konventionen aus [Abschnitt 4](#4-parameter-und-payloads).

Danach wird die Antwort gerendert: bei HTMX-Anfragen nur das Fragment, sonst die ganze Seite.

## 2. URL-Schema

| Methode & Pfad | Kind | Aufruf an das Modul | Antwort (HTMX) | Antwort (ohne JS) |
|---|---|---|---|---|
| `GET /login`, `POST /login`, `POST /logout` | – | – (siehe „Anmeldung und Sicherheit“) | | |
| `GET /` | – | `Catalog.ListModules` | Startseite: Modul-Kacheln | Seite |
| `GET /m/{module}` | `list` | wie unten, für das **erste** Object des Moduls | Tabelle | Seite |
| `GET /m/{module}/{object}` | `list` | `{query}` | `<section id="list">` mit Tabelle | Seite |
| `GET /m/{module}/{object}/new` | (`create`) | – (nur Metamodell) | Formular im Dialog `#modal` | Seite mit Formular |
| `POST /m/{module}/{object}` | `create` | `{data}` | neue `<tr>` + Dialog zu + Toast | `303` → Liste |
| `GET /m/{module}/{object}/{id}` | `item` | `{id}` | `<section id="detail">` | Seite |
| `GET /m/{module}/{object}/{id}/edit` | `item` + (`update`) | `{id}` | Formular mit Werten im Dialog | Seite mit Formular |
| `PUT /m/{module}/{object}/{id}` | `update` | `{id, data}` | Zeile oder Detail + Dialog zu + Toast | – |
| `DELETE /m/{module}/{object}/{id}` | – | – | immer **405**, siehe [Lebenszyklus](#lebenszyklus-statt-löschen) | – |
| `GET/POST /m/{module}/{object}/{id}/end` | `expire` / `deactivate` | `{id, valid_to}` / `{id}` | Dialog bzw. Zeile/Detail + Toast | Seite / `303` |
| `POST /m/{module}/{object}/{id}` | `update` | wie PUT, gesteuert über `_method` (DELETE → 405) | wie PUT | `303` → Detail |
| `GET /action/{module}/{object}/{name}` | `custom` | – (nur Metamodell) | Formular der Action im Dialog | Seite mit Formular |
| `POST /action/{module}/{object}/{name}` | `custom` | `{id?, data}` | Ergebnis im Dialog + Toast | Seite mit Ergebnis |
| `GET /api/v1/{module}` | – | `Catalog.ListActions` je Object | JSON, siehe [API](#json-api) | |
| `POST /api/v1/{module}/{object}/{action}` | beliebig | JSON-Body | JSON, siehe [API](#json-api) | |
| `GET /m/{module}/{object}/{id}/rel/{section}` | `list` des Unter-Objects | `{query: {<fk>: <id>}}` | eingebettete Tabelle | – |
| `GET /lookup?from=…&field=…` | `list` des Nachschlage-Objects | `{query: {q}}` | Auswahldialog in `#lookup` | – |
| `GET /api/v1/{module}/{object}` | – | Catalog | JSON: Metadaten (Lookups, Relationen, Actions) | |
| `GET /static/…` | – | – | CSS u. a. | |

**Hinweise:**

- `{id}` ist URL-kodiert. Die ID `p/1` steht in der URL als `p%2F1`. Die ID `new` ist nicht adressierbar.
- **Custom-Actions** liegen unter `/action/` statt unter `/m/{module}/{object}/…`. Sonst gäbe es
  Mehrdeutigkeiten mit `…/{id}/edit`, etwa bei einer Action namens `edit`.
- In Klammern gesetzte Kinds werden nur geprüft: Das Formular erscheint nur, wenn das
  Object die Action anbietet.
- Bietet ein Object ein Kind nicht an, antwortet der WebServer mit **404**. In der
  Oberfläche erscheinen nur die Buttons angebotener Actions.
- Die früheren Objekt-Routen `/ui/{object}` gibt es seit Version 0.4.0 nicht mehr.

### JSON-API

Die API nutzt dieselbe Session wie die Oberfläche (Cookie aus `POST /login`).

- `GET /api/v1/{module}` beschreibt das Modul: Objects mit Titel, Gruppe und den Actions,
  die der Benutzer aufrufen darf.
- `POST /api/v1/{module}/{object}/{action}` ruft eine beliebige Route eines Objects des
  Moduls auf. Der JSON-Body ist das Payload. Die Antwort lautet `{"payload": …, "metadata": …}`.
- Fehler kommen als `{"error": "…"}` mit demselben Status wie in der Oberfläche, zum
  Beispiel `404`, `403` oder `422`. Ohne Session antwortet die API mit `401`.
- **CSRF-Schutz:** Ein Body verlangt `Content-Type: application/json`, sonst antwortet die
  API mit `415`. Fremde Seiten können diesen Content-Type nicht ohne CORS-Freigabe senden;
  zusätzlich greift die Origin-Prüfung.

```bash
curl -b cookies.txt -H "Content-Type: application/json" \
     -d '{"query":{"q":"muster"}}' http://localhost:8080/api/v1/businesspartner/BusinessPartner/list
```

## 3. Object und Action bestimmen

Das Metamodell (`pkg/sdk/metamodel`) verbindet Oberfläche und Dispatcher:

```go
Actions: []metamodel.ActionConfig{
    {Name: "List",   Kind: metamodel.KindList,   Label: "Übersicht"},
    {Name: "Item",   Kind: metamodel.KindItem,   Label: "Anzeigen"},
    {Name: "Create", Kind: metamodel.KindCreate, Label: "Neu"},
    {Name: "Update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
    {Name: "Delete", Kind: metamodel.KindDelete, Label: "Löschen", Confirm: "Wirklich löschen?"},
    {Name: "Notify", Kind: metamodel.KindCustom, Label: "Benachrichtigen"},
}
```

| Feld | Bedeutung |
|---|---|
| `Name` | Action im Dispatcher. `Kind: list, Name: "List"` führt zum Aufruf `BusinessPartner.List`. |
| `Kind` | Rolle in der Oberfläche: legt Endpunkt und Payload-Form fest |
| `Label` | Text auf Buttons und in Toasts |
| `Confirm` | Sicherheitsabfrage vor dem Ausführen (`hx-confirm`) |

**Regeln:**

- Pro Kind gilt die **erste** Action. Mehrere `custom`-Actions sind erlaubt; sie werden über
  ihren `Name` angesprochen.
- Der Catalog hat beim Registrieren bereits geprüft, dass jede `ActionConfig.Name` eine echte
  Route des Moduls ist. Die Oberfläche bietet also nie eine Action an, die es nicht gibt.

## 4. Parameter und Payloads

Für jedes Kind gilt eine **feste Payload-Form**. Fachmodule, die diese Form
einhalten, funktionieren ohne Anpassung mit dem WebServer.

### Anfrage an das Modul

| Kind | Payload | Herkunft |
|---|---|---|
| `list` | `{"query": {"<param>": "<wert>", …}}` | alle URL-Parameter. Ein einzelner Wert wird String, mehrfache werden Liste. |
| `item` | `{"id": "<id>"}` | `{id}` aus dem Pfad |
| `create` | `{"data": {<feld>: <wert>, …}}` | Formular, siehe [Abschnitt 5](#5-formulardaten--payload) |
| `update` | `{"id": "<id>", "data": {…}}` | Pfad + Formular |
| `expire` | `{"id": "<id>", "valid_to": "JJJJ-MM-TT"}` | Pfad + Datum aus dem Ende-Dialog |
| `deactivate` | `{"id": "<id>"}` | Pfad |
| `custom` | `{"id": "<id>", "data": {…}}` | `id` optional (aus `?id=` bzw. `_id`), Formular |

**Beispiel:** `GET /m/businesspartner/BusinessPartner?q=acme&page=2&tag=a&tag=b` führt zu

```json
{ "object": "BusinessPartner", "action": "List",
  "payload": { "query": { "q": "acme", "page": "2", "tag": ["a", "b"] } } }
```

Die Bedeutung der Parameter (Suche, Seite, Sortierung) legt das Modul fest. Der
WebServer reicht sie nur durch.

### Antwort des Moduls

| Kind | Erwartete Antwort |
|---|---|
| `list` | `[record, …]` **oder** `{"items": [record, …]}` (dort ist Platz für z. B. `"total"`) |
| `item`, `create`, `update` | `record` |
| `expire`, `deactivate` | geänderter Datensatz (für Zeile bzw. Detail) |
| `custom` | beliebig. Ein String-Feld `"message"` wird als Text gezeigt, sonst das JSON. |

Ein **record** ist ein JSON-Objekt mit den Feldern aus dem Metamodell.

- **`"_id"`** identifiziert den Datensatz und erscheint in den URLs. Bei Zeitscheiben enthält
  er das Beginndatum, zum Beispiel `4711|2026-01-01`. Fehlt `"_id"`, gilt `"id"`.
- **`"id"`** ist der fachliche Schlüssel, auf den andere Datensätze verweisen. Eingebettete
  Abschnitte filtern ihre Unter-Objects danach (`{<fk>: <id>}`), denn ein Unter-Object verweist
  auf den Partner, nicht auf eine bestimmte Zeitscheibe.

Die Werte werden anhand von `FieldDefinition.Type` angezeigt: Bei `select` das Label der Option,
bei `boolean` „Ja“/„Nein“. Fehlt in der Antwort von `create` oder `update` die `id`,
ergänzt der WebServer sie aus dem Pfad.

## 5. Formulardaten → Payload

Das Formular entsteht aus `ObjectDefinition.Fields`:

| `Type` | Formular-Element | Wert im Payload |
|---|---|---|
| `text` | `<input type="text">` | String |
| `textarea` | `<textarea>` | String |
| `email` | `<input type="email">` | String (geprüft mit `net/mail`) |
| `number` | `<input type="number" step="any">` | Zahl (`float64`, `,` als Dezimaltrenner erlaubt) |
| `date` | `<input type="date">` | String `JJJJ-MM-TT` (geprüft) |
| `select` | `<select>` mit `Options` | String, muss eine der Options sein |
| `boolean` | `<input type="checkbox">` | `true`/`false` (ein fehlendes Feld zählt als `false`) |

**Regeln:**

- **Nur Felder mit `Editable: true`** gelangen in `data`. Alle anderen Formularwerte,
  auch eingeschleuste, ignoriert der WebServer. Das schützt vor Mass Assignment.
- **`Required: true`** setzt das HTML-Attribut `required` und wird zusätzlich serverseitig geprüft.
- **Leere optionale Felder** werden zu `null`.
- **Neu-Formular:** Nicht editierbare Felder fehlen. **Bearbeiten-Formular:** Sie werden
  schreibgeschützt angezeigt (`readonly` bzw. `disabled` bei `select`/Checkbox).
- **Fehler bei der Prüfung** (Pflichtfeld, Format, ungültige Option) führen nicht zum Aufruf
  des Moduls. Das Formular kommt mit Status **422** und den Eingaben zurück.
- Meldet das Modul `sdk.ErrInvalidArgument`, erscheint das Formular ebenfalls erneut,
  mit der Meldung des Moduls.
- **Steuerfelder** mit Unterstrich werden nicht ans Modul weitergegeben:

| Feld | Zweck |
|---|---|
| `_method` | `PUT`/`DELETE` bei Formularen ohne JavaScript |
| `_view` | `row`/`detail`: wohin die Antwort von `update`/`delete` gehört |
| `_id` | id für `custom`-Actions |

## 6. Aufrufkontext

Der WebServer ist **Ingress**: Er nimmt Anfragen von außen an. Er braucht deshalb in der
Host-Konfiguration `ingress: true`. Damit darf er für jede HTTP-Anfrage eine **eigene
Wurzelanfrage** starten. Plugins ohne diese Freigabe dürfen nur innerhalb laufender
Anfragen aufrufen.

| Feld (`sdk.CallContext`) | Wert |
|---|---|
| `RequestID` | zufällig, pro HTTP-Anfrage |
| `TenantID` | `tenant_id` des angemeldeten Benutzers, sonst `settings.tenant` |
| `UserID` | `id` des angemeldeten Benutzers |
| `Metadata["username"]` | Benutzername |
| `Metadata["ingress"]` | `"webserver"` |
| `Metadata["locale"]` | erste Sprache aus `Accept-Language`, z. B. `de-CH` |

Fachmodule lesen das mit `sdk.CallFromContext(ctx)`. Mandant und Benutzer stammen
damit immer aus der Anmeldung und nie aus dem Formular.

**Abbruch:** Bricht der Browser die Anfrage ab, endet über `r.Context()` die ganze
Aufrufkette im Host, einschließlich verschachtelter Plugin-Aufrufe.

**Was gesperrt bleibt:** Host-Routen wie `Catalog.Register` oder `DBSchema.Activate`
sind auch für den Ingress nicht erreichbar.

## 7. Rendering: HTMX-Fragment oder ganze Seite

Eine Entscheidung je Anfrage, über den Header `HX-Request`:

| Anfrage | Ausgabe |
|---|---|
| `HX-Request: true` (HTMX-Swap) | **nur das Fragment**, z. B. `<section id="list">`, `<form>`, `<tr>` |
| sonst (Seitenaufruf, F5, Lesezeichen) | **Layout** (Kopf, Sidebar, Skripte) **mit dem Fragment** in `#main-content` |
| `HX-History-Restore-Request: true` | ganze Seite (Zurück-Button, wenn der HTMX-Verlauf leer ist) |

**Umsetzung:** Alle Handler rufen `render(w, r, status, fragment, data, …)` auf. Für eine
ganze Seite wird das Fragment zuerst gerendert und dann als `.Content` in den Block
`layout` eingesetzt. Jede Antwort trägt `Vary: HX-Request`, damit Caches beide Varianten
auseinanderhalten.

**Sidebar:** Bei jeder ganzen Seite ruft der WebServer `Catalog.ListModules` auf. Er
zeigt alle Module mit mindestens einem sichtbaren Object. Nur das **aktive** Modul klappt
seine Objects auf, gruppiert nach `Section` (z. B. „Partnerdaten“, „Kataloge“):

```html
<a class="module active" href="/m/businesspartner">Geschäftspartner</a>
<div class="subnav">
  <span class="section">Partnerdaten</span>
  <a href="/m/businesspartner/BusinessPartner" hx-get="/m/businesspartner/BusinessPartner"
     hx-target="#main-content" hx-push-url="true">Geschäftspartner</a>
  …
</div>
```

Ein Modulwechsel lädt die ganze Seite, damit die Seitenleiste die Objects des neuen Moduls
zeigt. Innerhalb eines Moduls tauscht HTMX nur `#main-content`, und die URL landet im
Browserverlauf. Ohne JavaScript sind alle Einträge normale Links.

## 8. HTMX-Swaps im Detail

Pflicht-Container im Layout: `#main-content`, `#modal`, `#toast-container`.

| Aktion | Auslöser | Ziel / Swap | Antwort |
|---|---|---|---|
| Navigation | Sidebar, Zurück-Link | `#main-content` innerHTML, URL im Verlauf | Fragment `list`/`detail` |
| Neu / Bearbeiten / Custom öffnen | Button | `#modal` innerHTML | Fragment `form` im `<dialog>` |
| Anlegen | `hx-post` | `#rows` beforeend | `<tr>` + OOB `#modal` leeren + OOB Toast |
| Speichern (aus Tabelle) | `hx-put`, `_view=row` | `#row-<hex(id)>` outerHTML | `<tr>` + OOB + Toast |
| Speichern (aus Detail) | `hx-put`, `_view=detail` | `#detail` outerHTML | `<section id="detail">` + OOB + Toast |
| Löschen (Tabelle) | `hx-delete` | `closest tr` outerHTML | leer + OOB Toast |
| Löschen (Detail) | `hx-delete`, `_view=detail` | – | Header `HX-Location` → Übersicht |
| Custom ausführen | `hx-post` | `#modal` | Ergebnis + Toast, Header `HX-Trigger: coremesh-changed` |

Dazu zwei Mechanismen:

- **Toast:** Er kommt als Out-of-Band-Swap: `<div hx-swap-oob="beforeend:#toast-container">`.
- **Automatisches Nachladen:** Die Übersicht hört auf `coremesh-changed` und lädt sich
  danach neu (`hx-trigger="coremesh-changed from:body"`).

**Zeilen-IDs** sind `row-` plus die hexadezimal kodierte `id`. So bleiben sie gültige
CSS-Selektoren, auch bei IDs wie `p/1`.

**Keine Vererbung:** `htmx-config` setzt `disableInheritance: true`. Jedes Element nennt
`hx-target` und `hx-swap` selbst. In eigenen Templates also nicht darauf verlassen,
dass ein Elternelement sie vorgibt.

## Master-Detail und Lookups

Die Detailansicht folgt dem Stil von LeanIX: aufklappbare Abschnitte mit Feldern und
eingebetteten Tabellen zugeordneter Unter-Objects. Lookup-Felder wählen ihren Wert in einem
Dialog aus einer Stammdatentabelle. Alles kommt aus dem **Metamodell**. Der WebServer kennt
kein einziges Object.

### Metamodell

```go
metamodel.ObjectDefinition{
    Name: "BusinessPartner", TitleField: "name1",        // Überschrift „Geschäftspartner · Muster AG“
    Sections: []metamodel.SectionDefinition{
        {Key: "stammdaten", Title: "Stammdaten", Fields: []string{"type", "name1", "name2"}},
        {Key: "adressen", Title: "Adressen", Relation: &metamodel.Relation{
            Object: "PartnerAddress", ForeignKey: "bp_id",            // Unter-Object + Fremdschlüssel
            Columns: []string{"address_role_code", "address_id"}}},   // Spalten der eingebetteten Tabelle
        {Key: "bank", Title: "Bankverbindungen", Collapsed: true, Relation: …},
    },
}
metamodel.FieldDefinition{Key: "address_role_code", …,
    Lookup: &metamodel.Lookup{Object: "PartnerAddressRole", ValueField: "code", LabelFields: []string{"description"}}}
```

| Element | Wirkung |
|---|---|
| `Sections` | `<details>`-Abschnitte. Felder ohne Abschnitt stehen vorne in „Allgemein“. Ohne Sections bleibt die einfache Feldliste. |
| `Relation` | Eingebettete Tabelle des Unter-Objects, gefiltert über `{"query": {<ForeignKey>: <id>}}` |
| `Lookup` | Feld mit Schaltfläche „Auswählen …“, die den Auswahldialog öffnet |
| `"_labels"` im Datensatz | Lesbarer Text statt des Schlüssels, in Tabellen, Details und Formularen. Das Modul liefert ihn. |

Eine **n:m-Beziehung** ist eine Relation auf die Zwischentabelle, deren zweiter Schlüssel ein
Lookup ist: `BusinessPartner` → `PartnerAddress` (Rolle, Zeitscheibe) → `PartnerAddressData`
über `address_id`.

### Ablauf in der Oberfläche

| Aktion | Anfrage | Antwort |
|---|---|---|
| Abschnitt laden | `GET /m/{module}/{object}/{id}/rel/{section}` (`hx-trigger="load, coremesh-changed from:body"`) | Fragment `relation` |
| Hinzufügen | `GET …/{child}/new?{fk}={id}&_lock={fk}&_view=refresh` | Formular im Dialog; der Fremdschlüssel geht als verstecktes Feld fest mit |
| Bearbeiten | `GET …/{child}/{cid}/edit?view=refresh&_lock={fk}` | dito, mit Werten und Labels |
| Speichern | `POST`/`PUT` mit `_view=refresh` | leerer Swap in `#modal` (Dialog zu) + Toast + `HX-Trigger: coremesh-changed` → die Abschnitte laden neu |
| Löschen | `DELETE …/{child}/{cid}?_view=refresh` | Zeile entfernt + Toast |
| Verknüpften Datensatz bearbeiten (✎) | `GET …/{lookup-object}/{value}/edit?view=refresh` | z. B. die Adresse selbst ändern, direkt aus dem Abschnitt |

Der ✎-Link erscheint bei Lookups mit `ValueField: "id"` auf Objects desselben Moduls, also bei
Stammdaten wie Adressen. Kataloge werden über ihren Code referenziert und bekommen keinen Link.

**Jede Änderung ist eine eigene Action des Unter-Objects.** Fachregeln und Berechtigungen gelten
unverändert; es gibt keine Sonderwege für die Detailansicht.

### Lookup-Dialog

`GET /lookup?from=<Object>&field=<Feld>[&q=…][&rows=1]` wird in `#lookup` über dem Formular
geladen.

1. Das Nachschlage-Object kommt aus dem **Metamodell des Felds** (`from` + `field`), nicht aus
   der URL. Ein Aufrufer kann so kein beliebiges Object abfragen.
2. Gelesen wird über die `list`-Action des Ziels mit `{"query": {"q": …}}`. Der Dispatcher prüft
   die Berechtigung. Ignoriert das Ziel `q`, filtert der WebServer die Zeilen selbst.
3. Spalten: `Lookup.Columns`, sonst die listable Felder des Ziels. Höchstens 100 Zeilen.
4. Die Suche lädt nur die Zeilen neu (`rows=1`, Verzögerung 300 ms).
5. Ein Klick oder Enter auf eine Zeile ruft `coremeshPick` (Block `scripts`) auf. Die Funktion
   setzt den Schlüssel ins Feld und den Text (`LabelFields`) daneben und schließt den Dialog.

Lookups dürfen **Modulgrenzen überschreiten**, zum Beispiel die Buchungskreise aus `iam` in den
Partner-Buchungskreisdaten, aber nur lesend über die Actions des Ziels.

Ohne JavaScript bleibt das Lookup-Feld ein normales Textfeld für den Schlüssel.

### Aggregat-API (Fetch & Cascade Save)

Für Frontends, die ein zusammengesetztes Object als Ganzes laden und speichern, registriert
`pkg/sdk/module` je Object mit Relationen die Actions `getAggregate` und `saveAggregate`.
Die JSON-API erreicht sie wie jede andere Action:

```bash
curl -b jar -H "Content-Type: application/json" -d '{"id":"<bp>"}' \
     http://localhost:8080/api/v1/businesspartner/BusinessPartner/getAggregate
# → {"payload": {"record": {…}, "relations": {"adressen": […], "kommunikation": […], …}}}

curl -b jar -H "Content-Type: application/json" http://localhost:8080/api/v1/businesspartner/BusinessPartner/saveAggregate -d '{
  "id": "<bp>", "data": {"name2": "Zürich"},
  "relations": {
    "adressen":      {"create": [{"address_id": "<adr>", "address_role_code": "MAIN"}]},
    "kommunikation": {"update": [{"id": "<k>", "data": {"value": "info@muster.ch"}}], "expire": [{"id": "<k2>", "valid_to": "2026-12-31"}]}
  }}'
```

`saveAggregate` ist **atomar**: Scheitert ein Teil, wird alles zurückgerollt. Einzelheiten
stehen in [`pkg/sdk/module`](../../../pkg/sdk/module/README.md#aggregate-master-detail).

**Metadaten:** `GET /api/v1/{module}/{object}` liefert das Metamodell, die Lookups
(Fremdschlüssel), die Relationen, die erlaubten Actions und `aggregate: true|false`. Ein
eigenes Frontend erkennt daran, wo es einen Lookup-Dialog oder eine eingebettete Tabelle
braucht.

## Lebenszyklus statt Löschen

Physisch gelöscht wird im System nichts. Wie ein Datensatz endet, steht im Metamodell
(`ObjectDefinition.Lifecycle`):

| Typ | Erkennung | Oberfläche | Action | Backend |
|---|---|---|---|---|
| **A `timeslice`** | Zeitscheibe (`valid_from`/`valid_to`) | „Beenden …“ öffnet einen Dialog mit Datumswähler. Das Feld ist leer, Pflicht und frühestens „gültig ab“. | Kind `expire`, `{id, valid_to}` | setzt `valid_to` auf das gewählte Datum, nie automatisch auf heute; rückwirkend erlaubt |
| **B `status`** | Status-Flag (z. B. `is_active`) | „Inaktivieren“ öffnet eine Bestätigung. Bereits inaktive Datensätze zeigen „Inaktiv“. | Kind `deactivate`, `{id}` | setzt das Flag auf `false` |
| **C `immutable`** | weder noch | kein Button | – | `DELETE`, `/end` und `…/delete` per API antworten mit **405** |

```
GET  /m/{module}/{object}/{id}/end?view=row|detail|refresh   Dialog je Typ
POST /m/{module}/{object}/{id}/end                           valid_to (Typ A) bzw. Bestätigung (Typ B)
DELETE /m/{module}/{object}/{id}                             immer 405, mit Hinweis auf den richtigen Weg
```

Der Button ist eine Komponente (Block `end-button`, `lifecycle.html`) für Tabellenzeilen,
Detailansicht und eingebettete Abschnitte. Die Antwort ersetzt die Zeile, die Detailansicht
oder lädt die Abschnitte neu (`refresh`).

Fehler des Moduls, zum Beispiel ein Enddatum vor dem Beginn, erscheinen im Dialog (422).

Die Metadaten unter `GET /api/v1/{module}/{object}` enthalten
`"lifecycle": {"type", "end_action", "valid_from", "valid_to", "status_field"}`.

## Mehrsprachigkeit (de, en, zh-CN)

Unterstützt werden **Deutsch (`de`)**, **Englisch (`en`)** und **Chinesisch, Festland (`zh-CN`)**,
in vereinfachter Schrift. Die Tags `zh`, `zh-Hans` und `zh-SG` werden auf `zh-CN` abgebildet;
traditionelles Chinesisch (`zh-TW`, `zh-HK`) wird nicht unterstützt.

### Zwei Übersetzungsebenen

| Ebene | Wo | Schlüssel | Beispiele |
|---|---|---|---|
| **Framework** | WebServer: `i18n/de.json`, `en.json`, `zh-CN.json` | `core.…` | Speichern, Abbrechen, Ende-Dialog, Lookup, Validierung, Meldungen, Anmeldung, Standardtexte der Action-Kinds (`core.action.create` …) |
| **Fachtexte** | jedes Modul (z. B. `internal/businesspartner/i18n/*.json`) | `<modul>.…` | Titel, Feld-Labels, Auswahlwerte, Abschnitte, Navigation |

Fachtexte stehen im Metamodell als Übersetzungsschlüssel (`title_key`, `label_key`,
`confirm_key`, `section_key` …). Das Modul-SDK setzt sie nach einer Konvention, zum Beispiel
`businesspartner.BusinessPartner.fields.name1`; siehe `metamodel.WithKeys`. Das Modul liefert
seine Texte mit `Catalog.Describe`. Der Catalog prüft den Namensraum: Ein Modul darf nur eigene
Schlüssel setzen. Er liefert die Texte über `Catalog.Translations` zusammengeführt aus.

Der WebServer übersetzt jedes Metamodell vor dem Rendern (`localizeDef`). Fehlt ein Schlüssel,
gilt Deutsch, sonst der Originaltext. Actions ohne eigene Übersetzung erhalten den Standardtext
ihres Kinds. Templates übersetzen mit `{{t "core.app.save"}}`; dafür hat jede Sprache ihre
eigene Kopie der Templates.

### Sprachaushandlung (Middleware)

1. **Profil** des Benutzers: `iam`-Spalte `locale`, gesetzt über `PATCH /api/v1/user/profile`.
2. **Sprachwähler** der Oberfläche (Benutzermenü, Anmeldeseite): `POST /locale` setzt das
   Cookie `coremesh_lang`. Bei angemeldeten Benutzern wird die Wahl auch im Profil gespeichert.
   „Automatisch“ löscht beides.
3. **`Accept-Language`** des Browsers, mit q-Werten.
4. **Standard** aus `settings.default_locale`, sonst `de`.

Die Antwort trägt `Content-Language` und `<html lang="…">`.

### Endpunkte

| Endpunkt | Inhalt |
|---|---|
| `GET /api/v1/i18n/{lang}` | `{"locale", "locales", "translations": {…}}`: Core und alle Module, flach, mit Rückfall auf de |
| `GET /api/v1/user/profile` | eigenes Profil (`Account.Me`, inkl. `locale`) |
| `PATCH /api/v1/user/profile` | `{"locale": "en" \| "zh-CN" \| "de" \| ""}` (leer = automatisch) |
| `POST /locale` | Sprachwähler (Formular `locale`, `next`) |

Die Modulnamen `i18n` und `user` sind reserviert.

**Was nicht übersetzt wird:**
- **Datenwerte**, etwa Katalogbeschreibungen wie „Debitor“: Das sind Stammdaten. Mehrsprachige
  Stammdaten bräuchten Texttabellen je Sprache wie bei SAP.
- **Fehlermeldungen der Module**, etwa „Enddatum liegt vor dem Beginn“: Sie bleiben deutsch.
  Für eine Übersetzung bräuchten sie Fehlercodes.

## Historische Einträge

Listen und eingebettete Abschnitte zeigen standardmäßig nur **heute gültige bzw. aktive**
Datensätze. Bei Objects mit Zeitscheibe oder Status-Flag gibt es den Schalter
`[ ] Inaktive / historische Einträge anzeigen` (Block `history-toggle`). Er lädt die Liste
bzw. den Abschnitt mit `?includeHistory=true` neu. Der Parameter geht als
`{"query": {"includeHistory": "true"}}` an die `list`-Action; die API reicht ihn ebenso durch.

## Tags (TagEditor) und Services

**Services** sind Objects eines Moduls ohne Metamodell, zum Beispiel `Tags` im Modul
`tagmanagement`. Sie stehen in `ModuleDefinition.services`, sind nur über die JSON-API
`/api/v1/<modul>/<Service>/<action>` erreichbar und erscheinen nicht in der Navigation.
`GET /api/v1/<modul>` listet sie mit `"section": "service"` und den erlaubten Actions.

**TagEditor.** Ein Abschnitt mit `Tags: true` im Metamodell (`SectionDefinition`) bettet den
generischen Tag-Editor ein. Er wird lazy über den fachlichen Schlüssel `id` des Datensatzes
geladen, nicht über `_id`:

| Route | Zweck |
|---|---|
| `GET /tags/{entity}/{id}?effectiveDate=…&companyCode=…` | Editor als Fragment (`Tags.get`, Buchungskreise aus `CompanyCode.list`) |
| `POST /tags/{entity}/{id}/preview` | Regeln neu auswerten, ohne zu speichern (`Tags.validate`) |
| `POST /tags/{entity}/{id}` | speichern ab „Gültig ab“ (`Tags.set`) |

Templates: `templates/tags.html` (Blöcke `tag-editor`, `tag-field`), überschreibbar wie alle
Blöcke. Ablauf, Regeln und Buchungskreis-Logik beschreibt
[`cmd/plugins/tag/README.md`](../tag/README.md#ui-tageditor-entitytype-entityid).

## 9. Fehlerbehandlung

| Fehler des Moduls (`errors.Is`) | HTTP-Status |
|---|---|
| `sdk.ErrNotFound`, `sdk.ErrUnimplemented` | 404 |
| `sdk.ErrInvalidArgument` | 422 (bei Formularen: Formular erneut anzeigen) |
| `sdk.ErrPermissionDenied` | 403 |
| `sdk.ErrAlreadyExists`, `sdk.ErrFailedPrecondition` | 409 |
| `sdk.ErrUnavailable` (z. B. Modul läuft nicht) | 503 |
| Abbruch durch den Client | 499 |
| alles andere | 500. Die Meldung geht nur ins Host-Log, der Browser sieht „Interner Fehler“. |

**Wie der Fehler erscheint:**

- **Bei HTMX** als roter Toast: `HX-Retarget: #toast-container`, `HX-Reswap: beforeend`.
  Damit HTMX auch 4xx/5xx-Antworten tauscht, setzt das Layout
  `responseHandling` in `htmx-config`.
- **Ohne HTMX** als Fehlerseite im Layout.

## 10. Templates und Blöcke überschreiben

Die Standard-Templates sind eingebettet (`templates/*.html`, `go:embed`). Jedes Stück
ist ein benannter Block (`{{block}}` bzw. `{{define}}`). Mit `settings.templates_dir`
werden zusätzlich alle `*.html` aus diesem Verzeichnis **nach** den eingebetteten
geparst. Jeder dort neu definierte Block **ersetzt** den Standard-Block, alle anderen
bleiben.

| Datei | Blöcke | Daten |
|---|---|---|
| `layout.html` | `layout`, `head`, `brand`, `sidebar`, `nav-module`, `nav-item`, `content`, `footer`, `scripts` (Sprachwähler: `locale-switch` in `auth.html`) | `pageData` (`nav-module`: `navModule`, `nav-item`: `navItem`) |
| `list.html` | `list`, `list-toolbar`, `table`, `row`, `row-actions` | `view` |
| `form.html` | `form`, `form-buttons`, `field`, `lookup-field` | `view` bzw. `fieldCtx` |
| `detail.html` | `detail`, `detail-toolbar`, `detail-section`, `section-fields` | `view` |
| `relation.html` | `relation`, `relation-toolbar`, `relation-row` | `relationView` bzw. `relRow` |
| `lifecycle.html` | `end-button`, `end`, `end-timeslice`, `end-status`, `history-toggle` | `endBtn` bzw. `view` |
| `lookup.html` | `lookup`, `lookup-rows` | `lookupView` |
| `fragments.html` | `created`, `updated`, `modal-close`, `toast`, `result`, `home`, `error` | je Block |

**Beispiel** `web/templates/branding.html`:

```html
{{define "brand"}}<a class="brand" href="/"><img src="/static/logo.svg" alt=""> Meine Firma</a>{{end}}

{{define "head"}}
<link rel="stylesheet" href="/static/app.css">
<link rel="stylesheet" href="/static/firma.css">
<script src="/static/htmx.min.js"></script>  {{/* htmx lokal statt CDN */}}
{{end}}
```

Zusammen mit `static_dir: ./web/static` werden `logo.svg`, `firma.css` und
`htmx.min.js` von dort ausgeliefert. Was dort fehlt, kommt weiterhin aus den
eingebetteten Dateien.

**Zugriff in eigenen Blöcken:**

| Ausdruck | Ergebnis |
|---|---|
| `.Def` | `metamodel.ObjectDefinition` |
| `.Object` | Name des Objects |
| `.Module` | Namensraum des Moduls |
| `.URL` / `.ActionURL` | `/m/{module}/{object}` bzw. `/action/{module}/{object}` – Links immer hierüber bauen |
| `.Has.list` … `.Has.delete` | `*ActionConfig` oder `nil` |
| `.Custom` | `[]ActionConfig` |
| `.Rows` | Liste der Datensätze |
| `.Record` | aktueller Datensatz |
| `.ID` | id des aktuellen Datensatzes |
| `.Row .` | Sicht auf eine Tabellenzeile |

**Template-Funktionen:**

| Funktion | Zweck |
|---|---|
| `listable .Def` | Felder mit `Listable: true` |
| `value .Record field` | Wert zur Anzeige |
| `pathEscape` | Wert für URL-Pfade kodieren |
| `domID` | ID für DOM-Element-IDs |
| `json` | Wert als JSON |
| `t "core.…" args…` | Framework-Text in der Sprache der Anfrage |
| `locale`, `locales` | aktuelle bzw. alle Sprachen |
| `raw .Record "key"` | Rohwert eines Felds (z. B. Schlüssel für Links) |
| `sectionCtx`, `relRow` | Daten für die Blöcke `detail-section` und `relation-row` |

## 11. Konfiguration

`configs/04-webserver.yaml`:

```yaml
plugins:
  webserver:
    version: 0.4.0
    ingress: true                       # Pflicht: startet eigene Wurzelanfragen
    databases:
      main: { access: write }           # Pflicht: Session-Tabelle
    settings:
      listen: 0.0.0.0:8080              # alle Schnittstellen; Standard 127.0.0.1:8080
      title: CoreMesh
      tenant: demo                      # Mandant für Benutzer ohne eigenen Mandanten
      session_ttl: 12h                  # Gültigkeit einer Anmeldung
      tls_cert: ./certs/server.crt      # optional; beide gesetzt = HTTPS
      tls_key: ./certs/server.key
      cookie_secure: auto               # auto (bei TLS) | true (hinter TLS-Proxy) | false
      database: main                    # Datenbank der Session-Tabelle
      templates_dir: ./web/templates    # optional
      static_dir: ./web/static          # optional
```

- **Port belegt:** Der Start des Plugins scheitert mit einer klaren Meldung.
- **Netz ohne TLS:** Lauscht der Server nicht nur auf Loopback und hat kein TLS, warnt das Host-Log.
- **Windows-Firewall:** Beim ersten Start mit `0.0.0.0` fragt Windows unter Umständen nach
  einer Freigabe für `webserver-…exe`.

## Anmeldung und Sicherheit

Alle Seiten außer `/login` und `/static/…` verlangen eine Anmeldung.

| Methode & Pfad | Zweck |
|---|---|
| `GET /login`, `POST /login` | Anmeldeseite / Anmeldung (`username`, `password`, `next`) |
| `POST /logout` | Abmelden; die Session wird serverseitig gelöscht |
| `GET`/`POST /account/password` | eigenes Passwort ändern (beendet alle anderen Sessions) |

**Benutzer, Rollen, Passwörter** verwaltet das Core-Plugin **iam** (siehe
`internal/coreplugins/iam/README.md`). Der WebServer nutzt es über den Host:

| Schritt | Aufruf |
|---|---|
| Anmeldung prüfen | `Account.Authenticate {username, password}` (nur Ingress) |
| Profil des angemeldeten Benutzers pro Anfrage | `Account.Me` → Name, Mandant, Rollen, Berechtigungen |
| Passwort ändern | `Account.ChangePassword {current, new}` |

Der WebServer selbst hält nur die Sessions (über `DBSchema.Init`, siehe `schema.go`):

| Tabelle | Spalten |
|---|---|
| `webserver__sessions` | `id` (= SHA-256 des Cookie-Tokens), `user_id`, `created_at`, `expires_at` |

`webserver__users` aus Version 0.2.0 bleibt im Schema, wird aber nicht mehr genutzt.
DBSchema lehnt `DROP TABLE` ab. Der erste Benutzer entsteht jetzt in `iam`.

**Berechtigungen in der Oberfläche:** Die Navigation zeigt nur Module und Objects, für die der
Benutzer mindestens eine Berechtigung hat. Tabellen, Detailansichten und Dialoge zeigen
nur Buttons für erlaubte Actions. Ruft jemand eine verbotene Action direkt auf,
antwortet der WebServer mit **403** („keine Berechtigung …“). Verbindlich prüft unabhängig
davon der Dispatcher.

**Deaktivierte oder gelöschte Benutzer** verlieren ihre Session beim nächsten Aufruf.

**Ohne Anmeldung:**
- Seitenaufruf: `303` → `/login?next=<Pfad>`. Nach der Anmeldung geht es dorthin zurück,
  aber nur für lokale Pfade, damit kein Open Redirect möglich ist.
- HTMX-Anfrage: `401` mit `HX-Redirect: /login`.

**Schutzmaßnahmen:**

| Maßnahme | Umsetzung |
|---|---|
| Passwörter | bcrypt (Kosten 12) in iam, mindestens 10 Zeichen |
| Session-Cookie | zufälliges 256-Bit-Token, `HttpOnly`, `SameSite=Lax`, `Secure` bei TLS; in der Datenbank nur der Hash |
| Ablauf | `session_ttl`, danach neue Anmeldung; abgelaufene Sessions werden stündlich gelöscht |
| Brute Force | nach 5 Fehlversuchen je Benutzer und IP Sperre: 1, 2, 4, 8, max. 15 Minuten |
| Benutzer-Enumeration | gleiche Meldung und Laufzeit für „unbekannt“, „falsches Passwort“ und „deaktiviert“ (in iam) |
| CSRF | verändernde Anfragen nur von der eigenen Seite (`Sec-Fetch-Site`/`Origin`), zusammen mit `SameSite=Lax` und GET ohne Seiteneffekte |
| Header | `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, HSTS bei TLS |

Der WebServer greift über den Host auf seine Tabellen zu (`sdk.Host.Query`/`Exec`).
Als Ingress-Plugin läuft jeder dieser Aufrufe als eigene kurze Wurzelanfrage.
Transaktionen sind außerhalb einer Host-Anfrage deshalb nicht möglich, für die
Anmeldung aber auch nicht nötig.

## 12. Bauen, testen, austauschen

Der WebServer ist ein **eigenes Go-Modul** (`go.mod` in diesem Verzeichnis). Er
importiert nur `pkg/sdk`, `pkg/sdk/metamodel` und `pkg/sdk/plugin`. Den Zugriff
auf `internal/` verhindert der Go-Compiler. Abhängigkeiten wie SQLite, Atlas oder pgx
gelangen nicht in diese Binary.

```bash
cd cmd/plugins/webserver
go test ./...
go build -o ../../../bin/plugins/we/webserver-0.4.0-windows-amd64.exe .
```

Der Dateiname folgt der Konvention des Resolvers (`<name>-<version>-<os>-<arch>`, Unterordner `we/`).

- **Austauschen:** Jedes Plugin, das dieselben Payload-Konventionen nutzt, kann den
  WebServer ersetzen, etwa eine JSON-REST-API oder ein anderes Frontend.
- **Auslagern:** Für ein eigenes Repository genügt es, in `go.mod` die `replace`-Zeile durch
  eine Version von `github.com/camel/coremesh` zu ersetzen.
- **Debuggen:** Mit `-debug` starten und den Host mit `COREMESH_REATTACH_PLUGINS` anhängen,
  siehe `pkg/sdk/plugin`.

## 13. Grenzen und nächste Schritte

- **SSO:** OIDC oder SAML fehlen noch. Benutzer und Rollen verwaltet `iam` über die Objects
  `User` und `Role`.
- **Login-Sperre im Speicher:** Sie gilt pro WebServer-Prozess. Bei mehreren Instanzen
  hinter einem Load Balancer zählt jede für sich.
- **IP hinter einem Proxy:** Die Sperre nutzt `RemoteAddr`. Hinter einem Reverse Proxy
  wäre das immer die IP des Proxys, eine vertrauenswürdige `X-Forwarded-For`-Auswertung fehlt.
- **Keine Content-Security-Policy:** htmx vom CDN und Inline-Handler (`onclick`)
  bräuchten `unsafe-inline`. Mit lokal ausgeliefertem htmx und ohne Inline-Skripte wäre eine
  strikte CSP möglich.
- **htmx kommt per CDN** (unpkg, Version 2.0.4). Für den Betrieb ohne Internet: Block
  `head` überschreiben und die Datei über `static_dir` lokal ausliefern.
- **Kein Reaktivieren:** Inaktivierte Geschäftspartner lassen sich in der Oberfläche nicht wieder aktivieren (das Flag ist schreibgeschützt). Benutzer in `iam` schon (Feld „Aktiv“ im Formular).
- **Blättern und Sortieren** werden nur als URL-Parameter durchgereicht. Bedienelemente dafür
  folgen, sobald sich eine Konvention etwa für `total` etabliert hat.
