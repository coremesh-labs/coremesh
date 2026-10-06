// Package event ist das Core-Plugin "event": der Event-Dispatcher des Systems.
//
// Object SystemEvent:
//   - Register {object, action, company_code, callback} – ein Plugin abonniert
//     Events zu Object/Action/Buchungskreis ("*" = alle). Zugestellt wird an
//     <callback>.onEvent; die Route muss beim Registrieren existieren.
//   - Push {object, action, company_code, entity_id, source, data} – ein Plugin
//     meldet die Änderung eines Datensatzes. Der Dispatcher ergänzt ID, Zeit,
//     Mandant, Benutzer und Request-ID und stellt asynchron an alle passenden
//     Abonnements zu.
//   - List – die aktuellen Abonnements (Diagnose).
//
// Zustellung: Worker-Pool mit Warteschlange im Speicher. Jede Zustellung ist
// eine eigene Systemanfrage (ohne Benutzer, Metadaten ingress=event); das
// Plugin braucht dazu ingress: true. Bei Unavailable/Unimplemented (Empfänger
// startet gerade neu) wird mit wachsendem Abstand wiederholt. Garantie: at most
// once, ohne Persistenz – Abonnements erneuern die Plugins bei jedem Start.
package event

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/camel/coremesh/internal/dispatcher"
	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/events"
)

const (
	Name    = "event"
	Version = "0.1.0"
)

// Source liefert die aktuell aufrufbaren Routen (dispatcher.Catalog).
type Source func() []dispatcher.Entry

type settings struct {
	Workers     int `json:"workers"`      // parallele Zustellungen (Standard 4)
	QueueSize   int `json:"queue_size"`   // Puffer der Warteschlange (Standard 1000)
	MaxAttempts int `json:"max_attempts"` // Versuche je Zustellung (Standard 3)
	TimeoutSec  int `json:"timeout_sec"`  // Zeitlimit je Zustellung (Standard 30)
}

type delivery struct {
	sub events.Subscription
	ev  events.Event
}

// Plugin ist der Event-Dispatcher.
type Plugin struct {
	source Source
	host   sdk.Host
	cfg    settings

	mu   sync.RWMutex
	subs []events.Subscription

	queue chan delivery
	wg    sync.WaitGroup
	once  sync.Once
	// retryDelay ist der Grundabstand zwischen Versuchen (Tests verkürzen ihn).
	retryDelay time.Duration
}

var (
	_ sdk.Plugin     = (*Plugin)(nil)
	_ sdk.Shutdowner = (*Plugin)(nil)

	objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	actionRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
)

func New(source Source) *Plugin { return &Plugin{source: source, retryDelay: 500 * time.Millisecond} }

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name: Name, Version: Version, Description: "Event-Dispatcher: Abonnements und asynchrone Zustellung von SystemEvents",
		Capabilities: []sdk.Capability{{Object: events.Object,
			Actions:     []string{events.ActionRegister, events.ActionPush, events.ActionList},
			Description: "Änderungen an Datensätzen melden und abonnieren"}},
	}, nil
}

func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	var s settings
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	s.Workers, s.QueueSize, s.MaxAttempts, s.TimeoutSec = orDefault(s.Workers, 4), orDefault(s.QueueSize, 1000), orDefault(s.MaxAttempts, 3), orDefault(s.TimeoutSec, 30)
	p.host, p.cfg = cfg.Host, s
	p.queue = make(chan delivery, s.QueueSize)
	for range s.Workers {
		p.wg.Add(1)
		go p.worker()
	}
	return nil
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// Shutdown nimmt keine Events mehr an und stellt die Warteschlange zu Ende zu
// (begrenzt durch ctx).
func (p *Plugin) Shutdown(ctx context.Context) error {
	p.once.Do(func() {
		if p.queue != nil {
			close(p.queue)
		}
	})
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Plugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch req.Action {
	case events.ActionRegister:
		return p.register(ctx, req.Payload)
	case events.ActionPush:
		return p.push(ctx, req.Payload)
	case events.ActionList:
		p.mu.RLock()
		defer p.mu.RUnlock()
		return sdk.Response{Payload: map[string]any{"subscriptions": slices.Clone(p.subs)}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, events.Object, req.Action)
}

func (p *Plugin) register(ctx context.Context, payload any) (sdk.Response, error) {
	var s events.Subscription
	if err := sdk.Decode(payload, &s); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	s.Object, s.Action, s.CompanyCode, s.Callback = strings.TrimSpace(s.Object), strings.TrimSpace(s.Action), strings.TrimSpace(s.CompanyCode), strings.TrimSpace(s.Callback)
	if s.CompanyCode == "" {
		s.CompanyCode = events.All
	}
	if s.Action == "" {
		s.Action = events.All
	}
	switch {
	case s.Object != events.All && !objectRe.MatchString(s.Object):
		return sdk.Response{}, fmt.Errorf("%w: object %q (Name eines Objects oder *)", sdk.ErrInvalidArgument, s.Object)
	case s.Action != events.All && !actionRe.MatchString(s.Action):
		return sdk.Response{}, fmt.Errorf("%w: action %q", sdk.ErrInvalidArgument, s.Action)
	case !objectRe.MatchString(s.Callback):
		return sdk.Response{}, fmt.Errorf("%w: callback %q (eigenes Object mit Action %s)", sdk.ErrInvalidArgument, s.Callback, events.CallbackAction)
	}
	// Plugins abonnieren meist in Configure/Initialize – ihre Routen meldet der Host
	// erst danach an. Fehlt die Route noch, gilt das Abonnement trotzdem (pending).
	pending := !p.routable(s.Callback)
	p.mu.Lock()
	if !slices.Contains(p.subs, s) {
		p.subs = append(p.subs, s)
	}
	n := len(p.subs)
	p.mu.Unlock()
	_ = p.log(ctx, sdk.LogInfo, "Event-Abonnement", map[string]string{"object": s.Object, "action": s.Action,
		"company_code": s.CompanyCode, "callback": s.Callback, "pending": fmt.Sprint(pending)})
	return sdk.Response{Payload: map[string]any{"subscription": s, "subscriptions": n, "pending": pending}}, nil
}

func (p *Plugin) routable(object string) bool {
	if p.source == nil {
		return true
	}
	return slices.ContainsFunc(p.source(), func(e dispatcher.Entry) bool { return e.Object == object && e.Action == events.CallbackAction })
}

func (p *Plugin) push(ctx context.Context, payload any) (sdk.Response, error) {
	var ev events.Event
	if err := sdk.Decode(payload, &ev); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	if !objectRe.MatchString(ev.Object) || !actionRe.MatchString(ev.Action) {
		return sdk.Response{}, fmt.Errorf("%w: object und action des Events sind Pflicht", sdk.ErrInvalidArgument)
	}
	call := sdk.CallFromContext(ctx)
	ev.ID = newID()
	ev.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	ev.TenantID, ev.UserID, ev.RequestID = call.TenantID, call.UserID, call.RequestID

	p.mu.RLock()
	var targets []events.Subscription
	for _, s := range p.subs {
		if matches(s.Object, ev.Object) && matches(s.Action, ev.Action) && matches(s.CompanyCode, ev.CompanyCode) {
			targets = append(targets, s)
		}
	}
	p.mu.RUnlock()

	queued := 0
	for _, s := range targets {
		select {
		case p.queue <- delivery{sub: s, ev: ev}:
			queued++
		default:
			_ = p.log(ctx, sdk.LogWarn, "Event verworfen: Warteschlange voll", map[string]string{"event": ev.ID, "callback": s.Callback})
		}
	}
	return sdk.Response{Payload: map[string]any{"event_id": ev.ID, "subscribers": queued}}, nil
}

func matches(pattern, v string) bool { return pattern == events.All || pattern == v }

func (p *Plugin) worker() {
	defer p.wg.Done()
	for d := range p.queue {
		p.deliver(d)
	}
}

// deliver stellt ein Event als eigene Systemanfrage zu (ohne Benutzer, damit der
// Empfänger unabhängig von den Rechten des Auslösers arbeitet).
func (p *Plugin) deliver(d delivery) {
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.cfg.TimeoutSec)*time.Second)
		ctx = sdk.WithCall(ctx, sdk.CallContext{TenantID: d.ev.TenantID,
			Metadata: map[string]string{"ingress": "event", "event_id": d.ev.ID}})
		_, err := p.host.Handle(ctx, sdk.Request{Object: d.sub.Callback, Action: events.CallbackAction, Payload: map[string]any{"event": d.ev}})
		cancel()
		if err == nil {
			return
		}
		retry := errors.Is(err, sdk.ErrUnavailable) || errors.Is(err, sdk.ErrUnimplemented)
		if !retry || attempt >= p.cfg.MaxAttempts {
			_ = p.log(context.Background(), sdk.LogWarn, "Event nicht zugestellt", map[string]string{"event": d.ev.ID,
				"object": d.ev.Object, "action": d.ev.Action, "callback": d.sub.Callback, "attempts": fmt.Sprint(attempt), "err": err.Error()})
			return
		}
		time.Sleep(time.Duration(attempt) * p.retryDelay)
	}
}

func (p *Plugin) log(ctx context.Context, level sdk.LogLevel, msg string, fields map[string]string) error {
	if p.host == nil {
		return nil
	}
	return p.host.Log(ctx, level, msg, fields)
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
