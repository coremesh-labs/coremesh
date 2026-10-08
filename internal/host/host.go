// Package host verbindet Konfiguration, Datenbank, Dispatcher und
// Plugin-Prozesse zum laufenden CoreMesh-Host.
package host

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/coremesh-labs/coremesh/internal/config"
	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/internal/dispatcher"
	"github.com/coremesh-labs/coremesh/internal/pluginmgr"
	"github.com/coremesh-labs/coremesh/internal/resolver"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// InternalDeps sind die Host-Interna, auf die Core-Plugins direkt zugreifen.
type InternalDeps struct {
	DB      *database.Manager
	Catalog func() []dispatcher.Entry // aufrufbare Routen, live
}

// InternalFactory erzeugt ein Core-Plugin, das im Host-Prozess läuft.
type InternalFactory func(deps InternalDeps) sdk.Plugin

type Host struct {
	cfg  *config.Config
	db   *database.Manager
	disp *dispatcher.Dispatcher
	mgr  *pluginmgr.Manager
	log  *slog.Logger

	closing atomic.Bool
	mu      sync.Mutex
	started []started // in Startreihenfolge
}

type started struct {
	name     string
	external bool
	core     sdk.Plugin // nur intern: für sdk.Shutdowner
}

func New(cfg *config.Config, db *database.Manager, log *slog.Logger) (*Host, error) {
	mgr, err := pluginmgr.New(cfg.Host.PluginStartTimeout, hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Level:  hclog.Info,
		Output: os.Stderr,
	}))
	if err != nil {
		return nil, err
	}
	return &Host{
		cfg:  cfg,
		db:   db,
		disp: dispatcher.New(cfg.Host.MaxCallDepth, db.EndRequest, log.With("component", "dispatcher")),
		mgr:  mgr,
		log:  log,
	}, nil
}

// Handle ist der Einstiegspunkt für Anfragen von außen.
func (h *Host) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	return h.disp.Handle(ctx, req)
}

// Read ist der Einstiegspunkt für Datenströme von außen.
func (h *Host) Read(ctx context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	return h.disp.Read(ctx, req, w)
}

// Routes liefert die aktuelle Routing-Tabelle.
func (h *Host) Routes() []dispatcher.Route { return h.disp.Routes() }

// StartInternal startet alle bekannten Core-Plugins (kind: internal). Fehlt
// ein Core-Plugin in der Konfiguration, läuft es mit Standardeinstellungen;
// mit enabled: false wird es übersprungen.
func (h *Host) StartInternal(ctx context.Context, factories map[string]InternalFactory) error {
	for _, name := range sortedKeys(factories) {
		pc, ok := h.cfg.Plugins[name]
		if !ok {
			pc = config.Plugin{Kind: config.KindInternal}
		}
		if pc.Kind != config.KindInternal {
			return fmt.Errorf("plugins.%s: Core-Plugin muss kind: internal haben", name)
		}
		if !pc.IsEnabled() {
			h.log.Warn("Core-Plugin deaktiviert", "plugin", name)
			continue
		}
		svc := h.services(name, pc)
		core := factories[name](InternalDeps{DB: h.db, Catalog: h.disp.Catalog})
		p := &inProcess{Plugin: core, host: svc}
		if err := h.activate(ctx, name, p, pc, svc); err != nil {
			return fmt.Errorf("Core-Plugin %s: %w", name, err)
		}
		// Ein Core-Plugin, das Berechtigungen kennt (iam), wird zum Authorizer
		// des Dispatchers: Ab jetzt wird jede Anfrage mit Benutzer geprüft.
		if a, ok := core.(dispatcher.Authorizer); ok {
			h.disp.SetAuthorizer(a)
			h.log.Info("Berechtigungsprüfung aktiv", "authorizer", name)
		}
		h.track(started{name: name, core: core})
	}
	for _, name := range h.cfg.Names(config.KindInternal) {
		if _, ok := factories[name]; !ok {
			return fmt.Errorf("plugins.%s: unbekanntes Core-Plugin", name)
		}
	}
	return nil
}

// StartExternal löst alle Domain-Plugins (kind: external) auf – lokal oder per
// Download –, startet ihre Prozesse und registriert sie im Dispatcher.
func (h *Host) StartExternal(ctx context.Context, res *resolver.Resolver) error {
	for _, name := range h.cfg.Names(config.KindExternal) {
		pc := h.cfg.Plugins[name]
		if err := h.startExternal(ctx, res, name, pc); err != nil {
			if pc.Optional {
				h.log.Error("optionales Plugin nicht gestartet", "plugin", name, "err", err)
				continue
			}
			return fmt.Errorf("Plugin %s: %w", name, err)
		}
	}
	return nil
}

func (h *Host) startExternal(ctx context.Context, res *resolver.Resolver, name string, pc config.Plugin) error {
	spec := pluginmgr.Spec{Name: name, SHA256: pc.SHA256, Args: pc.Args, Env: pc.Env}
	if h.mgr.IsReattach(name) {
		h.log.Info("Plugin im Debug-Modus wird angehängt", "plugin", name)
	} else {
		path, err := res.Resolve(ctx, name, pc.Version, pc.SHA256)
		if err != nil {
			return err
		}
		spec.Path = path
	}

	p, err := h.mgr.Start(spec)
	if err != nil {
		return fmt.Errorf("Start: %w", err)
	}
	if err := h.activate(ctx, name, p, pc, h.services(name, pc)); err != nil {
		h.mgr.Stop(name)
		return err
	}
	h.track(started{name: name, external: true})
	go h.watch(name)
	return nil
}

// activate prüft das Manifest, konfiguriert das Plugin und registriert seine Routen.
func (h *Host) activate(ctx context.Context, name string, p sdk.Plugin, pc config.Plugin, svc *pluginHost) error {
	ctx, end, err := h.disp.Begin(ctx)
	if err != nil {
		return err
	}
	defer end()

	m, err := p.Manifest(ctx)
	if err != nil {
		return fmt.Errorf("Manifest: %w", err)
	}
	if pc.Version != "" && m.Version != pc.Version {
		return fmt.Errorf("Version %s gemeldet, konfiguriert ist %s", m.Version, pc.Version)
	}
	// Schema-Isolation: Modul-Verbindungen bekommen den search_path auf das
	// eigene DB-Schema – vor Configure, damit schon dort alles passt.
	if db := h.schemaDatabase(); h.schemaIsolation() && pc.Kind == config.KindExternal {
		if _, ok := pc.Databases[db]; ok {
			if err := h.db.SetScope(db, name, sdk.SchemaName(name)); err != nil {
				return err
			}
		}
	}
	if err := p.Configure(sdk.WithHost(ctx, svc), sdk.Config{Settings: pc.Settings, Host: svc}); err != nil {
		return fmt.Errorf("Configure: %w", err)
	}
	// Schema-Migration vor der Registrierung: Das Modul erhält erst Anfragen,
	// wenn seine Tabellen in der passenden Version existieren.
	if dispatcher.HasLifecycle(m, sdk.ObjectDBSchema, sdk.ActionInit) {
		if err := h.initSchema(ctx, m, p, svc); err != nil {
			return fmt.Errorf("DBSchema.Init: %w", err)
		}
	}
	if err := h.disp.Register(name, m, p); err != nil {
		return err
	}
	// Metamodell nach der Registrierung: Der Catalog prüft gegen die Routen,
	// dass das Modul nur eigene Objects und Actions beschreibt.
	if dispatcher.HasLifecycle(m, sdk.ObjectCatalog, sdk.ActionDescribe) {
		if err := h.describe(ctx, m, p, svc); err != nil {
			_ = h.disp.Unregister(name)
			return fmt.Errorf("Catalog.Describe: %w", err)
		}
	}
	h.log.Info("Plugin registriert", "plugin", name, "version", m.Version, "kind", pc.Kind, "capabilities", capabilities(m))
	return nil
}

// watch erkennt abgestürzte Plugin-Prozesse und räumt auf.
func (h *Host) watch(name string) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		if h.closing.Load() {
			return
		}
		if h.mgr.Exited(name) {
			if h.closing.Load() {
				return
			}
			h.log.Error("Plugin-Prozess beendet – Routen entfernt, Transaktionen zurückgerollt", "plugin", name)
			_ = h.disp.Unregister(name)
			h.db.AbortOwner(name)
			h.mgr.Stop(name)
			return
		}
	}
}

// Shutdown beendet alle Plugins in umgekehrter Startreihenfolge: Routen
// entfernen, laufende Aufrufe abwarten (Drain), dann Prozess beenden.
func (h *Host) Shutdown(ctx context.Context) {
	h.closing.Store(true)
	h.mu.Lock()
	plugins := slices.Clone(h.started)
	h.started = nil
	h.mu.Unlock()

	for _, s := range slices.Backward(plugins) {
		if err := h.disp.Unregister(s.name)(ctx); err != nil {
			h.log.Warn("Drain unvollständig", "plugin", s.name, "err", err)
		}
		if s.external {
			h.mgr.Stop(s.name)
		} else if sd, ok := s.core.(sdk.Shutdowner); ok {
			if err := sd.Shutdown(ctx); err != nil {
				h.log.Warn("Shutdown fehlgeschlagen", "plugin", s.name, "err", err)
			}
		}
		h.log.Info("Plugin beendet", "plugin", s.name)
	}
}

func (h *Host) track(s started) {
	h.mu.Lock()
	h.started = append(h.started, s)
	h.mu.Unlock()
}

// inProcess bindet ein Core-Plugin im Host-Prozess ein. Es setzt – wie der
// gRPC-Adapter bei externen Plugins – den Host in jeden Aufruf-Kontext.
type inProcess struct {
	sdk.Plugin
	host sdk.Host
}

func (p *inProcess) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	return p.Plugin.Handle(sdk.WithHost(ctx, p.host), req)
}

// Read reicht Datenströme an Core-Plugins durch, die sdk.Reader implementieren.
func (p *inProcess) Read(ctx context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	r, ok := p.Plugin.(sdk.Reader)
	if !ok {
		return sdk.ReadEnd{}, fmt.Errorf("%w: %s.%s ist nicht als Datenstrom abrufbar", sdk.ErrUnimplemented, req.Object, req.Action)
	}
	sw := &sdk.StrictWriter{W: w}
	end, err := r.Read(sdk.WithHost(ctx, p.host), req, sw)
	if err != nil {
		return end, err
	}
	return sw.Finish(end)
}

func capabilities(m sdk.Manifest) []string {
	var out []string
	for _, c := range m.Capabilities {
		for _, a := range c.Actions {
			out = append(out, c.Object+"."+a)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
