package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/coremesh-labs/coremesh/pkg/consoleapi/console/v1"
)

// Verben der Konsole (neben --object/--action und <modul>:<befehl>):
//
//	console handle  <Object>.<Action> [--name=wert …]       Action aufrufen
//	console read    <Object>.<Action> [--name=wert …]       Ergebnis als Datenstrom (csv, jsonl)
//	console list    [<ObjectMuster>]                        Objects
//	console list    <Object>.<ActionMuster>                 Actions eines Objects
//	console list    hooks [<Muster>]                        Hooks (synchrone Erweiterungspunkte)
//	console list    events [<ObjectMuster>]                 Event-Abonnements
//	console list    commands [<Muster>]                     Konsolenbefehle der Module
//	console details <Object>                                Felder, Actions, Filter
//	console details <Object>.<Action>                       Route und Formularfelder einer Action
//	console details hook <Name>                             Hook mit Phasen, Daten und Abos
//	console details event <Object>[.<Action>]               Abonnements zu Object/Action
//
// Muster: * und ? wie in der Shell (Groß-/Kleinschreibung egal); ohne
// Platzhalter genügt ein Teil des Namens. Parameter von handle/read werden
// nach dem Feldtyp des Objects umgewandelt: Textfelder bleiben Text
// (--company_code=2000), Zahl und Schalter werden gelesen, data/query als JSON.
// --format json gibt list/details als JSON aus.

var verbs = []string{"handle", "read", "list", "details"}

// verbFlags: CLI-Optionen, die im Verb-Modus einen Wert haben.
var verbFlags = map[string]bool{"addr": true, "tls-ca": true, "user": true, "out": true, "timeout": true, "format": true, "target-dir": true}

// isVerb: erstes Argument ist ein Verb (Groß-/Kleinschreibung egal).
func isVerb(arg string) bool { return slices.Contains(verbs, strings.ToLower(arg)) }

// verbArgs trennt die Argumente hinter dem Verb: Positionsargumente (Ziel,
// Muster), CLI-Optionen (für flag) und Parameter der Action (in p, als Text).
func verbArgs(args []string, p params) (pos, flags []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if verbFlags[name] || name == "param" {
			if !hasValue && i+1 < len(args) {
				i++
				value = args[i]
			}
			if name == "param" {
				if err := p.Set(value); err == nil {
					continue
				}
			}
			flags = append(flags, "--"+name+"="+value)
			continue
		}
		if !hasValue {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				value = args[i]
			} else {
				value = "true"
			}
		}
		p[name] = value
	}
	return pos, flags
}

func runVerb(ctx context.Context, c consolev1.ConsoleServiceClient, o options) error {
	target := ""
	if len(o.positional) > 0 {
		target = o.positional[0]
	}
	switch o.verb {
	case "handle", "read":
		obj, act, ok := strings.Cut(target, ".")
		if !ok || obj == "" || act == "" {
			return fmt.Errorf("%s <Object>.<Action> erwartet, nicht %q", o.verb, target)
		}
		def, _ := definition(ctx, c, obj)
		if err := convertParams(fieldParams(def), o.params); err != nil {
			return err
		}
		if err := readFileFields(def, o.params); err != nil {
			return err
		}
		o.object, o.action = obj, act
		if o.verb == "read" {
			return readStream(ctx, c, o)
		}
		o.verb, o.format = "", ""
		return call(ctx, c, o)
	case "list":
		return runList(ctx, c, o)
	case "details":
		return runDetails(ctx, c, o)
	}
	return fmt.Errorf("unbekanntes Verb %q", o.verb)
}

// --- list ----------------------------------------------------------------------------

func runList(ctx context.Context, c consolev1.ConsoleServiceClient, o options) error {
	arg := ""
	if len(o.positional) > 0 {
		arg = o.positional[0]
	}
	pattern := ""
	if len(o.positional) > 1 {
		pattern = o.positional[1]
	}
	switch strings.ToLower(arg) {
	case "hooks":
		return listHooks(ctx, c, o, pattern)
	case "events":
		return listEvents(ctx, c, o, pattern, "")
	case "commands":
		return listCommands(ctx, c, o, pattern)
	}
	if obj, act, ok := strings.Cut(arg, "."); ok {
		return listActions(ctx, c, o, obj, act)
	}
	return listObjects(ctx, c, o, arg)
}

func listObjects(ctx context.Context, c consolev1.ConsoleServiceClient, o options, pattern string) error {
	var out struct {
		Objects []struct {
			Object      string   `json:"object"`
			Description string   `json:"description"`
			Plugins     []string `json:"plugins"`
			Actions     int      `json:"actions"`
			Available   bool     `json:"available"`
		} `json:"objects"`
	}
	if err := execute(ctx, c, "Catalog", "ListObjects", nil, &out); err != nil {
		return err
	}
	var rows [][]string
	var items []any
	for _, ob := range out.Objects {
		if !matches(pattern, ob.Object) {
			continue
		}
		rows = append(rows, []string{ob.Object, strings.Join(ob.Plugins, ","), fmt.Sprint(ob.Actions), ob.Description})
		items = append(items, ob)
	}
	return printTable(o, []string{"OBJECT", "PLUGIN", "ACTIONS", "BESCHREIBUNG"}, rows, items)
}

func listActions(ctx context.Context, c consolev1.ConsoleServiceClient, o options, object, pattern string) error {
	acts, err := actions(ctx, c, object)
	if err != nil {
		return err
	}
	def, _ := definition(ctx, c, object)
	var rows [][]string
	var items []any
	for _, a := range acts {
		if !matches(pattern, a.Action) {
			continue
		}
		label := ""
		if cfg := actionConfig(def, a.Action); cfg != nil {
			label = cfg.Label
		}
		rows = append(rows, []string{object + "." + a.Action, a.Plugin, a.Version, label})
		items = append(items, map[string]any{"object": object, "action": a.Action, "plugin": a.Plugin, "version": a.Version, "label": label})
	}
	return printTable(o, []string{"ACTION", "PLUGIN", "VERSION", "BEZEICHNUNG"}, rows, items)
}

type hookInfo struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Owner         string `json:"owner"`
	Phases        string `json:"phases"`
	DataDoc       string `json:"data_doc"`
	OnFailure     string `json:"on_failure"`
	Subscriptions int    `json:"subscriptions"`
}

func listHooks(ctx context.Context, c consolev1.ConsoleServiceClient, o options, pattern string) error {
	var out struct {
		Items []hookInfo `json:"items"`
	}
	if err := execute(ctx, c, "Hook", "list", nil, &out); err != nil {
		return err
	}
	var rows [][]string
	var items []any
	for _, h := range out.Items {
		if !matches(pattern, h.Name) {
			continue
		}
		rows = append(rows, []string{h.Name, h.Owner, h.Phases, fmt.Sprint(h.Subscriptions), h.Description})
		items = append(items, h)
	}
	return printTable(o, []string{"HOOK", "BESITZER", "PHASEN", "ABOS", "BESCHREIBUNG"}, rows, items)
}

type eventSub struct {
	Object      string `json:"object"`
	Action      string `json:"action"`
	CompanyCode string `json:"company_code"`
	Callback    string `json:"callback"`
}

// listEvents: Abonnements, deren Object zum Muster passt (oder "*" abonniert);
// action schränkt zusätzlich auf eine Action ein (details event).
func listEvents(ctx context.Context, c consolev1.ConsoleServiceClient, o options, pattern, action string) error {
	var out struct {
		Subscriptions []eventSub `json:"subscriptions"`
	}
	if err := execute(ctx, c, "SystemEvent", "List", nil, &out); err != nil {
		return err
	}
	slices.SortFunc(out.Subscriptions, func(a, b eventSub) int {
		return strings.Compare(a.Object+"."+a.Action+"|"+a.Callback, b.Object+"."+b.Action+"|"+b.Callback)
	})
	var rows [][]string
	var items []any
	for _, s := range out.Subscriptions {
		if s.Object != "*" && !matches(pattern, s.Object) {
			continue
		}
		if action != "" && s.Action != "*" && !strings.EqualFold(s.Action, action) {
			continue
		}
		rows = append(rows, []string{s.Object + "." + s.Action, s.CompanyCode, s.Callback + ".onEvent"})
		items = append(items, s)
	}
	return printTable(o, []string{"EVENT", "BUCHUNGSKREIS", "EMPFÄNGER"}, rows, items)
}

func listCommands(ctx context.Context, c consolev1.ConsoleServiceClient, o options, pattern string) error {
	var out struct {
		Modules []struct {
			Name     string       `json:"name"`
			Commands []commandDef `json:"commands"`
		} `json:"modules"`
	}
	if err := execute(ctx, c, "Catalog", "ListModules", nil, &out); err != nil {
		return err
	}
	var rows [][]string
	var items []any
	for _, m := range out.Modules {
		for _, cmd := range m.Commands {
			name := m.Name + ":" + cmd.Name
			if !matches(pattern, name) {
				continue
			}
			var ps []string
			for _, p := range cmd.Params {
				s := "--" + p.Name
				if !p.Required {
					s = "[" + s + "]"
				}
				ps = append(ps, s)
			}
			rows = append(rows, []string{name, cmd.Object + "." + cmd.Action, strings.Join(ps, " "), cmd.Description})
			items = append(items, map[string]any{"command": name, "object": cmd.Object, "action": cmd.Action, "params": cmd.Params, "description": cmd.Description})
		}
	}
	return printTable(o, []string{"BEFEHL", "ACTION", "PARAMETER", "BESCHREIBUNG"}, rows, items)
}

// --- details -------------------------------------------------------------------------

func runDetails(ctx context.Context, c consolev1.ConsoleServiceClient, o options) error {
	if len(o.positional) == 0 {
		return errors.New("details <Object>, <Object>.<Action>, hook <Name> oder event <Object>[.<Action>] erwartet")
	}
	if len(o.positional) > 1 {
		switch strings.ToLower(o.positional[0]) {
		case "hook", "hooks":
			return hookDetails(ctx, c, o, o.positional[1])
		case "event", "events":
			obj, act, _ := strings.Cut(o.positional[1], ".")
			return listEvents(ctx, c, o, obj, act)
		}
	}
	obj, act, ok := strings.Cut(o.positional[0], ".")
	if ok {
		return actionDetails(ctx, c, o, obj, act)
	}
	return objectDetails(ctx, c, o, obj)
}

type objectDef struct {
	Name    string     `json:"name"`
	Title   string     `json:"title"`
	Fields  []fieldDef `json:"fields"`
	Actions []struct {
		Name   string   `json:"name"`
		Kind   string   `json:"kind"`
		Label  string   `json:"label"`
		Fields []string `json:"fields"`
		Record bool     `json:"record"`
	} `json:"actions"`
	Filters []string `json:"filters"`
	Search  bool     `json:"search"`
}

type fieldDef struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Editable bool   `json:"editable"`
	Lookup   *struct {
		Object string `json:"object"`
	} `json:"lookup"`
}

type defResponse struct {
	Module     string    `json:"module"`
	Version    string    `json:"version"`
	Available  bool      `json:"available"`
	Definition objectDef `json:"definition"`
}

func objectDetails(ctx context.Context, c consolev1.ConsoleServiceClient, o options, object string) error {
	acts, err := actions(ctx, c, object)
	if err != nil {
		return err
	}
	def, _ := definition(ctx, c, object)
	if o.format == "json" {
		return printJSON(o, map[string]any{"object": object, "definition": def, "actions": acts})
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	defer func() { w.Flush(); _ = output(o.out, []byte(b.String()), "Details") }()
	if def != nil {
		fmt.Fprintf(w, "%s – %s (Modul %s %s)\n", object, def.Definition.Title, def.Module, def.Version)
	} else {
		fmt.Fprintf(w, "%s (ohne Metamodell)\n", object)
	}
	if def != nil && len(def.Definition.Fields) > 0 {
		fmt.Fprintln(w, "\nFELD\tTYP\tPFLICHT\tVERWEIS\tBEZEICHNUNG")
		for _, f := range def.Definition.Fields {
			ref := ""
			if f.Lookup != nil {
				ref = f.Lookup.Object
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", f.Key, f.Type, yes(f.Required), ref, f.Label)
		}
	}
	fmt.Fprintln(w, "\nACTION\tPLUGIN\tART\tBEZEICHNUNG")
	for _, a := range acts {
		kind, label := "", ""
		if cfg := actionConfig(def, a.Action); cfg != nil {
			kind, label = cfg.Kind, cfg.Label
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.Action, a.Plugin+" "+a.Version, kind, label)
	}
	if def != nil && len(def.Definition.Filters) > 0 {
		fmt.Fprintf(w, "\nFilter (list): %s", strings.Join(def.Definition.Filters, ", "))
		if def.Definition.Search {
			fmt.Fprint(w, "; Suche: q")
		}
		fmt.Fprintln(w)
	}
	return nil
}

func actionDetails(ctx context.Context, c consolev1.ConsoleServiceClient, o options, object, action string) error {
	acts, err := actions(ctx, c, object)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(acts, func(a actionInfo) bool { return a.Action == action })
	if i < 0 {
		return fmt.Errorf("%s.%s gibt es nicht (console list %s.*)", object, action, object)
	}
	def, _ := definition(ctx, c, object)
	cfg := actionConfig(def, action)
	if o.format == "json" {
		return printJSON(o, map[string]any{"object": object, "action": acts[i], "config": cfg})
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	defer func() { w.Flush(); _ = output(o.out, []byte(b.String()), "Details") }()
	fmt.Fprintf(w, "%s.%s → Plugin %s %s\n", object, action, acts[i].Plugin, acts[i].Version)
	if cfg == nil {
		fmt.Fprintln(w, "keine Oberflächen-Action im Metamodell (nur API)")
		return nil
	}
	fmt.Fprintf(w, "Art: %s, Bezeichnung: %s", cfg.Kind, cfg.Label)
	if cfg.Record {
		fmt.Fprint(w, ", für einen Datensatz (--id)")
	}
	fmt.Fprintln(w)
	fields := cfg.Fields
	switch cfg.Kind {
	case "create", "update":
		fields = nil
		for _, f := range def.Definition.Fields {
			if f.Editable {
				fields = append(fields, f.Key)
			}
		}
		fmt.Fprintln(w, "Parameter: --data='{…}' mit den Feldern unten"+map[bool]string{true: ", --id", false: ""}[cfg.Kind == "update"])
	case "list":
		fmt.Fprintln(w, "Parameter: --query='{…}' mit Filtern: "+strings.Join(def.Definition.Filters, ", "))
		return nil
	case "item":
		fmt.Fprintln(w, "Parameter: --id")
		return nil
	}
	if len(fields) > 0 {
		fmt.Fprintln(w, "\nFELD\tTYP\tPFLICHT\tBEZEICHNUNG")
		for _, k := range fields {
			for _, f := range def.Definition.Fields {
				if f.Key == k {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Key, f.Type, yes(f.Required), f.Label)
				}
			}
		}
	}
	return nil
}

func hookDetails(ctx context.Context, c consolev1.ConsoleServiceClient, o options, name string) error {
	var h hookInfo
	if err := execute(ctx, c, "Hook", "get", map[string]any{"id": name}, &h); err != nil {
		return err
	}
	var subs struct {
		Items []struct {
			Hook        string `json:"hook"`
			Phase       string `json:"phase"`
			Callback    string `json:"callback"`
			Priority    int    `json:"priority"`
			Description string `json:"description"`
			LockedBy    string `json:"locked_by"`
		} `json:"items"`
	}
	if err := execute(ctx, c, "HookSubscription", "list", nil, &subs); err != nil {
		return err
	}
	var own []any
	for _, s := range subs.Items {
		if s.Hook == name {
			own = append(own, s)
		}
	}
	if o.format == "json" {
		return printJSON(o, map[string]any{"hook": h, "subscriptions": own})
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	defer func() { w.Flush(); _ = output(o.out, []byte(b.String()), "Details") }()
	fmt.Fprintf(w, "%s (Besitzer %s): %s\nPhasen: %s, bei Fehler: %s\nDaten: %s\n", h.Name, h.Owner, h.Description, h.Phases, h.OnFailure, h.DataDoc)
	if len(own) == 0 {
		fmt.Fprintln(w, "\nkeine Abos")
		return nil
	}
	fmt.Fprintln(w, "\nPHASE\tPRIO\tEMPFÄNGER\tGESPERRT\tBESCHREIBUNG")
	for _, s := range subs.Items {
		if s.Hook == name {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", s.Phase, s.Priority, s.Callback+".onHook", s.LockedBy, s.Description)
		}
	}
	return nil
}

// --- Hilfen --------------------------------------------------------------------------

type actionInfo struct {
	Action  string `json:"action"`
	Plugin  string `json:"plugin"`
	Version string `json:"version"`
}

func actions(ctx context.Context, c consolev1.ConsoleServiceClient, object string) ([]actionInfo, error) {
	var out struct {
		Actions []actionInfo `json:"actions"`
	}
	if err := execute(ctx, c, "Catalog", "ListActions", map[string]any{"object": object}, &out); err != nil {
		return nil, err
	}
	if len(out.Actions) == 0 {
		return nil, fmt.Errorf("Object %s gibt es nicht (console list %s)", object, object)
	}
	return out.Actions, nil
}

// definition: Metamodell des Objects; nil, wenn es keins hat.
func definition(ctx context.Context, c consolev1.ConsoleServiceClient, object string) (*defResponse, error) {
	var d defResponse
	if err := execute(ctx, c, "Catalog", "GetDefinition", map[string]any{"object": object}, &d); err != nil {
		return nil, err
	}
	if d.Definition.Name == "" {
		return nil, nil
	}
	return &d, nil
}

type actionCfg struct {
	Kind, Label string
	Fields      []string
	Record      bool
}

func actionConfig(def *defResponse, name string) *actionCfg {
	if def == nil {
		return nil
	}
	for _, a := range def.Definition.Actions {
		if a.Name == name {
			return &actionCfg{Kind: a.Kind, Label: a.Label, Fields: a.Fields, Record: a.Record}
		}
	}
	return nil
}

// fieldParams: Parametertypen aus den Feldern des Objects (für convertParams).
// id ist immer Text, data und query sind JSON.
func fieldParams(def *defResponse) *commandDef {
	cd := &commandDef{Params: []commandParam{{Name: "id"}, {Name: "data", Type: "json"}, {Name: "query", Type: "json"}}}
	if def == nil {
		return cd
	}
	for _, f := range def.Definition.Fields {
		typ := ""
		switch f.Type {
		case "number":
			typ = "number"
		case "boolean":
			typ = "boolean"
		case "file":
			continue
		}
		cd.Params = append(cd.Params, commandParam{Name: f.Key, Type: typ})
	}
	return cd
}

// readFileFields: Datei-Felder (Typ file) wie im Formular des WebServers –
// Inhalt als Text, Dateiname in <feld>_name.
func readFileFields(def *defResponse, p params) error {
	if def == nil {
		return nil
	}
	for _, f := range def.Definition.Fields {
		name, ok := p[f.Key].(string)
		if f.Type != "file" || !ok {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("--%s: %w", f.Key, err)
		}
		p[f.Key], p[f.Key+"_name"] = string(b), filepath.Base(name)
	}
	return nil
}

func execute(ctx context.Context, c consolev1.ConsoleServiceClient, object, action string, p map[string]any, out any) error {
	s, err := structpb.NewStruct(p)
	if err != nil {
		return err
	}
	resp, err := c.Execute(ctx, &consolev1.ExecuteRequest{TargetObject: object, TargetAction: action, Parameters: s})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(resp.Payload.AsInterface())
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// matches: Muster mit * und ? (Groß-/Kleinschreibung egal); ohne Platzhalter
// genügt ein Teil des Namens, leer passt immer.
func matches(pattern, name string) bool {
	if pattern == "" {
		return true
	}
	p, n := strings.ToLower(pattern), strings.ToLower(name)
	if !strings.ContainsAny(p, "*?[") {
		return strings.Contains(n, p)
	}
	ok, _ := path.Match(p, n)
	return ok
}

func printTable(o options, head []string, rows [][]string, items []any) error {
	if o.format == "json" {
		if items == nil {
			items = []any{}
		}
		return printJSON(o, items)
	}
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "keine Treffer")
		return nil
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(head, "\t"))
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	w.Flush()
	return output(o.out, []byte(b.String()), "Liste")
}

func printJSON(o options, v any) error {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return output(o.out, append(body, '\n'), "Antwort")
}

func yes(b bool) string {
	if b {
		return "ja"
	}
	return ""
}
