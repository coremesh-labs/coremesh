// Package events ist die Schnittstelle zum Event-Dispatcher (Core-Plugin event,
// Object SystemEvent). Plugins melden Änderungen an Bewegungsdaten (Push) und
// abonnieren Änderungen anderer Plugins (Register):
//
//	// Sender: nach dem Commit der eigenen Transaktion
//	events.Push(ctx, env.Services, events.Event{Object: "JournalEntry", Action: "post",
//		CompanyCode: "1000", EntityID: id, Data: map[string]any{"document_number": no}})
//
//	// Empfänger: beim Start abonnieren und eine Action onEvent bereitstellen
//	r.Object("RentContract").Handle(events.CallbackAction, m.onEvent)
//	events.Register(ctx, env.Services, events.Subscription{Object: "JournalEntry", Action: "*",
//		CompanyCode: "*", Callback: "RentContract"})
//
//	func (m *Module) onEvent(ctx context.Context, req sdk.Request) (sdk.Response, error) {
//		ev, err := events.Decode(req.Payload)
//		…
//	}
//
// Zustellung: asynchron nach Push, als Systemanfrage (ohne Benutzer; der
// auslösende Benutzer steht in Event.UserID), an <Callback>.onEvent jedes
// passenden Abonnements. Die Zustellung ist "at most once" mit Wiederholung
// bei vorübergehender Nichtverfügbarkeit, ohne Persistenz.
package events

import (
	"context"
	"errors"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/module"
)

// Object und Actions des Event-Dispatchers.
const (
	Object         = "SystemEvent"
	ActionRegister = "Register"
	ActionPush     = "Push"
	ActionList     = "List"

	// CallbackAction ist die Action, an die zugestellt wird – fest, damit
	// Abonnements keine beliebigen Actions mit Systemrechten auslösen können.
	CallbackAction = "onEvent"

	// All passt auf jedes Object, jede Action bzw. jeden Buchungskreis.
	All = "*"
)

// Subscription: Events zu Object/Action/Buchungskreis an Callback.onEvent.
type Subscription struct {
	Object      string `json:"object"`       // z. B. JournalEntry oder *
	Action      string `json:"action"`       // z. B. post oder *
	CompanyCode string `json:"company_code"` // Buchungskreis oder * (Standard)
	Callback    string `json:"callback"`     // eigenes Object mit Action onEvent
}

// Event ist eine Änderung an einem Datensatz.
type Event struct {
	ID          string         `json:"id"`
	Object      string         `json:"object"`
	Action      string         `json:"action"` // create, update, post, reverse, expire, deactivate …
	CompanyCode string         `json:"company_code,omitempty"`
	EntityID    string         `json:"entity_id,omitempty"`
	Source      string         `json:"source,omitempty"` // Modul, das das Event schickt
	OccurredAt  string         `json:"occurred_at"`      // RFC 3339, vom Dispatcher gesetzt
	TenantID    string         `json:"tenant_id,omitempty"`
	UserID      string         `json:"user_id,omitempty"` // auslösender Benutzer
	RequestID   string         `json:"request_id,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

// Register abonniert Events. Mehrfaches Registrieren desselben Abonnements ist
// unschädlich (z. B. nach einem Neustart des Plugins).
func Register(ctx context.Context, s module.Services, sub Subscription) error {
	_, err := s.Call(ctx, Object, ActionRegister, sub)
	return err
}

// Push meldet eine Änderung. Ist kein Event-Dispatcher konfiguriert, passiert
// nichts. Push gehört hinter den Commit der eigenen Transaktion, damit keine
// Events zu zurückgerollten Änderungen entstehen.
func Push(ctx context.Context, s module.Services, ev Event) error {
	_, err := s.Call(ctx, Object, ActionPush, ev)
	if errors.Is(err, sdk.ErrUnimplemented) {
		return nil
	}
	return err
}

// Decode liest das Event im Payload eines onEvent-Aufrufs ({"event": {…}}).
func Decode(payload any) (Event, error) {
	var in struct {
		Event Event `json:"event"`
	}
	err := sdk.Decode(payload, &in)
	return in.Event, err
}
