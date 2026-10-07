// Package hook ist das Core-Plugin "hook": der Hook-Dispatcher – synchrone
// Erweiterungspunkte zwischen Modulen (Schnittstelle: pkg/sdk/hook).
//
// Object Hook:
//   - Define {name, description, phases, data, on_failure, owner} – ein Modul
//     meldet einen Hook an (bei jedem Start).
//   - Subscribe {hook, phase, callback, priority, description} – ein Modul
//     abonniert einen Hook in einer Phase; aufgerufen wird <callback>.onHook.
//     Eine Sperre aus der Administration bleibt beim erneuten Anmelden erhalten.
//   - Call {hook, action, data} – der Besitzer ruft die Abonnenten der Phase
//     auf: synchron, im Kontext der auslösenden Anfrage, nach Priorität. modify
//     reicht ReturnData an den nächsten Abonnenten weiter; Meldungen (E/W/S)
//     werden gesammelt; in commit (nach dem Speichern) werden Fehler zu
//     Warnungen.
//   - list, get – Übersicht der Hooks (Administration → Erweiterungen).
//
// Object HookSubscription: list, get, lock, unlock (Abos einsehen und sperren).
//
// Hooks und Abos stehen in der Datenbank (hook__*), damit Sperren Neustarts
// überdauern und die Übersicht auch gestoppte Plugins zeigt.
package hook

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/internal/dispatcher"
	"github.com/camel/coremesh/pkg/sdk"
	hookapi "github.com/camel/coremesh/pkg/sdk/hook"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

const (
	Name    = "hook"
	Version = "0.1.0"
)

// Source liefert die aktuell aufrufbaren Routen (dispatcher.Catalog).
type Source func() []dispatcher.Entry

type settings struct {
	Database   string `json:"database"`
	TimeoutSec int    `json:"timeout_sec"` // Zeitlimit je Abonnent (Standard 10)
}

type Plugin struct {
	db       *database.Manager
	source   Source
	host     sdk.Host
	settings settings
}

var (
	_ sdk.Plugin = (*Plugin)(nil)

	hookNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+$`)
	objectRe   = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
)

func New(db *database.Manager, source Source) *Plugin { return &Plugin{db: db, source: source} }

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name: Name, Version: Version, Description: "Hook-Dispatcher: synchrone Erweiterungspunkte zwischen Modulen",
		Capabilities: []sdk.Capability{
			{Object: hookapi.Object, Actions: []string{hookapi.ActionDefine, hookapi.ActionSubscribe, hookapi.ActionCall, "list", "get"},
				Description: "Hooks definieren, abonnieren und aufrufen"},
			{Object: "HookSubscription", Actions: []string{"list", "get", "lock", "unlock"}, Description: "Abos einsehen und sperren"},
			{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
			{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}},
		},
	}, nil
}

func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	s := settings{Database: "main"}
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	if s.TimeoutSec <= 0 {
		s.TimeoutSec = 10
	}
	if _, err := p.db.DB(s.Database); err != nil {
		return err
	}
	p.settings, p.host = s, cfg.Host
	return nil
}

func (p *Plugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch req.Object + "." + req.Action {
	case sdk.ObjectDBSchema + "." + sdk.ActionInit:
		var in sdk.SchemaInitRequest
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: sdk.SchemaInitResponse{Module: in.Module, Schema: schemaHCL}}, nil
	case sdk.ObjectCatalog + "." + sdk.ActionDescribe:
		return sdk.Response{Payload: metamodel.DescribeResponse{
			Objects:      []metamodel.ObjectDefinition{metamodel.WithKeys("hooks", hookDef), metamodel.WithKeys("hooks", subscriptionDef)},
			Modules:      []metamodel.ModuleDefinition{metamodel.ModuleKeys(hooksModule)},
			Translations: translations,
		}}, nil
	case hookapi.Object + "." + hookapi.ActionDefine:
		return p.define(ctx, req.Payload)
	case hookapi.Object + "." + hookapi.ActionSubscribe:
		return p.subscribe(ctx, req.Payload)
	case hookapi.Object + "." + hookapi.ActionCall:
		return p.call(ctx, req.Payload)
	case "Hook.list":
		return p.hookList(ctx)
	case "Hook.get":
		return p.hookGet(ctx, req.Payload)
	case "HookSubscription.list":
		return p.subList(ctx, req.Payload)
	case "HookSubscription.get":
		return p.subGet(ctx, req.Payload)
	case "HookSubscription.lock":
		return p.setLocked(ctx, req.Payload, true)
	case "HookSubscription.unlock":
		return p.setLocked(ctx, req.Payload, false)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

// --- Define / Subscribe --------------------------------------------------------------

func validPhases(ps []string) ([]string, error) {
	var out []string
	for _, ph := range hookapi.AllPhases { // feste Reihenfolge
		if slices.Contains(ps, ph) {
			out = append(out, ph)
		}
	}
	for _, ph := range ps {
		if !slices.Contains(hookapi.AllPhases, ph) {
			return nil, fmt.Errorf("Phase %q – erlaubt: %s", ph, strings.Join(hookapi.AllPhases, ", "))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("mindestens eine Phase")
	}
	return out, nil
}

func (p *Plugin) define(ctx context.Context, payload any) (sdk.Response, error) {
	var d hookapi.Definition
	if err := sdk.Decode(payload, &d); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	d.Name, d.Owner = strings.TrimSpace(d.Name), strings.TrimSpace(d.Owner)
	if !hookNameRe.MatchString(d.Name) {
		return sdk.Response{}, fmt.Errorf("%w: Hook %q: <modul>.<punkt> in Kleinbuchstaben, z. B. ledger.posting", sdk.ErrInvalidArgument, d.Name)
	}
	phases, err := validPhases(d.Phases)
	if err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Hook %s: %v", sdk.ErrInvalidArgument, d.Name, err)
	}
	if d.OnFailure == "" {
		d.OnFailure = hookapi.FailBlock
	}
	if d.OnFailure != hookapi.FailBlock && d.OnFailure != hookapi.FailSkip {
		return sdk.Response{}, fmt.Errorf("%w: on_failure: block oder skip", sdk.ErrInvalidArgument)
	}
	existing, err := p.loadHook(ctx, d.Name)
	if err != nil {
		return sdk.Response{}, err
	}
	now := ts()
	if existing != nil && existing.Owner != "" && d.Owner != "" && existing.Owner != d.Owner {
		return sdk.Response{}, fmt.Errorf("%w: Hook %s gehört bereits Modul %s", sdk.ErrAlreadyExists, d.Name, existing.Owner)
	}
	if existing == nil {
		_, err = p.pool().ExecContext(ctx, p.q(`INSERT INTO hook__hooks (name, owner, description, phases, data_doc, on_failure, defined_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), d.Name, d.Owner, d.Description, strings.Join(phases, ","), d.Data, d.OnFailure, now, now)
	} else {
		_, err = p.pool().ExecContext(ctx, p.q(`UPDATE hook__hooks SET owner = ?, description = ?, phases = ?, data_doc = ?, on_failure = ?, updated_at = ? WHERE name = ?`),
			d.Owner, d.Description, strings.Join(phases, ","), d.Data, d.OnFailure, now, d.Name)
	}
	if err != nil {
		return sdk.Response{}, err
	}
	p.log(ctx, sdk.LogInfo, "Hook angemeldet", map[string]string{"hook": d.Name, "owner": d.Owner, "phases": strings.Join(phases, ",")})
	return sdk.Response{Payload: map[string]any{"hook": d.Name}}, nil
}

func (p *Plugin) subscribe(ctx context.Context, payload any) (sdk.Response, error) {
	var s hookapi.Subscription
	if err := sdk.Decode(payload, &s); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	s.Hook, s.Phase, s.Callback = strings.TrimSpace(s.Hook), strings.TrimSpace(s.Phase), strings.TrimSpace(s.Callback)
	switch {
	case !hookNameRe.MatchString(s.Hook):
		return sdk.Response{}, fmt.Errorf("%w: Hook %q", sdk.ErrInvalidArgument, s.Hook)
	case !slices.Contains(hookapi.AllPhases, s.Phase):
		return sdk.Response{}, fmt.Errorf("%w: Phase %q – erlaubt: %s", sdk.ErrInvalidArgument, s.Phase, strings.Join(hookapi.AllPhases, ", "))
	case !objectRe.MatchString(s.Callback):
		return sdk.Response{}, fmt.Errorf("%w: callback %q (eigenes Object mit Action %s)", sdk.ErrInvalidArgument, s.Callback, hookapi.CallbackAction)
	}
	if s.Priority == 0 {
		s.Priority = 100
	}
	now := ts()
	subscriber := p.pluginOf(s.Callback)
	res, err := p.pool().ExecContext(ctx, p.q(`UPDATE hook__subscriptions SET priority = ?, description = ?, subscriber = COALESCE(?, subscriber), last_seen = ?
		WHERE hook = ? AND phase = ? AND callback = ?`), s.Priority, s.Description, nullable(subscriber), now, s.Hook, s.Phase, s.Callback)
	if err != nil {
		return sdk.Response{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := p.pool().ExecContext(ctx, p.q(`INSERT INTO hook__subscriptions (id, hook, phase, callback, subscriber, priority, description, locked, registered_at, last_seen)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`), newID(), s.Hook, s.Phase, s.Callback, nullable(subscriber), s.Priority, s.Description, now, now); err != nil {
			return sdk.Response{}, err
		}
	}
	p.log(ctx, sdk.LogInfo, "Hook-Abo", map[string]string{"hook": s.Hook, "phase": s.Phase, "callback": s.Callback, "priority": strconv.Itoa(s.Priority)})
	return sdk.Response{Payload: map[string]any{"hook": s.Hook, "phase": s.Phase, "callback": s.Callback}}, nil
}

// pluginOf: Plugin, dem die Route <object>.onHook gehört ("" = noch keine Route).
func (p *Plugin) pluginOf(object string) string {
	if p.source == nil {
		return ""
	}
	for _, e := range p.source() {
		if e.Object == object && e.Action == hookapi.CallbackAction {
			return e.Plugin
		}
	}
	return ""
}

// --- Call ----------------------------------------------------------------------------

func (p *Plugin) call(ctx context.Context, payload any) (sdk.Response, error) {
	var in hookapi.Request
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	if !hookNameRe.MatchString(in.Hook) || !slices.Contains(hookapi.AllPhases, in.Action) {
		return sdk.Response{}, fmt.Errorf("%w: hook und action (Phase) sind Pflicht", sdk.ErrInvalidArgument)
	}
	h, err := p.loadHook(ctx, in.Hook)
	if err != nil {
		return sdk.Response{}, err
	}
	onFailure := hookapi.FailBlock
	if h != nil {
		if !slices.Contains(strings.Split(h.Phases, ","), in.Action) {
			return sdk.Response{}, fmt.Errorf("%w: Hook %s hat keine Phase %s", sdk.ErrInvalidArgument, in.Hook, in.Action)
		}
		onFailure = h.OnFailure
	}
	subs, err := p.loadSubs(ctx, "WHERE hook = ? AND phase = ? AND locked = 0 ORDER BY priority, registered_at, id", in.Hook, in.Action)
	if err != nil {
		return sdk.Response{}, err
	}
	out := hookapi.Result{Data: in.Data, Messages: []hookapi.Message{}}
	for _, s := range subs {
		if s.Subscriber == "" {
			s.Subscriber = p.pluginOf(s.Callback)
		}
		source := s.Callback
		if s.Subscriber != "" {
			source = s.Subscriber + "/" + s.Callback
		}
		resp, err := p.invoke(ctx, s.Callback, hookapi.Request{Hook: in.Hook, Action: in.Action, Data: out.Data})
		out.Subscribers++
		if err != nil {
			typ := hookapi.TypeError
			if onFailure == hookapi.FailSkip {
				typ = hookapi.TypeWarning
			}
			out.Messages = append(out.Messages, hookapi.Message{Type: typ, ID: "HOOK-001", Source: source,
				Text: fmt.Sprintf("Abonnent nicht erreichbar oder fehlerhaft: %v", err)})
			continue
		}
		for _, m := range resp.Messages {
			if m.Type != hookapi.TypeError && m.Type != hookapi.TypeWarning {
				m.Type = hookapi.TypeSuccess
			}
			m.Source = source
			out.Messages = append(out.Messages, m)
		}
		if in.Action == hookapi.PhaseModify && resp.ReturnData != nil {
			out.Data = resp.ReturnData
		}
	}
	// Nach dem Speichern lässt sich nichts mehr zurücknehmen: Fehler sind Warnungen.
	if in.Action == hookapi.PhaseCommit {
		for i, m := range out.Messages {
			if m.Type == hookapi.TypeError {
				out.Messages[i].Type = hookapi.TypeWarning
				out.Messages[i].Text += " (nach dem Speichern – bitte prüfen)"
			}
		}
	}
	return sdk.Response{Payload: out}, nil
}

// invoke ruft <callback>.onHook im Kontext der laufenden Anfrage auf.
func (p *Plugin) invoke(ctx context.Context, callback string, req hookapi.Request) (hookapi.Response, error) {
	if p.host == nil {
		return hookapi.Response{}, fmt.Errorf("%w: kein Host", sdk.ErrUnavailable)
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(p.settings.TimeoutSec)*time.Second)
	defer cancel()
	resp, err := p.host.Handle(cctx, sdk.Request{Object: callback, Action: hookapi.CallbackAction, Payload: req})
	if err != nil {
		return hookapi.Response{}, err
	}
	var out hookapi.Response
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return hookapi.Response{}, fmt.Errorf("Antwort: %v", err)
	}
	return out, nil
}

// --- Speicherung -----------------------------------------------------------------------

type hookRow struct {
	Name, Owner, Description, Phases, DataDoc, OnFailure, UpdatedAt string
	Defined                                                          bool
}

type subRow struct {
	ID, Hook, Phase, Callback, Subscriber, Description, LockedBy, LockedAt, RegisteredAt, LastSeen string
	Priority                                                                                       int
	Locked                                                                                         bool
}

func (p *Plugin) pool() *sql.DB {
	db, _ := p.db.DB(p.settings.Database)
	return db
}

func (p *Plugin) q(query string) string {
	return database.Rebind(p.db.Driver(p.settings.Database), query)
}

func (p *Plugin) loadHook(ctx context.Context, name string) (*hookRow, error) {
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT name, owner, description, phases, data_doc, on_failure, updated_at FROM hook__hooks WHERE name = ?`), name)
	if err != nil || len(res.Rows) == 0 {
		return nil, err
	}
	r := res.Rows[0]
	return &hookRow{Name: s(r[0]), Owner: s(r[1]), Description: s(r[2]), Phases: s(r[3]), DataDoc: s(r[4]), OnFailure: s(r[5]), UpdatedAt: s(r[6]), Defined: true}, nil
}

const subCols = `id, hook, phase, callback, subscriber, priority, description, locked, locked_by, locked_at, registered_at, last_seen`

func (p *Plugin) loadSubs(ctx context.Context, rest string, args ...any) ([]subRow, error) {
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT `+subCols+` FROM hook__subscriptions `+rest), args...)
	if err != nil {
		return nil, err
	}
	out := make([]subRow, len(res.Rows))
	for i, r := range res.Rows {
		prio, _ := strconv.Atoi(s(r[5]))
		locked, _ := strconv.Atoi(s(r[7]))
		out[i] = subRow{ID: s(r[0]), Hook: s(r[1]), Phase: s(r[2]), Callback: s(r[3]), Subscriber: s(r[4]), Priority: prio,
			Description: s(r[6]), Locked: locked != 0, LockedBy: s(r[8]), LockedAt: s(r[9]), RegisteredAt: s(r[10]), LastSeen: s(r[11])}
	}
	return out, nil
}

// --- Administration ------------------------------------------------------------------

func (p *Plugin) hookRecord(h hookRow, active int) map[string]any {
	return map[string]any{"id": h.Name, "name": h.Name, "owner": h.Owner, "description": h.Description,
		"phases": strings.ReplaceAll(h.Phases, ",", ", "), "on_failure": h.OnFailure, "data_doc": h.DataDoc,
		"subscriptions": active, "defined": h.Defined, "updated_at": h.UpdatedAt}
}

// hookList: definierte Hooks und Hooks, die nur abonniert sind (Besitzer noch
// nicht gestartet oder Name falsch geschrieben – Defined = false).
func (p *Plugin) hookList(ctx context.Context) (sdk.Response, error) {
	res, err := database.Query(ctx, p.pool(), `SELECT name, owner, description, phases, data_doc, on_failure, updated_at FROM hook__hooks ORDER BY name`)
	if err != nil {
		return sdk.Response{}, err
	}
	subs, err := p.loadSubs(ctx, "ORDER BY hook")
	if err != nil {
		return sdk.Response{}, err
	}
	active := map[string]int{}
	for _, sub := range subs {
		if !sub.Locked {
			active[sub.Hook]++
		}
	}
	items := []any{}
	seen := map[string]bool{}
	for _, r := range res.Rows {
		h := hookRow{Name: s(r[0]), Owner: s(r[1]), Description: s(r[2]), Phases: s(r[3]), DataDoc: s(r[4]), OnFailure: s(r[5]), UpdatedAt: s(r[6]), Defined: true}
		seen[h.Name] = true
		items = append(items, p.hookRecord(h, active[h.Name]))
	}
	for _, sub := range subs {
		if !seen[sub.Hook] {
			seen[sub.Hook] = true
			items = append(items, p.hookRecord(hookRow{Name: sub.Hook, Description: "nicht definiert (Besitzer nicht gestartet?)"}, active[sub.Hook]))
		}
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) hookGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	h, err := p.loadHook(ctx, id)
	if err != nil {
		return sdk.Response{}, err
	}
	if h == nil {
		h = &hookRow{Name: id, Description: "nicht definiert (Besitzer nicht gestartet?)"}
	}
	subs, err := p.loadSubs(ctx, "WHERE hook = ? AND locked = 0", id)
	if err != nil {
		return sdk.Response{}, err
	}
	if !h.Defined && len(subs) == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Hook %q", sdk.ErrNotFound, id)
	}
	return sdk.Response{Payload: p.hookRecord(*h, len(subs))}, nil
}

func (p *Plugin) subRecord(sub subRow) map[string]any {
	if sub.Subscriber == "" {
		sub.Subscriber = p.pluginOf(sub.Callback) // beim Abonnieren gab es die Route noch nicht
	}
	status := "active"
	switch {
	case sub.Locked:
		status = "locked"
	case p.source != nil && p.pluginOf(sub.Callback) == "":
		status = "unreachable"
	}
	// Nur die passende Aktion anbieten.
	hidden := []any{"lock"}
	if !sub.Locked {
		hidden = []any{"unlock"}
	}
	return map[string]any{"id": sub.ID, "hook": sub.Hook, "phase": sub.Phase, "priority": sub.Priority, "callback": sub.Callback,
		"subscriber": sub.Subscriber, "description": sub.Description, "status": status, "locked_by": sub.LockedBy,
		"locked_at": sub.LockedAt, "registered_at": sub.RegisteredAt, "last_seen": sub.LastSeen, "_hidden_actions": hidden}
}

func (p *Plugin) subList(ctx context.Context, payload any) (sdk.Response, error) {
	q := query(payload)
	var conds []string
	var args []any
	for _, k := range []string{"hook", "phase"} {
		if v := q[k]; v != "" {
			conds, args = append(conds, k+" = ?"), append(args, v)
		}
	}
	rest := ""
	if len(conds) > 0 {
		rest = "WHERE " + strings.Join(conds, " AND ")
	}
	subs, err := p.loadSubs(ctx, rest+" ORDER BY hook, CASE phase WHEN 'modify' THEN 1 WHEN 'check' THEN 2 ELSE 3 END, priority, registered_at", args...)
	if err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, len(subs))
	for i, sub := range subs {
		items[i] = p.subRecord(sub)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) subGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	subs, err := p.loadSubs(ctx, "WHERE id = ?", id)
	if err != nil {
		return sdk.Response{}, err
	}
	if len(subs) == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Abo %q", sdk.ErrNotFound, id)
	}
	return sdk.Response{Payload: p.subRecord(subs[0])}, nil
}

// setLocked sperrt bzw. entsperrt ein Abo (bleibt über Neustarts erhalten).
func (p *Plugin) setLocked(ctx context.Context, payload any, locked bool) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	var by, at any
	if locked {
		by, at = nullable(sdk.CallFromContext(ctx).UserID), ts()
	}
	res, err := p.pool().ExecContext(ctx, p.q(`UPDATE hook__subscriptions SET locked = ?, locked_by = ?, locked_at = ? WHERE id = ?`),
		map[bool]int{false: 0, true: 1}[locked], by, at, id)
	if err != nil {
		return sdk.Response{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Abo %q", sdk.ErrNotFound, id)
	}
	p.log(ctx, sdk.LogInfo, map[bool]string{true: "Hook-Abo gesperrt", false: "Hook-Abo entsperrt"}[locked], map[string]string{"id": id})
	msg := map[bool]string{true: "Abo gesperrt", false: "Abo entsperrt"}[locked]
	return sdk.Response{Payload: map[string]any{"message": msg}}, nil
}

// --- Hilfsfunktionen -------------------------------------------------------------------

func (p *Plugin) log(ctx context.Context, level sdk.LogLevel, msg string, fields map[string]string) {
	if p.host != nil {
		_ = p.host.Log(ctx, level, msg, fields)
	}
}

func ts() string { return time.Now().UTC().Format(time.RFC3339) }

func s(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	}
	return fmt.Sprint(v)
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func idParam(payload any) (string, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return "", err
	}
	if in.ID == "" {
		return "", fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	return in.ID, nil
}

// query liest Filter aus {"query": {...}} oder direkt aus dem Payload.
func query(payload any) map[string]string {
	m, _ := payload.(map[string]any)
	if q, ok := m["query"].(map[string]any); ok {
		m = q
	}
	out := map[string]string{}
	for k, v := range m {
		if str, ok := v.(string); ok {
			out[k] = str
		}
	}
	return out
}
