// Package hook ist die Schnittstelle zum Hook-Dispatcher (Core-Plugin hook,
// Object Hook): synchrone Erweiterungspunkte zwischen Modulen.
//
// Ein Modul definiert einen Hook (Name, Beschreibung, Phasen, Aufbau der
// Daten) und ruft ihn an festen Punkten seiner Verarbeitung auf. Andere Module
// abonnieren ihn je Phase; der Dispatcher ruft die Abonnenten der Reihe nach
// (Priorität aufsteigend) synchron auf und sammelt Daten und Meldungen:
//
//	modify  vor dem Speichern: Abonnenten liefern geänderte Daten (ReturnData);
//	        der nächste Abonnent erhält den geänderten Stand
//	check   vor dem Speichern: Meldungen vom Typ E brechen ab, W und S nicht
//	commit  nach dem Speichern: Folgeaktionen; Fehler können nichts mehr
//	        zurücknehmen und kommen als Warnung zurück
//
// Besitzer des Hooks:
//
//	hook.Define(ctx, env.Services, hook.Definition{Name: "ledger.posting",
//		Description: "Buchen eines Belegs", Phases: hook.AllPhases, Data: "ledgerapi.PostRequest …"})
//	res, err := hook.Call(ctx, env.Services, "ledger.posting", hook.PhaseCheck, req)
//	if err != nil { return err }
//	if res.HasErrors() { return res.Err() }
//
// Abonnent:
//
//	hook.Handle(r, "TaxCheck", m.onHook) // Route TaxCheck.onHook
//	hook.Subscribe(ctx, env.Services, hook.Subscription{Hook: "ledger.posting",
//		Phase: hook.PhaseCheck, Callback: "TaxCheck", Priority: 100})
//
//	func (m *Module) onHook(ctx context.Context, req hook.Request) (hook.Response, error) {
//		var in ledgerapi.PostRequest
//		if err := req.DecodeData(&in); err != nil { … }
//		return hook.Reply(nil, hook.Warning("TAX-1", "ohne Steuerkennzeichen")), nil
//	}
//
// Data und ReturnData sind beliebige JSON-Werte (über die Prozessgrenze als
// google.protobuf.Value). Aufgerufen wird im Kontext der auslösenden Anfrage
// (gleicher Benutzer, keine erneute Rechteprüfung). Abos lassen sich in der
// Administration sperren; die Sperre bleibt über Neustarts erhalten.
package hook

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/module"
)

// Object und Actions des Hook-Dispatchers.
const (
	Object          = "Hook"
	ActionDefine    = "Define"
	ActionSubscribe = "Subscribe"
	ActionCall      = "Call"

	// CallbackAction ist die Action, die der Dispatcher beim Abonnenten aufruft –
	// fest, damit Abos keine beliebigen Actions auslösen können.
	CallbackAction = "onHook"
)

// Phasen eines Hooks.
const (
	PhaseModify = "modify"
	PhaseCheck  = "check"
	PhaseCommit = "commit"
)

// AllPhases in der Reihenfolge der Verarbeitung.
var AllPhases = []string{PhaseModify, PhaseCheck, PhaseCommit}

// Verhalten, wenn ein Abonnent nicht erreichbar ist oder einen Fehler wirft.
const (
	FailBlock = "block" // wie eine Meldung E (in commit: W)
	FailSkip  = "skip"  // Warnung, weiter mit dem nächsten Abonnenten
)

// Definition eines Hooks (Catalog der Erweiterungspunkte).
type Definition struct {
	Name        string   `json:"name"`        // <modul>.<punkt>, z. B. ledger.posting
	Description string   `json:"description"` // wofür, wann aufgerufen
	Phases      []string `json:"phases"`      // Teilmenge von AllPhases
	Data        string   `json:"data"`        // Aufbau von Data (Vertrag zwischen den Modulen)
	OnFailure   string   `json:"on_failure"`  // FailBlock (Standard) oder FailSkip
	Owner       string   `json:"owner"`       // Modul, das den Hook aufruft
}

// Subscription: Abonnent eines Hooks in einer Phase.
type Subscription struct {
	Hook        string `json:"hook"`
	Phase       string `json:"phase"`
	Callback    string `json:"callback"` // eigenes Object mit Action onHook
	Priority    int    `json:"priority"` // aufsteigend; Standard 100
	Description string `json:"description"`
}

// Meldungstypen.
const (
	TypeError   = "E"
	TypeWarning = "W"
	TypeSuccess = "S"
)

// Message ist eine Zeile der Meldungstabelle.
type Message struct {
	Type   string `json:"type"`             // E, W, S
	ID     string `json:"id"`               // Meldungsnummer, z. B. TAX-001
	Text   string `json:"text"`             // lesbarer Text
	Field  string `json:"field,omitempty"`  // betroffenes Feld (optional)
	Source string `json:"source,omitempty"` // Abonnent (setzt der Dispatcher)
}

// Error, Warning, Info erzeugen Meldungen.
func Error(id, text string) Message   { return Message{Type: TypeError, ID: id, Text: text} }
func Warning(id, text string) Message { return Message{Type: TypeWarning, ID: id, Text: text} }
func Info(id, text string) Message    { return Message{Type: TypeSuccess, ID: id, Text: text} }

// OnField ordnet die Meldung einem Feld zu.
func (m Message) OnField(field string) Message { m.Field = field; return m }

// Request: Aufruf eines Abonnenten (Payload von <Callback>.onHook).
type Request struct {
	Hook   string `json:"hook"`
	Action string `json:"action"` // Phase: modify, check, commit
	Data   any    `json:"data"`
}

// DecodeData liest Data in v.
func (r Request) DecodeData(v any) error { return sdk.Decode(r.Data, v) }

// Response: Antwort eines Abonnenten.
type Response struct {
	ReturnData any       `json:"return_data,omitempty"` // modify: geänderte Daten (nil = unverändert)
	Messages   []Message `json:"messages,omitempty"`
}

// Reply baut eine Antwort.
func Reply(returnData any, msgs ...Message) Response {
	return Response{ReturnData: returnData, Messages: msgs}
}

// Result: Ergebnis eines Hook-Aufrufs für den Besitzer.
type Result struct {
	Data        any       `json:"data"`        // nach modify: geänderte Daten, sonst die Eingabe
	Messages    []Message `json:"messages"`    // alle Meldungen aller Abonnenten
	Subscribers int       `json:"subscribers"` // aufgerufene Abonnenten
}

// HasErrors meldet Meldungen vom Typ E.
func (r Result) HasErrors() bool {
	for _, m := range r.Messages {
		if m.Type == TypeError {
			return true
		}
	}
	return false
}

// Filter liefert die Meldungen eines Typs.
func (r Result) Filter(typ string) []Message {
	var out []Message
	for _, m := range r.Messages {
		if m.Type == typ {
			out = append(out, m)
		}
	}
	return out
}

// Err fasst die Fehlermeldungen als sdk.ErrInvalidArgument zusammen (nil ohne E).
func (r Result) Err() error {
	errs := r.Filter(TypeError)
	if len(errs) == 0 {
		return nil
	}
	texts := make([]string, len(errs))
	for i, m := range errs {
		texts[i] = m.String()
	}
	return fmt.Errorf("%w: %s", sdk.ErrInvalidArgument, strings.Join(texts, "; "))
}

// DecodeData liest die (geänderten) Daten in v.
func (r Result) DecodeData(v any) error { return sdk.Decode(r.Data, v) }

// String: "TAX-1 ohne Steuerkennzeichen (tax: Steuer)".
func (m Message) String() string {
	s := m.Text
	if m.ID != "" {
		s = m.ID + " " + s
	}
	if m.Source != "" {
		s += " (" + m.Source + ")"
	}
	return s
}

// Define meldet einen Hook an (beim Start des Besitzers; wiederholbar).
func Define(ctx context.Context, s module.Services, d Definition) error {
	_, err := s.Call(ctx, Object, ActionDefine, d)
	return err
}

// Subscribe abonniert einen Hook in einer Phase (beim Start; wiederholbar –
// eine Sperre aus der Administration bleibt erhalten).
func Subscribe(ctx context.Context, s module.Services, sub Subscription) error {
	_, err := s.Call(ctx, Object, ActionSubscribe, sub)
	return err
}

// Call ruft die Abonnenten eines Hooks in einer Phase auf. Ohne
// Hook-Dispatcher (nicht konfiguriert) kommen die Daten unverändert zurück.
func Call(ctx context.Context, s module.Services, name, phase string, data any) (Result, error) {
	resp, err := s.Call(ctx, Object, ActionCall, map[string]any{"hook": name, "action": phase, "data": data})
	if errors.Is(err, sdk.ErrUnimplemented) {
		return Result{Data: data}, nil
	}
	if err != nil {
		return Result{}, err
	}
	var r Result
	if err := sdk.Decode(resp.Payload, &r); err != nil {
		return Result{}, err
	}
	return r, nil
}

// Decode liest den Payload eines onHook-Aufrufs.
func Decode(payload any) (Request, error) {
	var r Request
	err := sdk.Decode(payload, &r)
	return r, err
}

// Handler verarbeitet einen Hook-Aufruf beim Abonnenten.
type Handler func(ctx context.Context, req Request) (Response, error)

// Handle meldet die Route <callback>.onHook im Router an.
func Handle(r *module.Router, callback string, h Handler) {
	r.Object(callback).Handle(CallbackAction, Func(h))
}

// Func macht aus einem Handler eine Route (für Plugins ohne module.Router).
func Func(h Handler) module.HandlerFunc {
	return func(ctx context.Context, req sdk.Request) (sdk.Response, error) {
		in, err := Decode(req.Payload)
		if err != nil {
			return sdk.Response{}, err
		}
		out, err := h(ctx, in)
		if err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: out}, nil
	}
}
