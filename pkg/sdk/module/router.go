package module

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// HandlerFunc bedient eine Action eines Objects.
type HandlerFunc func(ctx context.Context, req sdk.Request) (sdk.Response, error)

// Router nimmt die Routen eines Moduls auf. Jedes Modul erhält einen eigenen
// Router; ein Object gehört genau einem Modul.
//
//	func (m *Module) RegisterRoutes(r *module.Router) {
//		r.Object("BusinessPartner").
//			Describe(bpDefinition).
//			Handle("list", m.list).
//			Handle("get", m.get)
//		r.Object("PartnerRoleType").Section("Kataloge").Describe(…).Handle(…)
//	}
type Router struct {
	module   string
	objects  []*ObjectRoutes
	commands []metamodel.CommandDefinition
	errs    *[]error
	owners  map[string]string // Object -> Modul (über alle Module eines Plugins)
}

// ObjectRoutes sind die Actions eines Objects.
type ObjectRoutes struct {
	r        *Router
	name     string
	section  string
	def      *metamodel.ObjectDefinition
	actions  []string
	handlers map[string]HandlerFunc
}

var (
	objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	actionRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
)

// reserved sind Objects der Lebenszyklus- bzw. Core-Plugins.
var reserved = map[string]bool{sdk.ObjectDBSchema: true, sdk.ObjectCatalog: true}

func (r *Router) fail(format string, args ...any) {
	*r.errs = append(*r.errs, fmt.Errorf("Modul %s: "+format, append([]any{r.module}, args...)...))
}

// Object meldet ein Object des Moduls an.
func (r *Router) Object(name string) *ObjectRoutes {
	o := &ObjectRoutes{r: r, name: name, handlers: map[string]HandlerFunc{}}
	switch {
	case !objectRe.MatchString(name):
		r.fail("Object %q muss %s entsprechen", name, objectRe)
	case reserved[name]:
		r.fail("Object %s ist reserviert", name)
	case r.owners[name] != "":
		r.fail("Object %s gehört bereits zu Modul %s", name, r.owners[name])
	default:
		r.owners[name] = r.module
		r.objects = append(r.objects, o)
	}
	return o
}

// Section ordnet das Object in der Navigation des Moduls einer Gruppe zu.
func (o *ObjectRoutes) Section(title string) *ObjectRoutes {
	o.section = title
	return o
}

// Describe hinterlegt das Metamodell (Oberfläche). Objects ohne Metamodell
// sind nur per API erreichbar und erscheinen nicht in der Navigation.
func (o *ObjectRoutes) Describe(def metamodel.ObjectDefinition) *ObjectRoutes {
	if def.Name == "" {
		def.Name = o.name
	}
	if def.Name != o.name {
		o.r.fail("Metamodell %s passt nicht zu Object %s", def.Name, o.name)
		return o
	}
	o.def = &def
	return o
}

// Handle registriert eine Action.
func (o *ObjectRoutes) Handle(action string, h HandlerFunc) *ObjectRoutes {
	switch {
	case !actionRe.MatchString(action):
		o.r.fail("%s: Action %q muss %s entsprechen", o.name, action, actionRe)
	case h == nil:
		o.r.fail("%s.%s: Handler ist nil", o.name, action)
	case o.handlers[action] != nil:
		o.r.fail("%s.%s doppelt registriert", o.name, action)
	default:
		o.handlers[action] = h
		o.actions = append(o.actions, action)
	}
	return o
}

// Command meldet einen Konsolenbefehl an: `console <modul>:<name> --param=wert`
// ruft object.action des Moduls mit den Parametern auf.
//
//	r.Command(metamodel.CommandDefinition{Name: "load-coa", Object: "ChartOfAccounts", Action: "load",
//		Params: []metamodel.CommandParam{{Name: "chart", Required: true}, {Name: "file", File: true}}})
func (r *Router) Command(c metamodel.CommandDefinition) {
	r.commands = append(r.commands, c)
}

// check prüft das Ergebnis von RegisterRoutes.
func (r *Router) check() {
	if len(r.objects) == 0 {
		r.fail("registriert keine Objects")
	}
	for _, o := range r.objects {
		if len(o.actions) == 0 {
			r.fail("Object %s hat keine Actions", o.name)
		}
	}
	for _, c := range r.commands {
		i := slices.IndexFunc(r.objects, func(o *ObjectRoutes) bool { return o.name == c.Object })
		if i < 0 || r.objects[i].handlers[c.Action] == nil {
			r.fail("Befehl %s: %s.%s ist keine Route des Moduls", c.Name, c.Object, c.Action)
		}
	}
}

// definition liefert die Modul-Definition für den Catalog: alle Objects mit
// Metamodell, in der Reihenfolge der Registrierung.
func (r *Router) definition(d Descriptor) metamodel.ModuleDefinition {
	md := metamodel.ModuleDefinition{Name: d.Name, Title: d.Title, Icon: d.Icon, Description: d.Description}
	for _, o := range r.objects {
		if o.def != nil {
			md.Objects = append(md.Objects, metamodel.ModuleObject{Object: o.name, Section: o.section})
		} else {
			md.Services = append(md.Services, o.name) // ohne Metamodell: nur JSON-API
		}
	}
	md.Commands = r.commands
	return metamodel.ModuleKeys(md)
}
