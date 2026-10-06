// Package catalog ist das Core-Plugin für die Registry aller Business-Objects:
// welche es gibt, welche Actions sie anbieten und wie sie aussehen
// (Metamodell, pkg/sdk/metamodel).
//
// Actions des Objects Catalog:
//
//   - Register {module, version, objects} – Host-Route: Der Host reicht die
//     Definitionen weiter, die ein Modul über die Lebenszyklus-Capability
//     Catalog.Describe liefert. Ein Modul darf nur Objects beschreiben, die es
//     selbst bedient, und nur Actions anbieten, die als seine Route existieren.
//   - ListObjects {include_unavailable?} – alle aktuell aufrufbaren Objects mit
//     Titel/Icon aus dem Metamodell.
//   - GetDefinition {object} – Metamodell-Definition eines Objects.
//   - ListActions {object} – alle aufrufbaren Actions eines Objects.
//   - ListModules {include_unavailable?} – alle fachlichen Module mit ihren
//     Objects (metamodel.ModuleDefinition), ergänzt um Titel und Verfügbarkeit.
//   - GetModule {module} – ein Modul.
//
// Module: Ein Plugin beschreibt neben Objects auch Module (Namensräume, die
// Objects bündeln). Ein Modul enthält nur Objects mit Metamodell aus
// demselben Plugin; Modulnamen sind systemweit eindeutig. Der WebServer
// registriert ausschließlich Module.
//
// Verfügbarkeit kommt live aus der Routing-Tabelle des Dispatchers: Stürzt ein
// Modul ab, sind seine Objects sofort nicht mehr verfügbar. Definitionen
// werden in settings.cache_dir zwischengespeichert. Module schicken sie bei
// jedem Start neu; der Cache überbrückt Module, die (noch) nicht laufen.
package catalog

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/dispatcher"
	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

const (
	Name    = "catalog"
	Version = "0.5.0"
	Object  = sdk.ObjectCatalog
)

// Source liefert die aktuell aufrufbaren Routen (dispatcher.Catalog).
type Source func() []dispatcher.Entry

type Plugin struct {
	source Source

	mu       sync.RWMutex
	cacheDir string                  // leer = kein Festplatten-Cache
	modules  map[string]*moduleEntry // in diesem Lauf registriert
	cached   map[string]*moduleEntry // letzter bekannter Stand (Cache-Datei)
}

type settings struct {
	CacheDir string `json:"cache_dir"`
}

func New(source Source) *Plugin {
	return &Plugin{source: source, modules: map[string]*moduleEntry{}, cached: map[string]*moduleEntry{}}
}

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name:        Name,
		Version:     Version,
		Description: "Registry der Business-Objects, ihrer Actions und Metamodelle",
		Capabilities: []sdk.Capability{
			{Object: Object, Actions: []string{"Register", "ListObjects", "GetDefinition", "ListActions", "ListModules", "GetModule", "Translations"},
				Description: "Verzeichnis der Module und Business-Objects"},
		},
	}, nil
}

// Configure lädt den Festplatten-Cache. Eine beschädigte Cache-Datei wird
// verworfen (sie wird beim nächsten Register neu geschrieben).
func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	var s settings
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheDir = s.CacheDir
	if s.CacheDir == "" {
		return nil
	}
	cached, err := loadCache(s.CacheDir)
	if err != nil {
		_ = sdk.HostFrom(ctx).Log(ctx, sdk.LogWarn, "Catalog-Cache verworfen", map[string]string{"err": err.Error()})
		cached = map[string]*moduleEntry{}
	}
	p.cached = cached
	return nil
}

func (p *Plugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch req.Action {
	case "Register":
		return p.register(ctx, req.Payload)
	case "ListObjects":
		var in struct {
			IncludeUnavailable bool `json:"include_unavailable"`
		}
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: map[string]any{"objects": p.listObjects(in.IncludeUnavailable)}}, nil
	case "GetDefinition":
		object, err := objectParam(req.Payload)
		if err != nil {
			return sdk.Response{}, err
		}
		return p.getDefinition(object)
	case "ListActions":
		object, err := objectParam(req.Payload)
		if err != nil {
			return sdk.Response{}, err
		}
		actions := p.listActions(object)
		if len(actions) == 0 {
			return sdk.Response{}, fmt.Errorf("%w: Object %q", sdk.ErrNotFound, object)
		}
		return sdk.Response{Payload: map[string]any{"object": object, "actions": actions}}, nil
	case "ListModules":
		var in struct {
			IncludeUnavailable bool `json:"include_unavailable"`
		}
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: map[string]any{"modules": p.listModules(in.IncludeUnavailable)}}, nil
	case "Translations":
		var in struct {
			Locale string `json:"locale"`
		}
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return p.translations(in.Locale)
	case "GetModule":
		var in struct {
			Module string `json:"module"`
		}
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return p.getModule(in.Module)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, Object, req.Action)
}

type registerInput struct {
	Module       string                       `json:"module"`
	Version      string                       `json:"version"`
	Objects      []metamodel.ObjectDefinition `json:"objects"`
	Modules      []metamodel.ModuleDefinition `json:"modules"`
	Translations metamodel.Translations       `json:"translations"`
}

func (p *Plugin) register(ctx context.Context, payload any) (sdk.Response, error) {
	var in registerInput
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if !config.ValidPluginName(in.Module) || in.Version == "" {
		return sdk.Response{}, fmt.Errorf("%w: module und version sind Pflicht", sdk.ErrInvalidArgument)
	}
	if err := checkOwnership(in.Module, in.Objects, in.Modules, in.Translations, p.source()); err != nil {
		return sdk.Response{}, err
	}
	entry, err := newEntry(in.Module, in.Version, in.Objects, in.Modules, in.Translations)
	if err != nil {
		return sdk.Response{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Ein Object hat genau eine Definition – auch wenn mehrere Module Actions beisteuern.
	for _, d := range in.Objects {
		if owner, ok := p.definitionOwner(d.Name); ok && owner != in.Module {
			return sdk.Response{}, fmt.Errorf("%w: Object %s ist bereits von Modul %s definiert", sdk.ErrAlreadyExists, d.Name, owner)
		}
	}
	// Modulnamen sind systemweit eindeutig (Namensraum in URLs).
	for _, md := range in.Modules {
		if owner, ok := p.moduleOwner(md.Name); ok && owner != in.Module {
			return sdk.Response{}, fmt.Errorf("%w: Modul %s ist bereits von Plugin %s registriert", sdk.ErrAlreadyExists, md.Name, owner)
		}
	}
	p.modules[in.Module] = entry

	changed := true
	if old, ok := p.cached[in.Module]; ok && old.Checksum == entry.Checksum && old.Version == entry.Version {
		changed = false
	}
	if changed && p.cacheDir != "" {
		p.cached[in.Module] = entry
		if err := writeCache(p.cacheDir, p.cached); err != nil {
			// Der Cache ist nur eine Hilfe – Registrierung trotzdem gültig.
			_ = sdk.HostFrom(ctx).Log(ctx, sdk.LogWarn, "Catalog-Cache nicht geschrieben", map[string]string{"err": err.Error()})
		}
	}

	names := make([]string, len(in.Objects))
	for i, d := range in.Objects {
		names[i] = d.Name
	}
	modules := make([]string, len(in.Modules))
	for i, md := range in.Modules {
		modules[i] = md.Name
	}
	return sdk.Response{Payload: map[string]any{
		"module": in.Module, "version": in.Version, "objects": names, "modules": modules, "changed": changed,
	}}, nil
}

// definitionOwner sucht das Modul, das die Definition von object geliefert
// hat (nur in diesem Lauf registrierte Module). Aufrufer hält p.mu.
func (p *Plugin) definitionOwner(object string) (string, bool) {
	for _, name := range slices.Sorted(maps.Keys(p.modules)) {
		for _, d := range p.modules[name].Objects {
			if d.Name == object {
				return name, true
			}
		}
	}
	return "", false
}

// definition sucht die Definition von object: zuerst registriert, dann Cache.
func (p *Plugin) definition(object string) (def metamodel.ObjectDefinition, e *moduleEntry, fromCache bool, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, src := range []struct {
		m     map[string]*moduleEntry
		cache bool
	}{{p.modules, false}, {p.cached, true}} {
		for _, name := range slices.Sorted(maps.Keys(src.m)) {
			for _, d := range src.m[name].Objects {
				if d.Name == object {
					return d, src.m[name], src.cache, true
				}
			}
		}
	}
	return metamodel.ObjectDefinition{}, nil, false, false
}

// ObjectInfo beschreibt ein Business-Object in der Übersicht.
type ObjectInfo struct {
	Object      string   `json:"object"`
	Title       string   `json:"title,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Description string   `json:"description,omitempty"`
	Module      string   `json:"module,omitempty"` // fachliches Modul (leer = keinem zugeordnet)
	Plugins     []string `json:"plugins"`
	Actions     int      `json:"actions"`
	Defined     bool     `json:"defined"`   // Metamodell vorhanden
	Available   bool     `json:"available"` // aktuell aufrufbar
}

// ActionInfo beschreibt eine aufrufbare Action eines Objects.
type ActionInfo struct {
	Action      string `json:"action"`
	Plugin      string `json:"plugin"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// listObjects fasst die Routen pro Object zusammen und ergänzt Titel/Icon
// aus dem Metamodell. includeUnavailable fügt Objects hinzu, die nur noch im
// Cache stehen (Modul läuft gerade nicht).
func (p *Plugin) listObjects(includeUnavailable bool) []ObjectInfo {
	out := []ObjectInfo{}
	for _, e := range p.source() { // nach Object sortiert
		if n := len(out); n == 0 || out[n-1].Object != e.Object {
			out = append(out, ObjectInfo{Object: e.Object, Plugins: []string{}, Available: true})
		}
		o := &out[len(out)-1]
		o.Actions++
		if !slices.Contains(o.Plugins, e.Plugin) {
			o.Plugins = append(o.Plugins, e.Plugin)
		}
		if o.Description == "" {
			o.Description = e.Description
		}
	}
	for i := range out {
		slices.Sort(out[i].Plugins)
		if d, _, _, ok := p.definition(out[i].Object); ok {
			out[i].Title, out[i].Icon, out[i].Defined = d.Title, d.Icon, true
		}
		out[i].Module = p.moduleOf(out[i].Object)
	}

	if includeUnavailable {
		p.mu.RLock()
		for _, name := range slices.Sorted(maps.Keys(p.cached)) {
			for _, d := range p.cached[name].Objects {
				if !slices.ContainsFunc(out, func(o ObjectInfo) bool { return o.Object == d.Name }) {
					out = append(out, ObjectInfo{Object: d.Name, Title: d.Title, Icon: d.Icon, Module: p.moduleOf(d.Name),
						Plugins: []string{name}, Defined: true, Available: false})
				}
			}
		}
		p.mu.RUnlock()
		slices.SortFunc(out, func(a, b ObjectInfo) int { return cmp.Compare(a.Object, b.Object) })
	}
	return out
}

func (p *Plugin) getDefinition(object string) (sdk.Response, error) {
	d, e, fromCache, ok := p.definition(object)
	if !ok {
		return sdk.Response{}, fmt.Errorf("%w: keine Definition für Object %q", sdk.ErrNotFound, object)
	}
	available := len(p.listActions(object)) > 0
	return sdk.Response{Payload: map[string]any{
		"definition": d,
		"module":     e.Module,
		"version":    e.Version,
		"source":     map[bool]string{false: "registered", true: "cache"}[fromCache],
		"available":  available,
	}}, nil
}

func (p *Plugin) listActions(object string) []ActionInfo {
	var out []ActionInfo
	for _, e := range p.source() {
		if e.Object == object {
			out = append(out, ActionInfo{Action: e.Action, Plugin: e.Plugin, Version: e.Version, Description: e.Description})
		}
	}
	return out
}

func objectParam(payload any) (string, error) {
	var in struct {
		Object string `json:"object"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return "", err
	}
	if in.Object == "" {
		return "", fmt.Errorf("%w: object fehlt", sdk.ErrInvalidArgument)
	}
	return in.Object, nil
}
