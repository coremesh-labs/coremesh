// Package crud ist eine tabellengesteuerte CRUD-Engine für Fachmodule.
//
// Eine Entity beschreibt Tabelle, Schlüssel und Felder eines Business-Objects.
// list/get/create/update, Typumwandlung, Pflichtfelder, Verweise (zeitbezogen
// geprüft), lesbare Labels, Zeitscheiben (valid_from als letzter
// Schlüsselteil, keine Überschneidungen), Historie und der Lebenszyklus
// (expire bzw. deactivate statt Löschen) sind generisch; Fachregeln hängen
// als Hooks an der Entity.
//
//	set := crud.NewSet(orders(), orderTypes())
//	func (m *Module) RegisterRoutes(r *module.Router) { set.Register(r, "Belege") }
//	func (m *Module) Initialize(ctx context.Context, env module.Env) error { set.Bind(env.DB); return nil }
//
// Konventionen der Payloads wie im WebServer: {query}, {id}, {data}, {id, data},
// {id, valid_to}. Datensätze tragen "_id" (Datensatz-ID, bei Zeitscheiben mit
// Beginndatum), "id" (fachlicher Schlüssel) und "_labels" (Texte der Verweise).
package crud

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/module"
)

// Record ist ein Datensatz.
type Record = map[string]any

// Field ist eine Spalte (oder ein virtuelles Feld) einer Entity.
type Field struct {
	Key, Label string
	Type       metamodel.FieldType
	Required   bool
	Listable   bool
	Immutable  bool // nach dem Anlegen nicht änderbar
	ReadOnly   bool // nur Anzeige (z. B. generierte id)
	Virtual    bool // nicht in der Tabelle (verarbeiten Hooks)
	Options    []metamodel.Option
	Ref        *Ref              // Verweis (prüfen, Label, Lookup)
	Lookup     *metamodel.Lookup // Lookup ohne Ref, z. B. auf ein Object eines anderen Moduls
	// Formular: Feldgruppe, Neuauswertung bei Änderung, deklarative Regeln.
	Group      string
	Trigger    bool
	ShowIf     *metamodel.Condition
	RequiredIf *metamodel.Condition
}

// Ref: Der Wert muss in Table.Column existieren; bei TimeSliced zusätzlich am
// Stichtag (valid_from des Datensatzes, sonst heute) gültig sein, bei
// ActiveField aktiv. Object und LabelFields ergeben ein Lookup im Metamodell
// und den lesbaren Text in "_labels".
type Ref struct {
	Table, Column, Label string
	TimeSliced           bool
	ActiveField          string // Ziel mit Status-Flag (boolean): nur aktive sind gültig
	Object               string
	LabelFields          []string
}

func (f *Field) lookup() *metamodel.Lookup {
	if f.Lookup != nil {
		return f.Lookup
	}
	if f.Ref == nil || f.Ref.Object == "" {
		return nil
	}
	return &metamodel.Lookup{Object: f.Ref.Object, ValueField: f.Ref.Column, LabelFields: f.Ref.LabelFields}
}

// Action ist eine eigene Action einer Entity. Ohne Kind gilt KindCustom; Name,
// Label, Fields und Record steuern Formular und Platz in der Oberfläche.
type Action struct {
	metamodel.ActionConfig
	Handle module.HandlerFunc
}

// Entity beschreibt ein Business-Object.
type Entity struct {
	Object, Title, Icon, Table string
	Section                    string   // Gruppe in der Modul-Navigation
	Keys                       []string // Primärschlüssel; bei TimeSlice endet er mit valid_from
	Surrogate                  bool     // Keys[0] = generierte id
	TimeSlice                  bool     // valid_from/valid_to (Lebenszyklus timeslice)
	StatusField                string   // Status-Feld (Lebenszyklus status); leer = keins
	// Werte des Status-Felds; Standard true/false (boolean). Bei einem
	// Auswahlfeld z. B. "ACTIVE"/"DEPRECATED".
	StatusActive, StatusInactive any
	Fields                       []Field
	Order                        string   // ORDER BY
	Filters                      []string // erlaubte Filter in list
	Search                       []string // Spalten für den Suchparameter q (LIKE)

	TitleField string
	Sections   []metamodel.SectionDefinition

	// ReadOnly: keine generischen create/update – Datensätze entstehen nur über
	// eigene Actions (z. B. Buchungen, die Soll und Haben gemeinsam prüfen).
	ReadOnly bool
	// Actions sind weitere Actions der Entity (Kind custom), z. B. post oder reverse.
	Actions []Action
	// Events: Bewegungsdaten – nach create, update, expire und deactivate geht ein
	// SystemEvent an den Event-Dispatcher (Set.Events). CompanyCodeField ist das Feld
	// des Buchungskreises im Event (Standard: company_code_id bzw. company_code).
	Events           bool
	CompanyCodeField string

	// Hooks
	Validate    func(ctx context.Context, rec, old Record) error // nach der Typprüfung, in der Transaktion
	AfterCreate func(ctx context.Context, rec Record) error      // in der Transaktion
	ListScope   func(ctx context.Context) (where string, args []any, none bool, err error)
	CheckRecord func(ctx context.Context, action string, rec Record) error // Zugriff je Datensatz
	Decorate    func(ctx context.Context, rec Record) error                // virtuelle Felder füllen
	// FormState bestimmt die Maske für die aktuellen Formularwerte (Action
	// formState, siehe metamodel.FormState).
	FormState func(ctx context.Context, req metamodel.FormStateRequest) (metamodel.FormState, error)
	// Authorization: Berechtigungsfelder und reine Berechtigungs-Actions des
	// Objects für die Rollenpflege (geprüft wird im Modul mit sdk.Authorize
	// bzw. sdk.Grants).
	Authorization *metamodel.Authorization

	set *Set
}

// DB ist die Datenbank der Entity (Set.Bind).
func (e *Entity) DB() module.DB { return e.set.db }

func (e *Entity) statusValues() (active, inactive any) {
	active, inactive = e.StatusActive, e.StatusInactive
	if active == nil {
		active = true
	}
	if inactive == nil {
		inactive = false
	}
	return active, inactive
}

// Field liefert ein Feld oder nil.
func (e *Entity) Field(key string) *Field {
	for i := range e.Fields {
		if e.Fields[i].Key == key {
			return &e.Fields[i]
		}
	}
	return nil
}

// Columns sind die Tabellenspalten (ohne virtuelle Felder).
func (e *Entity) Columns() []string {
	var out []string
	for _, f := range e.Fields {
		if !f.Virtual {
			out = append(out, f.Key)
		}
	}
	return out
}

// --- Schlüssel ---------------------------------------------------------------

// RecordID: einzelner Schlüssel = Wert; zusammengesetzt = Teile URL-kodiert mit "|".
func (e *Entity) RecordID(rec Record) string {
	parts := make([]string, len(e.Keys))
	for i, k := range e.Keys {
		parts[i] = Str(rec[k])
	}
	if len(parts) == 1 {
		return parts[0]
	}
	for i := range parts {
		parts[i] = url.QueryEscape(parts[i])
	}
	return strings.Join(parts, "|")
}

// BusinessKey ist der fachliche Schlüssel: bei Zeitscheiben ohne valid_from,
// zusammengesetzt wie RecordID.
func (e *Entity) BusinessKey(rec Record) string {
	keys := e.Keys
	if e.TimeSlice && len(keys) > 1 && keys[len(keys)-1] == "valid_from" {
		keys = keys[:len(keys)-1]
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = Str(rec[k])
	}
	if len(parts) == 1 {
		return parts[0]
	}
	for i := range parts {
		parts[i] = url.QueryEscape(parts[i])
	}
	return strings.Join(parts, "|")
}

// ParseID liest eine Datensatz-ID. Bei Zeitscheiben darf valid_from fehlen
// (nur fachlicher Schlüssel): dann gilt die heute gültige Zeitscheibe, sonst
// die jüngste (siehe Load).
func (e *Entity) ParseID(id string) (Record, error) {
	if id == "" {
		return nil, Invalid("id fehlt")
	}
	key := Record{}
	if len(e.Keys) == 1 {
		key[e.Keys[0]] = id
		return key, nil
	}
	parts := strings.Split(id, "|")
	n := len(e.Keys)
	if len(parts) != n && !(e.TimeSlice && len(parts) == n-1) {
		return nil, Invalid("id %q passt nicht zu %s", id, e.Object)
	}
	for i, p := range parts {
		v, err := url.QueryUnescape(p)
		if err != nil {
			return nil, Invalid("id %q: %v", id, err)
		}
		key[e.Keys[i]] = v
	}
	return key, nil
}

// KeyOf liefert den vollständigen Schlüssel eines geladenen Datensatzes.
func (e *Entity) KeyOf(rec Record) Record {
	key := Record{}
	for _, k := range e.Keys {
		key[k] = rec[k]
	}
	return key
}

func (e *Entity) keyWhere(key Record) (string, []any) {
	var conds []string
	var args []any
	for _, k := range e.Keys {
		if v, ok := key[k]; ok {
			conds, args = append(conds, k+" = ?"), append(args, v)
		}
	}
	return strings.Join(conds, " AND "), args
}

// --- Werte -------------------------------------------------------------------

// Coerce wandelt Eingaben (Formular, JSON, CLI) und Datenbankwerte in den Feldtyp.
func Coerce(f *Field, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok && strings.TrimSpace(s) == "" && f.Type != metamodel.TypeBoolean {
		return nil, nil
	}
	switch f.Type {
	case metamodel.TypeBoolean:
		switch b := v.(type) {
		case bool:
			return b, nil
		case int64:
			return b != 0, nil
		case float64:
			return b != 0, nil
		case string:
			switch strings.ToLower(strings.TrimSpace(b)) {
			case "true", "1", "on", "ja", "yes":
				return true, nil
			case "false", "0", "off", "nein", "no", "":
				return false, nil
			}
		}
		return nil, Invalid("%s: Ja/Nein erwartet", f.Label)
	case metamodel.TypeDate:
		d, err := ParseDate(v)
		if err != nil {
			return nil, Invalid("%s: %v", f.Label, err)
		}
		return d, nil
	case metamodel.TypeNumber:
		switch n := v.(type) {
		case int64, float64:
			return n, nil
		}
		s := strings.TrimSpace(Str(v))
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, nil
		}
		return nil, Invalid("%s: Zahl erwartet", f.Label)
	case metamodel.TypeSelect:
		s := Str(v)
		if !slices.ContainsFunc(f.Options, func(o metamodel.Option) bool { return o.Value == s }) {
			return nil, Invalid("%s: ungültiger Wert %q", f.Label, s)
		}
		return s, nil
	}
	return strings.TrimSpace(Str(v)), nil
}

// Str wandelt einen Wert in Text (ganze Zahlen ohne Nachkommastellen).
func Str(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
	}
	return fmt.Sprint(v)
}

// AsBool liest Ja/Nein-Werte tolerant.
func AsBool(v any) bool {
	b, _ := Coerce(&Field{Type: metamodel.TypeBoolean}, v)
	ok, _ := b.(bool)
	return ok
}

// NewID erzeugt eine zufällige ID (32 Hex-Zeichen).
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// idOf liest {"id": …}.
func idOf(payload any) string {
	m, _ := payload.(map[string]any)
	return Str(m["id"])
}

// listQuery: WebServer schickt {"query": {...}}, die CLI die Parameter direkt.
func listQuery(payload any) map[string]any {
	m, _ := payload.(map[string]any)
	if q, ok := m["query"].(map[string]any); ok {
		return q
	}
	if m == nil {
		return map[string]any{}
	}
	return m
}
