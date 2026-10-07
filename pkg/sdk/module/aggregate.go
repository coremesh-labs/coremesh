package module

import (
	"context"
	"fmt"
	"slices"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// Aggregate (Master-Detail): Für jedes Object, dessen Metamodell Relationen
// enthält (SectionDefinition.Relation), registriert NewPlugin zwei Actions:
//
//	getAggregate  {"id": "…"}
//	  → {"record": {…}, "relations": {"<section>": [{…}, …], …}}
//
//	saveAggregate {"id"?: "…", "data"?: {…},
//	               "relations": {"<section>": {"create": [{…}], "update": [{"id": "…", "data": {…}}],
//	                                           "expire": [{"id": "…", "valid_to": "JJJJ-MM-TT"}], "deactivate": ["…"]}}}
//	  → wie getAggregate (Stand nach dem Speichern)
//
// saveAggregate speichert Master und Unter-Objects atomar in einer
// Transaktion der Moduldatenbank (Cascade Save). Ohne id wird der Master
// angelegt, und neue Unter-Objects erhalten seine id als Fremdschlüssel.
// Je Relation gilt die Reihenfolge expire → deactivate → update → create.
// Physisches Löschen gibt es nicht: Unter-Objects enden je nach Lifecycle
// über expire (Zeitscheibe) bzw. deactivate (Status-Flag).
//
// Jeder Teilschritt läuft über den Dispatcher (env.Services) mit den
// normalen Actions der Objects: Berechtigungen gelten je Object.Action, die
// Fachregeln der Actions greifen unverändert, und die Transaktion geht
// automatisch auf die Teilnehmer über.
const (
	ActionGetAggregate  = "getAggregate"
	ActionSaveAggregate = "saveAggregate"
)

type aggregate struct {
	m      *mounted
	master *ObjectRoutes
	rels   []relationRoute
}

type relationRoute struct {
	key   string
	rel   metamodel.Relation
	child *ObjectRoutes
}

// addAggregates registriert die Aggregat-Actions eines Moduls. Relationen
// müssen auf beschriebene Objects desselben Moduls zeigen.
func addAggregates(m *mounted) {
	r := m.router
	for _, o := range slices.Clone(r.objects) {
		if o.def == nil {
			continue
		}
		var rels []relationRoute
		for _, s := range o.def.Sections {
			if s.Relation == nil {
				continue
			}
			i := slices.IndexFunc(r.objects, func(c *ObjectRoutes) bool { return c.name == s.Relation.Object })
			if i < 0 || r.objects[i].def == nil {
				r.fail("%s: Relation %s verweist auf %s – kein beschriebenes Object dieses Moduls", o.name, s.Key, s.Relation.Object)
				continue
			}
			rels = append(rels, relationRoute{key: s.Key, rel: *s.Relation, child: r.objects[i]})
		}
		if len(rels) == 0 || o.handlers[ActionGetAggregate] != nil {
			continue // keine Relationen oder eigene Implementierung des Moduls
		}
		a := &aggregate{m: m, master: o, rels: rels}
		o.Handle(ActionGetAggregate, a.get)
		// Schreibgeschützte Master (ohne create und update, z. B. Buchungsbelege)
		// haben nur das Lesen.
		_, errC := actionOf(o, metamodel.KindCreate)
		_, errU := actionOf(o, metamodel.KindUpdate)
		if errC == nil || errU == nil {
			o.Handle(ActionSaveAggregate, a.save)
		}
	}
}

// actionOf liefert den Namen der Action vom Kind kind.
func actionOf(o *ObjectRoutes, kind metamodel.ActionKind) (string, error) {
	for _, a := range o.def.Actions {
		if a.Kind == kind {
			return a.Name, nil
		}
	}
	return "", fmt.Errorf("%w: %s bietet keine Action vom Kind %s", sdk.ErrUnimplemented, o.name, kind)
}

func (a *aggregate) call(ctx context.Context, o *ObjectRoutes, kind metamodel.ActionKind, payload any) (any, error) {
	action, err := actionOf(o, kind)
	if err != nil {
		return nil, err
	}
	resp, err := a.m.env.Services.Call(ctx, o.name, action, payload)
	return resp.Payload, err
}

func (a *aggregate) get(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if in.ID == "" {
		return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	out, err := a.load(ctx, in.ID)
	return sdk.Response{Payload: out}, err
}

func (a *aggregate) load(ctx context.Context, id string) (map[string]any, error) {
	rec, err := a.call(ctx, a.master, metamodel.KindItem, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	rels := map[string]any{}
	for _, rr := range a.rels {
		items, err := a.children(ctx, rr, id)
		if err != nil {
			return nil, fmt.Errorf("Relation %s: %w", rr.key, err)
		}
		rels[rr.key] = items
	}
	return map[string]any{"record": rec, "relations": rels}, nil
}

func (a *aggregate) children(ctx context.Context, rr relationRoute, id string) ([]any, error) {
	p, err := a.call(ctx, rr.child, metamodel.KindList, map[string]any{"query": map[string]any{rr.rel.ForeignKey: id}})
	if err != nil {
		return nil, err
	}
	switch v := p.(type) {
	case []any:
		return v, nil
	case map[string]any:
		items, _ := v["items"].([]any)
		return items, nil
	}
	return []any{}, nil
}

type relationOps struct {
	Create []map[string]any `json:"create"`
	Update []struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	} `json:"update"`
	// Ende eines Unter-Objects je nach Lifecycle – physisches Löschen gibt es nicht.
	Expire []struct {
		ID      string `json:"id"`
		ValidTo string `json:"valid_to"`
	} `json:"expire"` // timeslice: Gültigkeit zum gewählten Datum beenden
	Deactivate []string `json:"deactivate"` // status: inaktivieren
	Delete     []string `json:"delete"`     // nur zur Fehlermeldung
}

func (a *aggregate) save(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		ID        string                 `json:"id"`
		Data      map[string]any         `json:"data"`
		Relations map[string]relationOps `json:"relations"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	for key := range in.Relations {
		if !slices.ContainsFunc(a.rels, func(rr relationRoute) bool { return rr.key == key }) {
			return sdk.Response{}, fmt.Errorf("%w: %s hat keine Relation %q", sdk.ErrInvalidArgument, a.master.name, key)
		}
	}
	if in.ID == "" && in.Data == nil {
		return sdk.Response{}, fmt.Errorf("%w: id oder data (Neuanlage) fehlt", sdk.ErrInvalidArgument)
	}

	id := in.ID
	err := a.m.env.DB.InTx(ctx, nil, func(ctx context.Context) error {
		switch {
		case id == "":
			p, err := a.call(ctx, a.master, metamodel.KindCreate, map[string]any{"data": in.Data})
			if err != nil {
				return err
			}
			rec, _ := p.(map[string]any)
			if id, _ = rec["id"].(string); id == "" {
				return fmt.Errorf("%w: %s.create liefert keine id", sdk.ErrFailedPrecondition, a.master.name)
			}
		case in.Data != nil:
			if _, err := a.call(ctx, a.master, metamodel.KindUpdate, map[string]any{"id": id, "data": in.Data}); err != nil {
				return err
			}
		}
		for _, rr := range a.rels { // Reihenfolge der Abschnitte
			ops, ok := in.Relations[rr.key]
			if !ok {
				continue
			}
			if err := a.apply(ctx, rr, id, ops); err != nil {
				return fmt.Errorf("Relation %s: %w", rr.key, err)
			}
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	out, err := a.load(ctx, id)
	return sdk.Response{Payload: out}, err
}

func (a *aggregate) apply(ctx context.Context, rr relationRoute, masterID string, ops relationOps) error {
	fk := rr.rel.ForeignKey
	if len(ops.Delete) > 0 {
		return fmt.Errorf("%w: delete gibt es nicht – %s endet über %s", sdk.ErrInvalidArgument, rr.child.name, endHint(rr.child))
	}
	for i, e := range ops.Expire {
		if err := a.owned(ctx, rr, masterID, e.ID); err != nil {
			return fmt.Errorf("expire[%d]: %w", i, err)
		}
		if _, err := a.call(ctx, rr.child, metamodel.KindExpire, map[string]any{"id": e.ID, "valid_to": e.ValidTo}); err != nil {
			return fmt.Errorf("expire[%d]: %w", i, err)
		}
	}
	for i, childID := range ops.Deactivate {
		if err := a.owned(ctx, rr, masterID, childID); err != nil {
			return fmt.Errorf("deactivate[%d]: %w", i, err)
		}
		if _, err := a.call(ctx, rr.child, metamodel.KindDeactivate, map[string]any{"id": childID}); err != nil {
			return fmt.Errorf("deactivate[%d]: %w", i, err)
		}
	}
	for i, u := range ops.Update {
		if err := a.owned(ctx, rr, masterID, u.ID); err != nil {
			return fmt.Errorf("update[%d]: %w", i, err)
		}
		data := clone(u.Data)
		data[fk] = masterID // Zuordnung zum Master bleibt
		if _, err := a.call(ctx, rr.child, metamodel.KindUpdate, map[string]any{"id": u.ID, "data": data}); err != nil {
			return fmt.Errorf("update[%d]: %w", i, err)
		}
	}
	for i, c := range ops.Create {
		data := clone(c)
		data[fk] = masterID
		if _, err := a.call(ctx, rr.child, metamodel.KindCreate, map[string]any{"data": data}); err != nil {
			return fmt.Errorf("create[%d]: %w", i, err)
		}
	}
	return nil
}

// owned stellt sicher, dass ein Unter-Object zum Master gehört – über ein
// Aggregat lassen sich keine Datensätze anderer Master ändern.
func (a *aggregate) owned(ctx context.Context, rr relationRoute, masterID, childID string) error {
	p, err := a.call(ctx, rr.child, metamodel.KindItem, map[string]any{"id": childID})
	if err != nil {
		return err
	}
	rec, _ := p.(map[string]any)
	if fmt.Sprint(rec[rr.rel.ForeignKey]) != masterID {
		return fmt.Errorf("%w: %s %s gehört nicht zu %s %s", sdk.ErrInvalidArgument, rr.child.name, childID, a.master.name, masterID)
	}
	return nil
}

func clone(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// endHint beschreibt, wie ein Datensatz des Objects enden kann.
func endHint(o *ObjectRoutes) string {
	switch o.def.Lifecycle.Kind() {
	case metamodel.LifecycleTimeSlice:
		return `"expire" (Enddatum)`
	case metamodel.LifecycleStatus:
		return `"deactivate"`
	}
	return "nichts (immutable: weder löschen noch deaktivieren)"
}
