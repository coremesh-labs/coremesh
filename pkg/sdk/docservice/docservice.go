// Package docservice ist die Service-Schnittstelle der Dokumentverweise
// (Plugin document, Modul documents): beliebig viele Dokumente an jedem
// Datensatz jedes Objects – Vertrag, AGB, Nachtrag, Rechnung, Bescheid …
//
// Andere Plugins und der WebServer sprechen nur mit dem Object Documents
// (list, attach, update, detach, types). Ein Dokumentenmanagementsystem
// ersetzt das Plugin, indem ein Adapter-Plugin dasselbe Object mit denselben
// Payloads anbietet.
//
//	docs := docservice.New(env.Services)
//	l, err := docs.List(ctx, docservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-2026-0003"})
//	_, err = docs.Attach(ctx, docservice.AttachRequest{EntityType: "Contract", EntityID: "1000|MV-2026-0003",
//		Document: docservice.Document{DocType: "AGB", Title: "AGB Stand 03/2026", Location: "vertraege/mv-0003/agb.pdf"}})
//
// Ziel ist der fachliche Schlüssel des Datensatzes – bei Zeitscheiben ohne
// Beginndatum; ValidFrom/ValidTo sagen, für welchen Zeitraum ein Dokument gilt
// (z. B. Nachtrag ab 2027). Lesen darf, wer den Datensatz lesen darf
// (<Object>.get); anhängen, ändern und entfernen, wer ihn ändern darf
// (<Object>.update im Buchungskreis des Datensatzes).
package docservice

import (
	"context"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Object ist das Business-Object der Dokumentverweise im Dispatcher.
const Object = "Documents"

// Actions.
const (
	ActionList   = "list"
	ActionAttach = "attach"
	ActionUpdate = "update"
	ActionDetach = "detach"
	ActionTypes  = "types"
)

// Document ist ein Dokumentverweis.
type Document struct {
	ID          string `json:"id,omitempty"`
	EntityType  string `json:"entity_type,omitempty"`
	EntityID    string `json:"entity_id,omitempty"`
	DocType     string `json:"doc_type"`                // Dokumentart (Katalog), z. B. CONTRACT, AGB
	DocTypeName string `json:"doc_type_name,omitempty"` // Bezeichnung der Dokumentart (Antwort)
	Title       string `json:"title"`
	DocDate     string `json:"doc_date,omitempty"`   // JJJJ-MM-TT
	Location    string `json:"location"`             // Dateiname, Ablageort oder Link (DMS: Dokument-ID)
	ValidFrom   string `json:"valid_from,omitempty"` // gilt ab (leer = immer)
	ValidTo     string `json:"valid_to,omitempty"`
	Note        string `json:"note,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
}

// DocType ist eine Dokumentart.
type DocType struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ListRequest: Dokumente eines Datensatzes; EffectiveDate (optional) liefert
// nur die an diesem Tag gültigen.
type ListRequest struct {
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	EffectiveDate string `json:"effective_date,omitempty"`
}

// ListResponse: Dokumente (neueste zuerst), ob der Benutzer ändern darf, Dokumentarten.
type ListResponse struct {
	Items   []Document `json:"items"`
	CanEdit bool       `json:"can_edit"`
	Types   []DocType  `json:"types"`
}

type AttachRequest struct {
	EntityType string   `json:"entity_type"`
	EntityID   string   `json:"entity_id"`
	Document   Document `json:"document"`
}

type UpdateRequest struct {
	ID       string   `json:"id"`
	Document Document `json:"document"`
}

type DetachRequest struct {
	ID string `json:"id"`
}

// Client ruft die Actions des Objects Documents auf.
type Client struct{ s module.Services }

func New(s module.Services) Client { return Client{s: s} }

func (c Client) List(ctx context.Context, r ListRequest) (ListResponse, error) {
	var out ListResponse
	return out, c.call(ctx, ActionList, r, &out)
}

func (c Client) Attach(ctx context.Context, r AttachRequest) (Document, error) {
	var out Document
	return out, c.call(ctx, ActionAttach, r, &out)
}

func (c Client) Update(ctx context.Context, r UpdateRequest) (Document, error) {
	var out Document
	return out, c.call(ctx, ActionUpdate, r, &out)
}

func (c Client) Detach(ctx context.Context, r DetachRequest) error {
	return c.call(ctx, ActionDetach, r, nil)
}

func (c Client) call(ctx context.Context, action string, payload, out any) error {
	resp, err := c.s.Call(ctx, Object, action, payload)
	if err != nil || out == nil {
		return err
	}
	return sdk.Decode(resp.Payload, out)
}
