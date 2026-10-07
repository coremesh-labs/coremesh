package module

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// Info beschreibt das Plugin, das die Module beherbergt.
type Info struct {
	Name        string // Plugin-Name (Host-Config, DB-Präfix)
	Version     string
	Description string
}

// Plugin ist ein sdk.Plugin aus einem oder mehreren Modulen. Es übernimmt
// alles, was nicht Fachlogik ist: Manifest, Routing (Object, Action) →
// Handler, DBSchema.Init, Catalog.Describe, Dependency Injection und
// Lebenszyklus der Module.
type Plugin struct {
	info     Info
	mounted  []*mounted
	handlers map[[2]string]HandlerFunc
	err      error // Fehler aus RegisterRoutes – das Plugin startet dann nicht

	mu      sync.Mutex
	running []*mounted // initialisiert, in Startreihenfolge
}

type mounted struct {
	mod    Module
	desc   Descriptor
	router *Router
	env    Env // ab Configure (für Aggregate)
}

var (
	_ sdk.Plugin     = (*Plugin)(nil)
	_ sdk.Shutdowner = (*Plugin)(nil)
)

// NewPlugin ruft RegisterRoutes aller Module auf und prüft das Ergebnis.
// Fehler (doppelte Objects, ungültige Namen, …) liefert Err; der Host
// startet ein solches Plugin nicht (Manifest schlägt fehl).
func NewPlugin(info Info, modules ...Module) *Plugin {
	p := &Plugin{info: info, handlers: map[[2]string]HandlerFunc{}}
	var errs []error
	if info.Name == "" || info.Version == "" {
		errs = append(errs, errors.New("Info.Name und Info.Version sind Pflicht"))
	}
	if len(modules) == 0 {
		errs = append(errs, errors.New("mindestens ein Modul ist Pflicht"))
	}
	owners := map[string]string{}
	seen := map[string]bool{}
	for _, mod := range modules {
		d := mod.Descriptor()
		switch {
		case !metamodel.ValidModuleName(d.Name):
			errs = append(errs, fmt.Errorf("Modulname %q ist ungültig (Kleinbuchstaben, Ziffern, -; 2–40 Zeichen)", d.Name))
			continue
		case seen[d.Name]:
			errs = append(errs, fmt.Errorf("Modul %s doppelt", d.Name))
			continue
		case d.Title == "":
			errs = append(errs, fmt.Errorf("Modul %s: Title fehlt", d.Name))
		}
		seen[d.Name] = true
		r := &Router{module: d.Name, errs: &errs, owners: owners}
		mod.RegisterRoutes(r)
		mt := &mounted{mod: mod, desc: d, router: r}
		addAggregates(mt)
		r.check()
		for _, o := range r.objects {
			for _, a := range o.actions {
				p.handlers[[2]string{o.name, a}] = o.handlers[a]
			}
		}
		p.mounted = append(p.mounted, mt)
	}
	errs = append(errs, p.checkSchemas()...)
	errs = append(errs, p.checkTranslations()...)
	if len(errs) > 0 {
		p.err = fmt.Errorf("Plugin %s: %w", info.Name, errors.Join(errs...))
	}
	return p
}

// Err meldet Fehler aus RegisterRoutes (nil = in Ordnung).
func (p *Plugin) Err() error { return p.err }

// TablePrefix ist das Tabellen-Präfix eines Moduls, wenn ein Plugin mehrere
// Module mit Schema beherbergt: Plugin-Präfix plus Modulname, z. B.
// TablePrefix("crm", "sales-order") == "crm__sales_order_". Bei nur einem
// Modul mit Schema genügt das Plugin-Präfix (sdk.TablePrefix).
func TablePrefix(plugin, module string) string {
	return sdk.TablePrefix(plugin) + strings.ReplaceAll(module, "-", "_") + "_"
}

var (
	tableRe       = regexp.MustCompile(`(?m)^\s*table\s+"([^"]+)"`)
	schemaBlockRe = regexp.MustCompile(`(?m)^\s*schema\s+"`)
)

// checkSchemas trennt die Tabellen mehrerer Module eines Plugins: Jedes
// Modul nutzt nur Tabellen mit seinem Modul-Präfix. DBSchema schützt die
// Grenze zwischen Plugins; diese Prüfung die zwischen Modulen im selben Plugin.
func (p *Plugin) checkSchemas() []error {
	var withSchema []*mounted
	for _, m := range p.mounted {
		if _, ok := m.mod.(SchemaProvider); ok {
			withSchema = append(withSchema, m)
		}
	}
	var errs []error
	for _, m := range withSchema {
		s := m.mod.(SchemaProvider).Schema()
		if schemaBlockRe.MatchString(s.HCL) {
			errs = append(errs, fmt.Errorf("Modul %s: Schema enthält einen schema-Block – nur table-Blöcke liefern", m.desc.Name))
		}
		if len(withSchema) < 2 {
			continue
		}
		prefix := TablePrefix(p.info.Name, m.desc.Name)
		for _, t := range tableRe.FindAllStringSubmatch(s.HCL, -1) {
			if !strings.HasPrefix(t[1], prefix) {
				errs = append(errs, fmt.Errorf("Modul %s: Tabelle %s muss mit %q beginnen", m.desc.Name, t[1], prefix))
			}
		}
		for _, seed := range s.Seed {
			if !strings.HasPrefix(seed.Table, prefix) {
				errs = append(errs, fmt.Errorf("Modul %s: Seed für fremde Tabelle %s", m.desc.Name, seed.Table))
			}
		}
	}
	return errs
}

func (p *Plugin) hasSchema() bool {
	return slices.ContainsFunc(p.mounted, func(m *mounted) bool {
		_, ok := m.mod.(SchemaProvider)
		return ok
	})
}

// --- sdk.Plugin --------------------------------------------------------------

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	if p.err != nil {
		return sdk.Manifest{}, p.err
	}
	m := sdk.Manifest{Name: p.info.Name, Version: p.info.Version, Description: p.info.Description}
	for _, mt := range p.mounted {
		for _, o := range mt.router.objects {
			desc := mt.desc.Title
			if o.def != nil {
				desc = o.def.Title + " (" + mt.desc.Title + ")"
			}
			m.Capabilities = append(m.Capabilities, sdk.Capability{Object: o.name, Actions: slices.Clone(o.actions), Description: desc})
		}
	}
	if p.hasSchema() {
		m.Capabilities = append(m.Capabilities, sdk.Capability{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}})
	}
	m.Capabilities = append(m.Capabilities, sdk.Capability{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}})
	return m, nil
}

// settings ist der vom Plugin ausgewertete Teil der Plugin-Settings.
type settings struct {
	Database string                    `json:"database"`
	Modules  map[string]map[string]any `json:"modules"`
}

// Configure initialisiert alle Module in Registrierungsreihenfolge. Schlägt
// eines fehl, werden die bereits initialisierten wieder beendet.
func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	if p.err != nil {
		return p.err
	}
	var s settings
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	for name := range s.Modules {
		if !slices.ContainsFunc(p.mounted, func(m *mounted) bool { return m.desc.Name == name }) {
			return fmt.Errorf("%w: settings.modules.%s: unbekanntes Modul", sdk.ErrInvalidArgument, name)
		}
	}
	if s.Database == "" {
		s.Database = "main"
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.running) > 0 {
		return fmt.Errorf("%w: Plugin %s ist bereits konfiguriert", sdk.ErrFailedPrecondition, p.info.Name)
	}
	for _, m := range p.mounted {
		modCfg := s.Modules[m.desc.Name]
		dbName := s.Database
		if v, ok := modCfg["database"].(string); ok && v != "" {
			dbName = v
		}
		env := Env{
			Plugin: p.info.Name, Version: p.info.Version, Module: m.desc.Name,
			Log:      newLogger(cfg.Host, m.desc.Name),
			DB:       hostDB{name: dbName, host: cfg.Host},
			Services: hostServices{host: cfg.Host},
			config:   modCfg,
		}
		m.env = env
		if err := m.mod.Initialize(ctx, env); err != nil {
			p.shutdownLocked(ctx)
			return fmt.Errorf("Modul %s: Initialize: %w", m.desc.Name, err)
		}
		p.running = append(p.running, m)
	}
	return nil
}

// Handle leitet (Object, Action) an den Handler des zuständigen Moduls.
func (p *Plugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch {
	case req.Object == sdk.ObjectDBSchema && req.Action == sdk.ActionInit && p.hasSchema():
		return p.schema(req)
	case req.Object == sdk.ObjectCatalog && req.Action == sdk.ActionDescribe:
		return sdk.Response{Payload: p.describe()}, nil
	}
	if h := p.handlers[[2]string{req.Object, req.Action}]; h != nil {
		return h(ctx, req)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

// Shutdown beendet die Module in umgekehrter Startreihenfolge.
func (p *Plugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shutdownLocked(ctx)
}

func (p *Plugin) shutdownLocked(ctx context.Context) error {
	var errs []error
	for _, m := range slices.Backward(p.running) {
		if err := m.mod.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("Modul %s: %w", m.desc.Name, err))
		}
	}
	p.running = nil
	return errors.Join(errs...)
}

// schema fasst die Schemata aller Module zu einem Soll-Schema des Plugins zusammen.
func (p *Plugin) schema(req sdk.Request) (sdk.Response, error) {
	var in sdk.SchemaInitRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	var hcl strings.Builder
	hcl.WriteString("schema \"main\" {}\n")
	var seed []sdk.SchemaSeed
	for _, m := range p.mounted {
		if sp, ok := m.mod.(SchemaProvider); ok {
			s := sp.Schema()
			fmt.Fprintf(&hcl, "\n# --- Modul %s ---\n%s\n", m.desc.Name, s.HCL)
			seed = append(seed, s.Seed...)
		}
	}
	return sdk.Response{Payload: sdk.SchemaInitResponse{Module: in.Module, Schema: hcl.String(), Seed: seed}}, nil
}

// describe liefert Metamodelle und Module für den Catalog.
func (p *Plugin) describe() metamodel.DescribeResponse {
	var out metamodel.DescribeResponse
	for _, m := range p.mounted {
		for _, o := range m.router.objects {
			if o.def != nil {
				out.Objects = append(out.Objects, metamodel.WithKeys(m.desc.Name, *o.def))
			}
		}
		if md := m.router.definition(m.desc); len(md.Objects) > 0 {
			out.Modules = append(out.Modules, md)
		}
		if tr, ok := m.mod.(Translator); ok {
			for loc, dict := range tr.Translations() {
				if out.Translations == nil {
					out.Translations = metamodel.Translations{}
				}
				if out.Translations[loc] == nil {
					out.Translations[loc] = map[string]string{}
				}
				for k, v := range dict {
					out.Translations[loc][k] = v
				}
			}
		}
	}
	return out
}
