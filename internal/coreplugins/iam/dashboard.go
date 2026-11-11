package iam

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Status eines Moduls (seit 0.12.7): Die Action, die ein Modul als Dashboard meldet
// (ModuleDefinition.Dashboard), darf jeder aufrufen, der für mindestens ein Object
// oder einen Service dieses Moduls eine Berechtigung hat – ohne eigenen Eintrag in den
// Rollen. So zeigen auch Personen und vorhandene Modulrollen den Status ihrer Module.
// Was gezählt wird, prüft das Modul selbst je Buchungskreis (sdk.CheckAccess).

// dashboardTTL: so lange gilt die Liste der Dashboards aus dem Catalog.
const dashboardTTL = time.Minute

type dashboardCache struct {
	mu    sync.Mutex
	at    time.Time
	byKey map[string][]string // "Object.Action" → Objects und Services des Moduls
}

// dashboardObjects: Objects und Services des Moduls, dessen Dashboard object.action ist (nil = keins).
func (p *Plugin) dashboardObjects(ctx context.Context, object, action string) []string {
	p.dash.mu.Lock()
	defer p.dash.mu.Unlock()
	if p.dash.byKey == nil || time.Since(p.dash.at) > dashboardTTL {
		if p.host == nil {
			return nil
		}
		resp, err := p.host.Handle(ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: "ListModules", Payload: map[string]any{}})
		var out struct {
			Modules []metamodel.ModuleDefinition `json:"modules"`
		}
		if err != nil || sdk.Decode(resp.Payload, &out) != nil {
			return nil
		}
		byKey := map[string][]string{}
		for _, m := range out.Modules {
			if m.Dashboard == nil {
				continue
			}
			names := slices.Clone(m.Services)
			for _, o := range m.Objects {
				names = append(names, o.Object)
			}
			byKey[m.Dashboard.Object+"."+m.Dashboard.Action] = names
		}
		p.dash.byKey, p.dash.at = byKey, time.Now()
	}
	return p.dash.byKey[object+"."+action]
}

// dashboardAllowed: object.action ist das Dashboard eines Moduls, in dem der Benutzer
// (mit den Erlaubnissen gs) etwas darf.
func (p *Plugin) dashboardAllowed(ctx context.Context, gs []grant, object, action string) bool {
	for _, name := range p.dashboardObjects(ctx, object, action) {
		if name != object && slices.ContainsFunc(gs, func(g grant) bool { return g.matchesObject(name) }) {
			return true
		}
	}
	return false
}
