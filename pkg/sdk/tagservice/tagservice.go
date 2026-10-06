// Package tagservice ist die Service-Schnittstelle des TagManagements
// (Plugin tag, Modul tagmanagement). Andere Plugins erweitern ihre Objects
// damit um Tags – ausschließlich über diese Schnittstelle, nie über die
// Tabellen des Plugins:
//
//	tags := tagservice.New(env.Services)
//	et, err := tags.Get(ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EntityID: id})
//	_, err = tags.Set(ctx, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: id,
//		Values: map[string]*tagservice.Value{"RISK_CLASS": tagservice.Option("HIGH")}})
//
// Der Client ruft die Actions des Objects Tags über den Dispatcher auf
// (Tags.schema, Tags.get, Tags.set, Tags.validate, Tags.history, Tags.find).
// Dieselben Payloads gelten für die JSON-API (/api/v1/tagmanagement/Tags/…)
// – eine spätere Auslagerung als eigener Dienst ändert den Vertrag nicht.
//
// Buchungskreis: Tag Sets werden einem Objekttyp je Buchungskreis zugewiesen
// (oder mit "*" für alle). Aufrufe mit CompanyCode erhalten die globalen und
// die Tag Sets dieses Buchungskreises; derselbe Datensatz kann so je
// Buchungskreis andere Tags und Werte haben. Ohne CompanyCode gelten nur die
// globalen Tag Sets. Gehört ein Tag zu beiden, gilt der Buchungskreis.
//
// Stichtag: Alle lesenden Aufrufe akzeptieren EffectiveDate (JJJJ-MM-TT,
// Standard heute). Gelesen werden die Tag Sets, Zuordnungen, Auswahlwerte
// und Werte, deren Zeitscheibe den Stichtag enthält.
package tagservice

import (
	"context"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/module"
)

// Object ist das Business-Object des TagService im Dispatcher.
const Object = "Tags"

// AllCompanyCodes kennzeichnet Zuordnungen und Werte, die in allen Buchungskreisen gelten.
const AllCompanyCodes = "*"

// DataType ist der Datentyp eines Tags.
type DataType string

const (
	TypeString    DataType = "STRING"
	TypeInteger   DataType = "INTEGER"
	TypeCurrency  DataType = "CURRENCY" // Betrag (Dezimal) + Währung (ISO 4217)
	TypeDate      DataType = "DATE"     // JJJJ-MM-TT
	TypeTimestamp DataType = "TIMESTAMP"
	// TypeReference verweist auf den fachlichen Schlüssel (id) eines Datensatzes
	// eines anderen Objects (TagType.RefObject), z. B. ein Mietobjekt.
	TypeReference DataType = "REFERENCE"
)

// ValueMode: freie Eingabe oder vordefinierte Auswahlwerte.
type ValueMode string

const (
	ModeFree    ValueMode = "FREE"
	ModeOptions ValueMode = "OPTIONS"
)

// RuleType ist eine logische Abhängigkeit innerhalb eines Tag Sets.
type RuleType string

const (
	// RuleRequires: Hat Source einen Wert (bzw. den Wert Condition), braucht Target einen.
	RuleRequires RuleType = "REQUIRES"
	// RuleExcludes: Hat Source einen Wert (bzw. Condition), darf Target keinen haben.
	RuleExcludes RuleType = "EXCLUDES"
	// RuleShowIf: Target ist nur sichtbar und erlaubt, wenn Source einen Wert (bzw. Condition) hat.
	RuleShowIf RuleType = "SHOW_IF"
)

// Value ist ein typisierter Tag-Wert; genau ein Feld ist gesetzt (bei
// CURRENCY Amount und Currency). Bei Tags mit Auswahlwerten ist Option der
// Code des Auswahlwerts.
type Value struct {
	String    *string `json:"string,omitempty"`
	Integer   *int64  `json:"integer,omitempty"`
	Amount    *string `json:"amount,omitempty"`    // Dezimalzahl als Text, z. B. "1250.50"
	Currency  *string `json:"currency,omitempty"`  // ISO 4217, z. B. "EUR"
	Date      *string `json:"date,omitempty"`      // JJJJ-MM-TT
	Timestamp *string `json:"timestamp,omitempty"` // RFC 3339, gespeichert in UTC
	Option    *string `json:"option,omitempty"`    // Code eines TagValueOption
	Ref       *string `json:"ref,omitempty"`       // REFERENCE: id des Datensatzes von TagType.RefObject
}

// Konstruktoren für Werte.
func String(s string) *Value               { return &Value{String: &s} }
func Integer(i int64) *Value               { return &Value{Integer: &i} }
func Money(amount, currency string) *Value { return &Value{Amount: &amount, Currency: &currency} }
func Date(d string) *Value                 { return &Value{Date: &d} }
func Timestamp(ts string) *Value           { return &Value{Timestamp: &ts} }
func Option(code string) *Value            { return &Value{Option: &code} }
func Ref(id string) *Value                 { return &Value{Ref: &id} }

// TagType ist die Definition eines Tags (ohne Zeitscheibe; Status ACTIVE/DEPRECATED).
type TagType struct {
	Code           string        `json:"code"`
	Name           string        `json:"name"` // Text in der angefragten Sprache (sonst Rückfall)
	TranslationKey string        `json:"translation_key,omitempty"`
	DataType       DataType      `json:"data_type"`
	ValueMode      ValueMode     `json:"value_mode"`
	RefObject      string        `json:"ref_object,omitempty"` // REFERENCE: Object des Ziels, z. B. "RentalObject"
	Status         string        `json:"status"`               // ACTIVE | DEPRECATED
	Options        []ValueOption `json:"options,omitempty"`    // am Stichtag gültig
}

// ValueOption ist ein vordefinierter Auswahlwert.
type ValueOption struct {
	Code           string `json:"code"`
	Label          string `json:"label"`
	TranslationKey string `json:"translation_key,omitempty"`
	ValidFrom      string `json:"valid_from"`
	ValidTo        string `json:"valid_to"`
}

// Rule ist eine Regel eines Tag Sets.
type Rule struct {
	Type      RuleType `json:"type"`
	Source    string   `json:"source"`              // Tag-Code
	Target    string   `json:"target"`              // Tag-Code
	Condition string   `json:"condition,omitempty"` // optional: Wert (bzw. Options-Code) von Source
}

// SetItem ist ein Tag eines Tag Sets.
type SetItem struct {
	Tag       TagType `json:"tag"`
	Mandatory bool    `json:"mandatory"`
	SortOrder int     `json:"sort_order"`
	// Scope: Buchungskreis, unter dem der Wert gespeichert wird ("*" = alle).
	Scope string `json:"scope"`
}

// TagSet ist ein Tag Set, wie es am Stichtag gilt.
type TagSet struct {
	Code        string `json:"code"`
	CompanyCode string `json:"company_code"` // Zuordnung: Buchungskreis oder "*"
	// Condition: Das Set gilt nur für Datensätze, deren Feld einen der Werte hat
	// (z. B. contract_type in RENT, LEASE). nil = für alle Datensätze des Objekttyps.
	Condition      *SetCondition `json:"condition,omitempty"`
	Name           string        `json:"name"`
	TranslationKey string        `json:"translation_key,omitempty"`
	Items          []SetItem     `json:"items"`
	Rules          []Rule        `json:"rules"`
}

// SetCondition schränkt die Zuordnung eines Tag Sets auf Datensätze mit bestimmten
// Feldwerten ein (ODER-Verknüpfung der Werte).
type SetCondition struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// Schema sind die Tag Sets eines Objekttyps am Stichtag (für Eingabemasken).
type Schema struct {
	EntityType    string   `json:"entity_type"`
	CompanyCode   string   `json:"company_code,omitempty"`
	EffectiveDate string   `json:"effective_date"`
	Sets          []TagSet `json:"sets"`
}

// Assignment ist ein Wert eines Tags für ein Objekt, mit Zeitscheibe.
type Assignment struct {
	ID          string `json:"id"`
	Tag         string `json:"tag"`
	CompanyCode string `json:"company_code"` // Buchungskreis des Werts oder "*"
	Value       Value  `json:"value"`
	OptionLabel string `json:"option_label,omitempty"`
	RefLabel    string `json:"ref_label,omitempty"` // REFERENCE: lesbarer Text des Ziels
	ValidFrom   string `json:"valid_from"`
	ValidTo     string `json:"valid_to"`
}

// State ist der Zustand der Regeln für die aktuellen Werte.
type State struct {
	Visible  map[string]bool `json:"visible"`  // Tag sichtbar (SHOW_IF erfüllt)
	Required map[string]bool `json:"required"` // Pflicht (mandatory oder REQUIRES)
}

// EntityTags sind die Tags eines Objekts am Stichtag.
type EntityTags struct {
	Schema
	EntityID string       `json:"entity_id"`
	Values   []Assignment `json:"values"`
	State    State        `json:"state"`
}

// Violation ist ein Verstoß gegen Datentyp, Auswahlwerte oder Regeln.
type Violation struct {
	Tag     string `json:"tag"`
	Code    string `json:"code"` // z. B. "required", "type", "option", "excludes", "hidden"
	Message string `json:"message"`
}

// GetRequest liest die Tags eines Objekts.
type GetRequest struct {
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	CompanyCode   string `json:"company_code,omitempty"` // leer = nur globale Tag Sets
	EffectiveDate string `json:"effective_date,omitempty"`
	Locale        string `json:"locale,omitempty"` // Sprache für Name/Label (de, en, zh-CN)
	// Attributes: Feldwerte für Tag Sets mit Bedingung, wenn es den Datensatz noch
	// nicht gibt (Tags.schema, z. B. {"contract_type": "RENT"}). Mit EntityID gelten die
	// Werte des Datensatzes; ohne beides liefert Tags.schema alle Sets samt Bedingung.
	Attributes map[string]string `json:"attributes,omitempty"`
}

// SetRequest setzt Werte ab ValidFrom (Standard heute). Ein Wert nil beendet
// das Tag zum Vortag von ValidFrom. Nicht genannte Tags bleiben unverändert.
type SetRequest struct {
	EntityType  string            `json:"entity_type"`
	EntityID    string            `json:"entity_id"`
	CompanyCode string            `json:"company_code,omitempty"`
	ValidFrom   string            `json:"valid_from,omitempty"`
	Values      map[string]*Value `json:"values"`
	Locale      string            `json:"locale,omitempty"`
}

// FindRequest sucht Objekte mit einem Tag (und optional einem Wert) am Stichtag.
type FindRequest struct {
	EntityType    string `json:"entity_type"`
	CompanyCode   string `json:"company_code,omitempty"`
	Tag           string `json:"tag"`
	Value         *Value `json:"value,omitempty"`
	EffectiveDate string `json:"effective_date,omitempty"`
}

// Service ist die Schnittstelle des TagManagements.
type Service interface {
	Schema(ctx context.Context, req GetRequest) (Schema, error) // ohne EntityID
	Get(ctx context.Context, req GetRequest) (EntityTags, error)
	Set(ctx context.Context, req SetRequest) (EntityTags, error)
	Validate(ctx context.Context, req SetRequest) ([]Violation, error)
	History(ctx context.Context, req GetRequest, tag string) ([]Assignment, error)
	Find(ctx context.Context, req FindRequest) ([]string, error)
}

// New liefert einen Client über die Services des Moduls (Dispatcher).
func New(services module.Services) Service { return client{services} }

type client struct{ s module.Services }

func (c client) call(ctx context.Context, action string, payload, out any) error {
	resp, err := c.s.Call(ctx, Object, action, payload)
	if err != nil {
		return err
	}
	return sdk.Decode(resp.Payload, out)
}

func (c client) Schema(ctx context.Context, req GetRequest) (Schema, error) {
	var out Schema
	err := c.call(ctx, "schema", req, &out)
	return out, err
}

func (c client) Get(ctx context.Context, req GetRequest) (EntityTags, error) {
	var out EntityTags
	err := c.call(ctx, "get", req, &out)
	return out, err
}

func (c client) Set(ctx context.Context, req SetRequest) (EntityTags, error) {
	var out EntityTags
	err := c.call(ctx, "set", req, &out)
	return out, err
}

func (c client) Validate(ctx context.Context, req SetRequest) ([]Violation, error) {
	var out struct {
		Violations []Violation `json:"violations"`
	}
	err := c.call(ctx, "validate", req, &out)
	return out.Violations, err
}

func (c client) History(ctx context.Context, req GetRequest, tag string) ([]Assignment, error) {
	var out struct {
		Values []Assignment `json:"values"`
	}
	err := c.call(ctx, "history", map[string]any{"entity_type": req.EntityType, "entity_id": req.EntityID, "company_code": req.CompanyCode, "tag": tag}, &out)
	return out.Values, err
}

func (c client) Find(ctx context.Context, req FindRequest) ([]string, error) {
	var out struct {
		EntityIDs []string `json:"entity_ids"`
	}
	err := c.call(ctx, "find", req, &out)
	return out.EntityIDs, err
}
