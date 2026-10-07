// Package dispatcher leitet Anfragen über den Routing-Schlüssel
// (object, action) an das zuständige Plugin weiter.
//
// Er unterscheidet zwei Einstiege:
//   - Handle: Anfragen von außen (vertrauenswürdig). Sie eröffnen eine
//     Wurzelanfrage; ihr CallContext gilt für die ganze Aufrufkette.
//   - HandleNested: Aufrufe aus Plugins (HostService.Dispatch). Sie müssen zu
//     einer laufenden Wurzelanfrage gehören; Mandant und Benutzer werden durch
//     die vertrauenswürdigen Werte der Wurzelanfrage ersetzt.
package dispatcher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/camel/coremesh/pkg/sdk"
)

// Route ist ein Eintrag der Routing-Tabelle.
type Route struct {
	Object, Action, Plugin string
}

type key struct{ object, action string }

type plugin struct {
	name     string
	manifest sdk.Manifest
	handler  sdk.Handler
	inflight atomic.Int64
}

type request struct {
	call   sdk.CallContext
	nested int
}

type Dispatcher struct {
	mu      sync.RWMutex
	routes  map[key]*plugin
	plugins map[string]*plugin

	reqMu    sync.Mutex
	requests map[string]*request

	maxDepth     int
	onRequestEnd func(requestID string)
	log          *slog.Logger
	authz        Authorizer // geschützt durch mu
}

// New erzeugt einen Dispatcher. maxDepth begrenzt gleichzeitig verschachtelte
// Plugin-Aufrufe pro Anfrage (Schutz vor Zyklen A -> B -> A ...);
// onRequestEnd wird am Ende jeder Wurzelanfrage aufgerufen (z. B. um offene
// Transaktionen zurückzurollen).
func New(maxDepth int, onRequestEnd func(requestID string), log *slog.Logger) *Dispatcher {
	return &Dispatcher{
		routes:       map[key]*plugin{},
		plugins:      map[string]*plugin{},
		requests:     map[string]*request{},
		maxDepth:     maxDepth,
		onRequestEnd: onRequestEnd,
		log:          log,
	}
}

var (
	objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	actionRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
)

// lifecycle sind reservierte Capabilities, die nur der Host direkt auf dem
// Plugin aufruft. Sie landen nicht in der Routing-Tabelle – mehrere Plugins
// dürfen sie melden, und von außen sind sie nicht erreichbar.
var lifecycle = map[key]bool{
	{sdk.ObjectDBSchema, sdk.ActionInit}:    true,
	{sdk.ObjectCatalog, sdk.ActionDescribe}: true,
}

// hostOnly sind Routen, die nur der Host selbst über Call erreicht – weder
// Plugins (HandleNested) noch Anfragen von außen (Handle). Sonst könnte z. B.
// jedes Plugin über DBSchema.Activate beliebiges DDL ausführen.
var hostOnly = map[key]bool{
	{sdk.ObjectDBSchema, "Activate"}:     true,
	{sdk.ObjectDBSchema, "CheckVersion"}: true,
	{sdk.ObjectCatalog, "Register"}:      true,
}

// rootOnly sind Routen, die nur als Wurzelanfrage (Ingress, Host) erreichbar
// sind – nicht verschachtelt aus einem Plugin. So kann z. B. kein Fachmodul
// Passwörter durchprobieren.
var rootOnly = map[key]bool{
	{"Account", "Authenticate"}: true,
}

// userBaseline darf jeder angemeldete Benutzer, unabhängig von seinen Rollen:
// Katalog lesen (Navigation) und das eigene Konto.
var userBaseline = map[key]bool{
	{sdk.ObjectCatalog, "ListObjects"}:   true,
	{sdk.ObjectCatalog, "GetDefinition"}: true,
	{sdk.ObjectCatalog, "ListActions"}:   true,
	{sdk.ObjectCatalog, "ListModules"}:   true,
	{sdk.ObjectCatalog, "GetModule"}:     true,
	{sdk.ObjectCatalog, "Translations"}:  true,
	{"Account", "UpdateProfile"}:         true,
	{"Account", "Me"}:                    true,
	{"Account", "ChangePassword"}:        true,
	{"Account", "Check"}:                 true,
	{"Account", "Granted"}:               true,
	{"Account", "Display"}:               true,
}

// Authorizer entscheidet, ob ein Benutzer (object, action) aufrufen darf.
// Das interne Plugin iam implementiert es.
type Authorizer interface {
	Allowed(ctx context.Context, userID, object, action string) (bool, error)
}

// SetAuthorizer aktiviert die Berechtigungsprüfung. Ohne Authorizer wird
// nicht geprüft.
func (d *Dispatcher) SetAuthorizer(a Authorizer) {
	d.mu.Lock()
	d.authz = a
	d.mu.Unlock()
}

// authorize prüft eine Wurzelanfrage mit Benutzer. Anfragen ohne UserID sind
// System-Anfragen (Host, Start, Ingress vor der Anmeldung) und werden nicht
// geprüft. Fehler des Authorizers führen zur Ablehnung (fail closed).
func (d *Dispatcher) authorize(ctx context.Context, req sdk.Request) error {
	d.mu.RLock()
	a := d.authz
	d.mu.RUnlock()
	call := sdk.CallFromContext(ctx)
	if a == nil || call.UserID == "" || userBaseline[key{req.Object, req.Action}] {
		return nil
	}
	ok, err := a.Allowed(ctx, call.UserID, req.Object, req.Action)
	if err != nil {
		return fmt.Errorf("%w: Berechtigungsprüfung fehlgeschlagen", sdk.ErrPermissionDenied)
	}
	if !ok {
		return fmt.Errorf("%w: keine Berechtigung für %s.%s", sdk.ErrPermissionDenied, req.Object, req.Action)
	}
	return nil
}

// HasLifecycle meldet, ob das Manifest die Lebenszyklus-Capability object.action meldet.
func HasLifecycle(m sdk.Manifest, object, action string) bool {
	if !lifecycle[key{object, action}] {
		return false
	}
	for _, c := range m.Capabilities {
		if c.Object == object && slices.Contains(c.Actions, action) {
			return true
		}
	}
	return false
}

// Register prüft das Manifest und trägt alle (object, action)-Paare des
// Plugins ein – vollständig oder gar nicht. Ein Paar, das bereits einem
// anderen Plugin gehört, führt zur Ablehnung.
func (d *Dispatcher) Register(name string, m sdk.Manifest, h sdk.Handler) error {
	if m.Name != name {
		return fmt.Errorf("%w: Manifest-Name %q passt nicht zur Konfiguration %q", sdk.ErrInvalidArgument, m.Name, name)
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("%w: Plugin %s meldet keine Capabilities", sdk.ErrInvalidArgument, name)
	}
	var keys []key
	seen := map[key]bool{}
	for _, c := range m.Capabilities {
		if !objectRe.MatchString(c.Object) {
			return fmt.Errorf("%w: Plugin %s: ungültiges Object %q (PascalCase erwartet)", sdk.ErrInvalidArgument, name, c.Object)
		}
		if len(c.Actions) == 0 {
			return fmt.Errorf("%w: Plugin %s: Object %s ohne Actions", sdk.ErrInvalidArgument, name, c.Object)
		}
		for _, a := range c.Actions {
			if !actionRe.MatchString(a) {
				return fmt.Errorf("%w: Plugin %s: ungültige Action %q", sdk.ErrInvalidArgument, name, a)
			}
			k := key{c.Object, a}
			if seen[k] {
				return fmt.Errorf("%w: Plugin %s: %s.%s doppelt", sdk.ErrInvalidArgument, name, c.Object, a)
			}
			seen[k] = true
			if !lifecycle[k] {
				keys = append(keys, k)
			}
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.plugins[name]; ok {
		return fmt.Errorf("%w: Plugin %s ist bereits registriert", sdk.ErrAlreadyExists, name)
	}
	// Ein Object gehört genau einem Plugin – auch wenn sich die Actions
	// unterscheiden (sonst mischten sich z. B. Account.Check von iam und
	// Account.create eines Fachplugins unter einem Object, samt Rechten und
	// Metamodell). Ausgenommen sind die Lebenszyklus-Capabilities.
	objects := map[string]bool{}
	for _, k := range keys {
		objects[k.object] = true
	}
	owners := map[string]string{}
	for k, p := range d.routes {
		if objects[k.object] {
			owners[k.object] = p.name
		}
	}
	var conflicts []string
	for _, o := range sortedKeys(owners) {
		conflicts = append(conflicts, fmt.Sprintf("Object %s gehört bereits Plugin %s", o, owners[o]))
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%w: Plugin %s: %s", sdk.ErrAlreadyExists, name, strings.Join(conflicts, ", "))
	}
	p := &plugin{name: name, manifest: m, handler: h}
	d.plugins[name] = p
	for _, k := range keys {
		d.routes[k] = p
	}
	return nil
}

// Unregister entfernt die Routen eines Plugins; neue Anfragen erreichen es
// danach nicht mehr. Die zurückgegebene Funktion wartet, bis laufende
// Aufrufe abgeschlossen sind (Drain) oder ctx endet.
func (d *Dispatcher) Unregister(name string) (drain func(ctx context.Context) error) {
	d.mu.Lock()
	p, ok := d.plugins[name]
	if ok {
		delete(d.plugins, name)
		for k, rp := range d.routes {
			if rp == p {
				delete(d.routes, k)
			}
		}
	}
	d.mu.Unlock()

	return func(ctx context.Context) error {
		if !ok {
			return nil
		}
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		for p.inflight.Load() > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Plugin %s: %d Aufrufe noch offen: %w", name, p.inflight.Load(), ctx.Err())
			case <-t.C:
			}
		}
		return nil
	}
}

// Routes liefert die Routing-Tabelle, sortiert.
func (d *Dispatcher) Routes() []Route {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Route, 0, len(d.routes))
	for k, p := range d.routes {
		out = append(out, Route{k.object, k.action, p.name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Object != out[j].Object {
			return out[i].Object < out[j].Object
		}
		return out[i].Action < out[j].Action
	})
	return out
}

// Entry ist ein öffentlich aufrufbares (object, action)-Paar mit Herkunft.
type Entry struct {
	Object      string
	Action      string
	Plugin      string
	Version     string
	Description string // Beschreibung der Capability aus dem Manifest
}

// Catalog liefert alle von außen aufrufbaren Routen, sortiert nach Object
// und Action. Lebenszyklus-Capabilities (nicht geroutet) und Host-Routen
// (nur über Call erreichbar) fehlen bewusst.
func (d *Dispatcher) Catalog() []Entry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []Entry
	for k, p := range d.routes {
		if hostOnly[k] {
			continue
		}
		e := Entry{Object: k.object, Action: k.action, Plugin: p.name, Version: p.manifest.Version}
		for _, c := range p.manifest.Capabilities {
			if c.Object == k.object && slices.Contains(c.Actions, k.action) {
				e.Description = c.Description
				break
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Object != out[j].Object {
			return out[i].Object < out[j].Object
		}
		return out[i].Action < out[j].Action
	})
	return out
}

// Begin eröffnet eine vertrauenswürdige Wurzelanfrage, ohne zu routen –
// z. B. für Configure-Aufrufe beim Start. Fehlt die request_id, wird eine
// erzeugt. end muss genau einmal aufgerufen werden.
func (d *Dispatcher) Begin(ctx context.Context) (_ context.Context, end func(), _ error) {
	call := sdk.CallFromContext(ctx)
	if call.RequestID == "" {
		call.RequestID = NewRequestID()
		ctx = sdk.WithCall(ctx, call)
	}
	id := call.RequestID

	d.reqMu.Lock()
	if _, ok := d.requests[id]; ok {
		d.reqMu.Unlock()
		return ctx, func() {}, fmt.Errorf("%w: request_id %s läuft bereits", sdk.ErrAlreadyExists, id)
	}
	d.requests[id] = &request{call: call}
	d.reqMu.Unlock()

	return ctx, func() {
		d.reqMu.Lock()
		delete(d.requests, id)
		d.reqMu.Unlock()
		if d.onRequestEnd != nil {
			d.onRequestEnd(id)
		}
	}, nil
}

// Verify prüft, dass ctx zu einer laufenden Wurzelanfrage gehört, und liefert
// deren vertrauenswürdigen CallContext (Metadaten aus ctx bleiben erhalten).
func (d *Dispatcher) Verify(ctx context.Context) (sdk.CallContext, error) {
	got := sdk.CallFromContext(ctx)
	d.reqMu.Lock()
	r, ok := d.requests[got.RequestID]
	d.reqMu.Unlock()
	if !ok {
		return sdk.CallContext{}, fmt.Errorf("%w: unbekannte request_id", sdk.ErrPermissionDenied)
	}
	trusted := r.call
	trusted.Metadata = got.Metadata
	return trusted, nil
}

// Handle ist der Einstiegspunkt für Anfragen von außen (implementiert sdk.Handler).
func (d *Dispatcher) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	ctx, end, err := d.Begin(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	defer end()
	if err := d.authorize(ctx, req); err != nil {
		return sdk.Response{}, err
	}
	return d.route(ctx, req, false)
}

// HandleNested leitet den Aufruf eines Plugins weiter (HostService.Dispatch).
func (d *Dispatcher) HandleNested(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	if rootOnly[key{req.Object, req.Action}] {
		return sdk.Response{}, fmt.Errorf("%w: %s.%s ist nur als Wurzelanfrage erreichbar", sdk.ErrPermissionDenied, req.Object, req.Action)
	}
	got := sdk.CallFromContext(ctx)

	d.reqMu.Lock()
	r, ok := d.requests[got.RequestID]
	if !ok {
		d.reqMu.Unlock()
		return sdk.Response{}, fmt.Errorf("%w: unbekannte request_id", sdk.ErrPermissionDenied)
	}
	if d.maxDepth > 0 && r.nested >= d.maxDepth {
		d.reqMu.Unlock()
		return sdk.Response{}, fmt.Errorf("%w: maximale Aufruftiefe %d erreicht (Zyklus?) bei %s.%s",
			sdk.ErrFailedPrecondition, d.maxDepth, req.Object, req.Action)
	}
	r.nested++
	trusted := r.call
	d.reqMu.Unlock()
	defer func() {
		d.reqMu.Lock()
		r.nested--
		d.reqMu.Unlock()
	}()

	trusted.Metadata = got.Metadata
	return d.route(sdk.WithCall(ctx, trusted), req, false)
}

// Call routet einen Aufruf des Hosts selbst innerhalb einer bereits mit Begin
// eröffneten Wurzelanfrage (z. B. DBSchema.CheckVersion beim Plugin-Start).
func (d *Dispatcher) Call(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	if _, err := d.Verify(ctx); err != nil {
		return sdk.Response{}, err
	}
	return d.route(ctx, req, true)
}

func (d *Dispatcher) route(ctx context.Context, req sdk.Request, fromHost bool) (sdk.Response, error) {
	if hostOnly[key{req.Object, req.Action}] && !fromHost {
		return sdk.Response{}, fmt.Errorf("%w: %s.%s ist dem Host vorbehalten", sdk.ErrPermissionDenied, req.Object, req.Action)
	}
	d.mu.RLock()
	p, ok := d.routes[key{req.Object, req.Action}]
	if ok {
		p.inflight.Add(1) // unter RLock, damit Drain keinen Aufruf übersieht
	}
	d.mu.RUnlock()
	if !ok {
		return sdk.Response{}, fmt.Errorf("%w: kein Plugin für %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
	}
	defer p.inflight.Add(-1)
	return p.handler.Handle(ctx, req)
}

// NewRequestID erzeugt eine zufällige Korrelations-ID.
func NewRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
