package catalog

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"github.com/camel/coremesh/pkg/sdk"
)

// ModuleInfo ist ein fachliches Modul mit Darstellung und Verfügbarkeit.
type ModuleInfo struct {
	Name        string             `json:"name"`
	Title       string             `json:"title"`
	Icon        string             `json:"icon,omitempty"`
	Description string             `json:"description,omitempty"`
	Plugin      string             `json:"plugin"`    // Plugin, das das Modul bedient
	Available   bool               `json:"available"` // mindestens ein Object aufrufbar
	Objects     []ModuleObjectInfo `json:"objects"`
}

// ModuleObjectInfo ist ein Object eines Moduls.
type ModuleObjectInfo struct {
	Object    string `json:"object"`
	Title     string `json:"title"`
	Icon      string `json:"icon,omitempty"`
	Section   string `json:"section,omitempty"`
	Available bool   `json:"available"`
}

// moduleOwner sucht das Plugin, das das Modul name in diesem Lauf
// registriert hat. Aufrufer hält p.mu.
func (p *Plugin) moduleOwner(name string) (string, bool) {
	for _, plugin := range slices.Sorted(maps.Keys(p.modules)) {
		for _, md := range p.modules[plugin].Modules {
			if md.Name == name {
				return plugin, true
			}
		}
	}
	return "", false
}

// listModules liefert alle Module, sortiert nach Titel. Verfügbarkeit kommt
// live aus den Routen. includeUnavailable ergänzt Module aus dem Cache
// (Plugin läuft gerade nicht) und Module ohne aufrufbares Object.
func (p *Plugin) listModules(includeUnavailable bool) []ModuleInfo {
	available := map[string]bool{}
	for _, e := range p.source() {
		available[e.Object] = true
	}

	p.mu.RLock()
	entries := map[string]*moduleEntry{}
	if includeUnavailable {
		maps.Copy(entries, p.cached)
	}
	maps.Copy(entries, p.modules) // registriert schlägt Cache
	p.mu.RUnlock()

	out := []ModuleInfo{}
	seen := map[string]bool{}
	for _, plugin := range slices.Sorted(maps.Keys(entries)) {
		e := entries[plugin]
		titles := map[string][2]string{}
		for _, d := range e.Objects {
			titles[d.Name] = [2]string{d.Title, d.Icon}
		}
		for _, md := range e.Modules {
			if seen[md.Name] {
				continue
			}
			seen[md.Name] = true
			mi := ModuleInfo{Name: md.Name, Title: md.Title, Icon: md.Icon, Description: md.Description,
				Plugin: plugin, Objects: []ModuleObjectInfo{}}
			for _, o := range md.Objects {
				t := titles[o.Object]
				oi := ModuleObjectInfo{Object: o.Object, Title: t[0], Icon: t[1], Section: o.Section, Available: available[o.Object]}
				mi.Available = mi.Available || oi.Available
				mi.Objects = append(mi.Objects, oi)
			}
			if mi.Available || includeUnavailable {
				out = append(out, mi)
			}
		}
	}
	slices.SortFunc(out, func(a, b ModuleInfo) int { return cmp.Or(cmp.Compare(a.Title, b.Title), cmp.Compare(a.Name, b.Name)) })
	return out
}

func (p *Plugin) getModule(name string) (sdk.Response, error) {
	if name == "" {
		return sdk.Response{}, fmt.Errorf("%w: module fehlt", sdk.ErrInvalidArgument)
	}
	for _, m := range p.listModules(true) {
		if m.Name == name {
			return sdk.Response{Payload: m}, nil
		}
	}
	return sdk.Response{}, fmt.Errorf("%w: Modul %q", sdk.ErrNotFound, name)
}

// moduleOf liefert das Modul, zu dem object gehört ("" = keines).
func (p *Plugin) moduleOf(object string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, src := range []map[string]*moduleEntry{p.modules, p.cached} {
		for _, plugin := range slices.Sorted(maps.Keys(src)) {
			for _, md := range src[plugin].Modules {
				for _, o := range md.Objects {
					if o.Object == object {
						return md.Name
					}
				}
			}
		}
	}
	return ""
}
