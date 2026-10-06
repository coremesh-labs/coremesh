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
}

// NewSet fasst Entities zusammen (Reihenfolge = Navigation).
func NewSet(entities ...*Entity) *Set {
	s := &Set{byObject: map[string]*Entity{}}
	for _, e := range entities {
		e.set = s
		s.entities = append(s.entities, e)
		s.byObject[e.Object] = e
	}
	return s
}

// Bind übernimmt die Datenbank des Moduls (in Initialize, vor der ersten Anfrage).
func (s *Set) Bind(db module.DB) { s.db = db }

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
			Handle("get", payloadOnly(e.Get)).
			Handle("create", payloadOnly(e.Create)).
			Handle("update", payloadOnly(e.Update))
		switch e.Lifecycle().Kind() {
		case metamodel.LifecycleTimeSlice:
			o.Handle("expire", payloadOnly(e.Expire))
		case metamodel.LifecycleStatus:
			o.Handle("deactivate", payloadOnly(e.Deactivate))
		}
	}
}

func payloadOnly(f func(ctx context.Context, payload any) (sdk.Response, error)) module.HandlerFunc {
	return func(ctx context.Context, req sdk.Request) (sdk.Response, error) { return f(ctx, req.Payload) }
}

// Definition liefert das Metamodell der Entity.
func (e *Entity) Definition() metamodel.ObjectDefinition {
	d := metamodel.ObjectDefinition{Name: e.Object, Title: e.Title, Icon: e.Icon, TitleField: e.TitleField, Sections: e.Sections}
	for i := range e.Fields {
		f := &e.Fields[i]
		d.Fields = append(d.Fields, metamodel.FieldDefinition{
			Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required,
			Listable: f.Listable, Editable: !f.ReadOnly, Options: f.Options, Lookup: f.lookup(),
		})
	}
	d.Actions = []metamodel.ActionConfig{
		{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
		{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		{Name: "create", Kind: metamodel.KindCreate, Label: "Neu"},
		{Name: "update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
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
