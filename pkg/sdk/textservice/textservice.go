// Package textservice ist die Service-Schnittstelle der Texte (Plugin tag,
// Modul textmanagement): Anmerkungen, Beschreibungen und andere Texte an jedem
// Datensatz jedes Objects – Vertrag, Mietobjekt, Gebäude …
//
// Ein Text hat eine Kategorie (Textart), eine Position und einen Sprachschlüssel.
// Die Kategorie legt fest, ob ein Datensatz sie einmal (Position 1) oder mehrfach
// (Positionen 1, 2, …) haben kann; je Position gibt es den Text in mehreren
// Sprachen. Welche Kategorien an welchem Object (und Datensatz, über eine
// Bedingung auf ein Feld) erlaubt sind, steht in den Zuordnungen des Moduls.
//
// Andere Plugins und der WebServer sprechen nur mit dem Object Texts (list,
// save, remove). Ziel ist der fachliche Schlüssel des Datensatzes. Lesen darf,
// wer den Datensatz lesen darf; schreiben, wer ihn ändern darf
// (<Object>.update im Buchungskreis des Datensatzes).
//
//	texts := textservice.New(env.Services)
//	l, err := texts.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-2026-0003"})
//	_, err = texts.Save(ctx, textservice.Text{EntityType: "Contract", EntityID: "1000|MV-2026-0003",
//		Category: "NOTE", Language: "de", Content: "Schlüssel beim Hausmeister"})
package textservice

import (
	"context"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Object ist das Business-Object der Texte im Dispatcher.
const Object = "Texts"

// Actions.
const (
	ActionList   = "list"
	ActionSave   = "save"
	ActionRemove = "remove"
	ActionCheck  = "check"
)

// Häufigkeit einer Kategorie.
const (
	Single   = "SINGLE"   // einmal je Datensatz (Position 1)
	Multiple = "MULTIPLE" // mehrfach (Positionen 1, 2, …)
)

// Text ist ein Text an einem Datensatz.
type Text struct {
	ID           string `json:"id,omitempty"`
	EntityType   string `json:"entity_type,omitempty"`
	EntityID     string `json:"entity_id,omitempty"`
	Category     string `json:"category"`
	CategoryName string `json:"category_name,omitempty"` // Antwort
	Position     int    `json:"position,omitempty"`      // 0 beim Anlegen = nächste freie (einmalig: 1)
	Language     string `json:"language"`                // Sprachschlüssel, z. B. de, en
	Content      string `json:"content"`
	ChangedAt    string `json:"changed_at,omitempty"`
	ChangedBy    string `json:"changed_by,omitempty"`
}

// Category ist eine am Datensatz erlaubte Kategorie; Pflicht = mindestens ein
// Text der Kategorie (in irgendeiner Sprache) muss am Datensatz stehen.
type Category struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Cardinality string `json:"cardinality"` // SINGLE | MULTIPLE
	Mandatory   bool   `json:"mandatory"`
}

// Language ist ein Sprachschlüssel.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ListRequest: Texte eines Datensatzes; Language (optional) nur in dieser Sprache.
type ListRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Language   string `json:"language,omitempty"`
}

// ListResponse: Texte (Kategorie, Position, Sprache), ob der Benutzer ändern darf,
// die am Datensatz erlaubten Kategorien, die Sprachen und die fehlenden Pflichtkategorien.
type ListResponse struct {
	Items      []Text     `json:"items"`
	CanEdit    bool       `json:"can_edit"`
	Categories []Category `json:"categories"`
	Languages  []Language `json:"languages"`
	Missing    []Category `json:"missing"`
}

// CheckRequest: fehlende Pflichttexte eines Datensatzes (z. B. vor dem Aktivieren).
type CheckRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

// CheckResponse: Pflichtkategorien ohne Text (leer = vollständig).
type CheckResponse struct {
	Missing []Category `json:"missing"`
}

// RemoveRequest: Text entfernen.
type RemoveRequest struct {
	ID string `json:"id"`
}

// Client ruft die Actions des Objects Texts auf.
type Client struct{ s module.Services }

func New(s module.Services) Client { return Client{s: s} }

func (c Client) List(ctx context.Context, r ListRequest) (ListResponse, error) {
	var out ListResponse
	return out, c.call(ctx, ActionList, r, &out)
}

// Save legt einen Text an (ohne ID) oder ändert ihn (mit ID: Inhalt, Position, Sprache).
func (c Client) Save(ctx context.Context, t Text) (Text, error) {
	var out Text
	return out, c.call(ctx, ActionSave, t, &out)
}

// Check liefert die Pflichtkategorien ohne Text.
func (c Client) Check(ctx context.Context, r CheckRequest) (CheckResponse, error) {
	var out CheckResponse
	return out, c.call(ctx, ActionCheck, r, &out)
}

func (c Client) Remove(ctx context.Context, r RemoveRequest) error {
	return c.call(ctx, ActionRemove, r, nil)
}

func (c Client) call(ctx context.Context, action string, payload, out any) error {
	resp, err := c.s.Call(ctx, Object, action, payload)
	if err != nil || out == nil {
		return err
	}
	return sdk.Decode(resp.Payload, out)
}
