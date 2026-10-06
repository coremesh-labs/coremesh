package host

import (
	"context"
	"errors"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// describe holt die Metamodell-Definitionen eines Moduls (Catalog.Describe)
// und registriert sie mit dem geprüften Modulnamen beim Catalog-Plugin
// (Catalog.Register, Host-Route). Läuft bei jedem Start; der Catalog
// vergleicht mit seinem Cache.
func (h *Host) describe(ctx context.Context, m sdk.Manifest, p sdk.Plugin, svc *pluginHost) error {
	resp, err := p.Handle(sdk.WithHost(ctx, svc), sdk.Request{
		Object:  sdk.ObjectCatalog,
		Action:  sdk.ActionDescribe,
		Payload: metamodel.DescribeRequest{Module: m.Name, Version: m.Version},
	})
	if err != nil {
		return err
	}
	var desc metamodel.DescribeResponse
	if err := sdk.Decode(resp.Payload, &desc); err != nil {
		return err
	}

	resp, err = h.disp.Call(ctx, sdk.Request{
		Object: sdk.ObjectCatalog,
		Action: "Register",
		Payload: map[string]any{
			"module": m.Name, "version": m.Version, "objects": desc.Objects, "modules": desc.Modules,
		},
	})
	if errors.Is(err, sdk.ErrUnimplemented) {
		h.log.Warn("Catalog-Plugin nicht aktiv – Metamodell nicht registriert", "plugin", m.Name)
		return nil
	}
	if err != nil {
		return err
	}
	h.log.Info("Metamodell registriert", "plugin", m.Name, "ergebnis", resp.Payload)
	return nil
}
