package crud

import (
	"context"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/module"
)

// Set sind die Entities eines Moduls mit gemeinsamer Datenbank.
type Set struct {
	entities []*Entity
	byObject map[string]*Entity
	db       module.DB
	services module.Services // Event-Dispatcher (Set.Events); nil = keine Events
	source   string
}

// NewSet fasst Entities zusammen (Reihenfolge = Navigation).
func NewSet(entities ...*Entity) *Set {
	s := &Set{byObject: map[string]*Entity{}}
	for _, e := range entities {
		if err := e.checkAccess(); err != nil {
			panic(err) // Programmierfehler im Modul – beim Start sichtbar
		}
		e.set = s
		s.entities = append(s.entities, e)
		s.byObject[e.Object] = e
	}
	return s
}

// Bind übernimmt die Datenbank des Moduls (in Initialize, vor der ersten Anfrage).
func (s *Set) Bind(db module.DB) { s.db = db }

// Events schaltet SystemEvents für Entities mit Events: true ein (in Initialize).
// source ist der Name des Moduls im Event.
func (s *Set) Events(services module.Services, source string) {
	s.services, s.source = services, source
}

// Entity liefert eine Entity über ihr Object (oder nil).
func (s *Set) Entity(object string) *Entity { return s.byObject[object] }

// Entities in Registrierungsreihenfolge.
func (s *Set) Entities() []*Entity { return s.entities }

// Register meldet je Entity list, get, create, update und – je nach
// Lebenszyklus – expire oder deactivate mit Metamodell an. defaultSection
// gilt für Entities ohne eigene Section.
func (s *Set) Register(r *module.Router, defaultSection string) {
	for _, e := range s.entities {
		section := e.Section
		if section == "" {
			section = defaultSection
		}
		o := r.Object(e.Object).Section(section).Describe(e.Definition()).
			Handle("list", payloadOnly(e.List)).
			Handle("get", payloadOnly(e.Get))
		if !e.ReadOnly {
			o.Handle("create", payloadOnly(e.Create)).Handle("update", payloadOnly(e.Update))
		}
		for _, a := range e.Actions {
			o.Handle(a.Name, e.guardRecord(a))
		}
		if e.FormState != nil {
			f := e.FormState
			o.Handle(formStateAction, func(ctx context.Context, req sdk.Request) (sdk.Response, error) {
				var in metamodel.FormStateRequest
				if err := sdk.Decode(req.Payload, &in); err != nil {
					return sdk.Response{}, err
				}
				if in.Values == nil {
					in.Values = map[string]string{}
				}
				st, err := f(ctx, in)
				return sdk.Response{Payload: st}, err
			})
		}
		switch e.Lifecycle().Kind() {
		case metamodel.LifecycleTimeSlice:
			o.Handle("expire", payloadOnly(e.Expire))
		case metamodel.LifecycleStatus:
			o.Handle("deactivate", payloadOnly(e.Deactivate))
		}
	}
}

// guardRecord: Eigene Actions auf einem Datensatz (Record) nur, wenn der
// Benutzer ihn sehen darf (Access.Records).
func (e *Entity) guardRecord(a Action) module.HandlerFunc {
	if !a.Record || e.Access == nil || !e.Access.Records {
		return a.Handle
	}
	return func(ctx context.Context, req sdk.Request) (sdk.Response, error) {
		key, err := e.ParseID(idOf(req.Payload))
		if err != nil {
			return sdk.Response{}, err
		}
		rec, err := e.Load(ctx, key)
		if err != nil {
			return sdk.Response{}, err
		}
		if _, err := e.checkReadable(ctx, rec); err != nil {
			return sdk.Response{}, err
		}
		return a.Handle(ctx, req)
	}
}

func payloadOnly(f func(ctx context.Context, payload any) (sdk.Response, error)) module.HandlerFunc {
	return func(ctx context.Context, req sdk.Request) (sdk.Response, error) { return f(ctx, req.Payload) }
}

// Definition liefert das Metamodell der Entity.
func (e *Entity) Definition() metamodel.ObjectDefinition {
	d := metamodel.ObjectDefinition{Name: e.Object, Title: e.Title, Icon: e.Icon, TitleField: e.TitleField, Sections: e.Sections, Authorization: e.authorization()}
	if e.FormState != nil {
		d.FormState = formStateAction
	}
	d.Search = len(e.Search) > 0
	for _, k := range e.Filters {
		if e.Field(k) != nil {
			d.Filters = append(d.Filters, k)
		}
	}
	for i := range e.Fields {
		f := &e.Fields[i]
		d.Fields = append(d.Fields, metamodel.FieldDefinition{
			Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required,
			Listable: f.Listable, Editable: !f.ReadOnly, Options: f.Options, Lookup: f.lookup(),
			Group: f.Group, Trigger: f.Trigger, ShowIf: f.ShowIf, RequiredIf: f.RequiredIf,
		})
	}
	d.Actions = []metamodel.ActionConfig{
		{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
		{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
	}
	if !e.ReadOnly {
		d.Actions = append(d.Actions,
			metamodel.ActionConfig{Name: "create", Kind: metamodel.KindCreate, Label: "Neu"},
			metamodel.ActionConfig{Name: "update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"})
	}
	for _, a := range e.Actions {
		if a.Kind == "" {
			a.Kind = metamodel.KindCustom
		}
		d.Actions = append(d.Actions, a.ActionConfig)
	}
	d.Lifecycle = e.Lifecycle()
	switch d.Lifecycle.Kind() {
	case metamodel.LifecycleTimeSlice:
		d.Actions = append(d.Actions, metamodel.ActionConfig{Name: "expire", Kind: metamodel.KindExpire, Label: "Beenden …"})
	case metamodel.LifecycleStatus:
		d.Actions = append(d.Actions, metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate, Label: "Inaktivieren", Confirm: e.Title + " inaktivieren?"})
	}
	return d
}

// formStateAction ist die Action des FormState-Hooks.
const formStateAction = "formState"
