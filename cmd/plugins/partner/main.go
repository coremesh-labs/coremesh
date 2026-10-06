// Command partner ist das Fachmodul Geschäftspartner nach dem Vorbild des
// SAP-Business-Partner-Modells: ein Partner (Person/Organisation) mit Rollen,
// zeitabhängigen Adressen, Kommunikation und Bankverbindungen sowie
// Finanzdaten je Buchungskreis für Debitor- und Kreditorrollen. Alle Typen
// und Rollen sind Stammdaten-Kataloge. Beschreibung: README.md.
package main

import (
	"context"
	"fmt"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/plugin"
)

const (
	name    = "partner"
	version = "0.1.0"
)

type partner struct {
	byObject map[string]*entity
	order    []*entity
}

func newPartner() *partner {
	p := &partner{byObject: map[string]*entity{}}
	for _, e := range entities() {
		p.byObject[e.Object] = e
		p.order = append(p.order, e)
	}
	return p
}

var crudActions = []string{"list", "get", "create", "update", "delete"}

func (p *partner) Manifest(context.Context) (sdk.Manifest, error) {
	m := sdk.Manifest{Name: name, Version: version, Description: "Geschäftspartner (SAP-BP-Modell)"}
	for _, e := range p.order {
		m.Capabilities = append(m.Capabilities, sdk.Capability{Object: e.Object, Actions: crudActions, Description: e.Title})
	}
	m.Capabilities = append(m.Capabilities,
		sdk.Capability{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
		sdk.Capability{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}},
	)
	return m, nil
}

func (p *partner) Configure(context.Context, sdk.Config) error { return nil }

func (p *partner) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch {
	case req.Object == sdk.ObjectDBSchema && req.Action == sdk.ActionInit:
		var in sdk.SchemaInitRequest
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: sdk.SchemaInitResponse{Module: in.Module, Schema: schemaHCL, Seed: seeds}}, nil
	case req.Object == sdk.ObjectCatalog && req.Action == sdk.ActionDescribe:
		var defs []metamodel.ObjectDefinition
		for _, e := range p.order {
			defs = append(defs, e.definition())
		}
		return sdk.Response{Payload: metamodel.DescribeResponse{Objects: defs}}, nil
	}

	e, ok := p.byObject[req.Object]
	if !ok {
		return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
	}
	switch req.Action {
	case "list":
		return e.list(ctx, req.Payload)
	case "get":
		return e.get(ctx, req.Payload)
	case "create":
		return e.create(ctx, req.Payload)
	case "update":
		return e.update(ctx, req.Payload)
	case "delete":
		return e.delete(ctx, req.Payload)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func main() {
	plugin.Main(newPartner())
}
