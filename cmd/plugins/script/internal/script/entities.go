package script

import (
	"context"
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/hook"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

var (
	codeRe   = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,39}$`)
	nameRe   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	objectRe = regexp.MustCompile(`^(\*|[A-Z][A-Za-z0-9]*)$`)
	actionRe = regexp.MustCompile(`^(\*|[a-zA-Z][A-Za-z0-9_-]*)$`)

	logOptions = []metamodel.Option{{Value: "ALL", Label: "alle Läufe"}, {Value: "ERRORS", Label: "nur Fehler"}, {Value: "NONE", Label: "keine"}}
	paramTypes = []metamodel.Option{{Value: "TEXT", Label: "Text"}, {Value: "NUMBER", Label: "Zahl"}, {Value: "DATE", Label: "Datum"},
		{Value: "BOOLEAN", Label: "Ja/Nein"}, {Value: "SELECT", Label: "Auswahl"}}
	bindingKinds = []metamodel.Option{{Value: trigEvent, Label: "Event (nach dem Speichern, asynchron)"}, {Value: trigHook, Label: "Hook (synchron: ändern, prüfen, nach dem Speichern)"}}
	phases       = []metamodel.Option{{Value: hook.PhaseModify, Label: "modify – Daten ändern"}, {Value: hook.PhaseCheck, Label: "check – prüfen (Fehler bricht ab)"},
		{Value: hook.PhaseCommit, Label: "commit – nach dem Speichern"}}
	triggers    = []metamodel.Option{{Value: trigRun, Label: "Maske"}, {Value: trigEvent, Label: "Event"}, {Value: trigHook, Label: "Hook"}, {Value: trigReport, Label: "Bericht"}}
	runStatus   = []metamodel.Option{{Value: stDone, Label: "Fertig"}, {Value: stError, Label: "Fehler"}}
	sourceKinds = []metamodel.Option{{Value: "OBJECT", Label: "Object (Action eines Moduls)"}, {Value: "SCRIPT", Label: "Skript (liefert Zeilen)"}}
	aggregates  = []metamodel.Option{{Value: "NONE", Label: "keine (Wert bzw. erster Wert der Gruppe)"}, {Value: "SUM", Label: "Summe"},
		{Value: "COUNT", Label: "Anzahl"}, {Value: "MIN", Label: "Minimum"}, {Value: "MAX", Label: "Maximum"}, {Value: "AVG", Label: "Durchschnitt"}}
	formats = []metamodel.Option{{Value: "TEXT", Label: "Text"}, {Value: "NUMBER", Label: "Zahl (zwei Nachkommastellen)"}, {Value: "INTEGER", Label: "ganze Zahl"},
		{Value: "MONEY", Label: "Betrag"}, {Value: "MONEY_MINOR", Label: "Betrag aus kleinster Einheit (Cent)"}, {Value: "DATE", Label: "Datum (TT.MM.JJJJ)"}}

	refScript = &crud.Ref{Table: "script__script", Column: "code", Label: "Skript", Object: scriptObject, LabelFields: []string{"name"}, ActiveField: "is_active"}
	refReport = &crud.Ref{Table: "script__report", Column: "code", Label: "Bericht", Object: reportObject, LabelFields: []string{"name"}}
)

// defaultAllowedCalls: Vorschlag für neue Skripte – nur lesen und Vorlagen.
const defaultAllowedCalls = `# je Zeile ein Muster Object.action (* = beliebig), # = Kommentar
*.list
*.get
*.getAggregate
Template.render
Template.get`

func checkCode(rec crud.Record, key, label string) error {
	v := strings.ToUpper(strings.TrimSpace(crud.Str(rec[key])))
	if !codeRe.MatchString(v) {
		return crud.Invalid("%s %q: Großbuchstaben, Ziffern, _ und - (höchstens 40 Zeichen)", label, v)
	}
	rec[key] = v
	return nil
}

func checkOption(rec crud.Record, key, label string, opts []metamodel.Option, def string) error {
	v := firstOf(strings.ToUpper(strings.TrimSpace(crud.Str(rec[key]))), def)
	if !slices.ContainsFunc(opts, func(o metamodel.Option) bool { return o.Value == v }) {
		return crud.Invalid("%s %q unbekannt", label, v)
	}
	rec[key] = v
	return nil
}

// --- Skripte --------------------------------------------------------------------------

func (m *Module) scriptEntity() *crud.Entity {
	return &crud.Entity{
		Object: scriptObject, Title: "Skripte", Icon: "icon-code", Table: "script__script", Section: "Skripte",
		Keys: []string{"code"}, Order: "code", StatusField: "is_active", TitleField: "name",
		Filters: []string{"is_active"}, Search: []string{"code", "name", "description", "source"},
		Fields: []crud.Field{
			{Key: "code", Label: "Schlüssel", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			{Key: "description", Label: "Beschreibung", Type: tArea},
			{Key: "source", Label: "Quelltext (JavaScript; return liefert das Ergebnis)", Type: tArea, Required: true},
			{Key: "allowed_calls", Label: "Erlaubte Aufrufe (je Zeile Object.action, * = beliebig, z. B. *.list; leer = keine)", Type: tArea},
			{Key: "timeout_sec", Label: "Zeitlimit (Sekunden)", Short: "Zeitlimit", Type: tNum, Listable: true},
			{Key: "log_runs", Label: "Protokoll", Type: tSel, Listable: true, Options: logOptions},
			{Key: "retention_days", Label: "Protokoll aufbewahren (Tage)", Short: "Tage", Type: tNum},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
			{Key: "codes", Label: "Skripte und Berichte (Schlüssel, Komma-Liste; leer = alle)", Type: tText, ActionOnly: true},
			{Key: "file", Label: "Datei (YAML aus „Exportieren …“)", Type: tFile, ActionOnly: true},
			{Key: "dry_run", Label: "Nur prüfen, nichts ändern", Type: tBool, ActionOnly: true},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "skript", Title: "Skript", Fields: []string{"code", "name", "description", "timeout_sec", "log_runs", "retention_days", "is_active"}},
			{Key: "quelltext", Title: "Quelltext", Fields: []string{"source"}},
			{Key: "aufrufe", Title: "Erlaubte Aufrufe", Fields: []string{"allowed_calls"}},
			{Key: "parameter", Title: "Parameter", Relation: &metamodel.Relation{Object: paramObject, ForeignKey: "script",
				Columns: []string{"position", "name", "label", "type", "required", "default_value", "lookup_object"}}},
			{Key: "bindungen", Title: "Bindungen (Events, Hooks)", Relation: &metamodel.Relation{Object: bindingObject, ForeignKey: "script",
				Columns: []string{"kind", "event_object", "event_action", "company_code", "hook", "phase", "priority", "is_active"}}},
			{Key: "protokoll", Title: "Protokoll", Collapsed: true, Relation: &metamodel.Relation{Object: runObject, ForeignKey: "script",
				Columns: []string{"run_id", "trigger", "detail", "status", "message", "duration_ms", "created_at"}}},
		},
		Validate: m.checkScript,
		Actions: []crud.Action{
			{ActionConfig: metamodel.ActionConfig{Name: "run", Label: "Ausführen …", Record: true, FormState: true}, Handle: m.runAction},
			{ActionConfig: metamodel.ActionConfig{Name: "export", Label: "Exportieren …", Fields: []string{"codes"}}, Handle: m.exportAction},
			{ActionConfig: metamodel.ActionConfig{Name: "import", Label: "Importieren …", Fields: []string{"file", "dry_run"}}, Handle: m.importAction},
		},
		FormState: m.scriptFormState,
	}
}

func (m *Module) checkScript(ctx context.Context, rec, _ crud.Record) error {
	if err := checkCode(rec, "code", "Schlüssel"); err != nil {
		return err
	}
	if _, err := compile(crud.Str(rec["code"]), crud.Str(rec["source"])); err != nil {
		return crud.Invalid("Quelltext: %v", err)
	}
	if _, err := parsePatterns(crud.Str(rec["allowed_calls"])); err != nil {
		return err
	}
	if t := toInt(rec["timeout_sec"]); t <= 0 {
		rec["timeout_sec"] = 30
	} else if t > int64(m.cfg.MaxTimeoutSec) {
		return crud.Invalid("Zeitlimit: höchstens %d Sekunden", m.cfg.MaxTimeoutSec)
	}
	if toInt(rec["retention_days"]) <= 0 {
		rec["retention_days"] = 30
	}
	return checkOption(rec, "log_runs", "Protokoll", logOptions, "ALL")
}

// parsePatterns liest die erlaubten Aufrufe: je Zeile Object.action mit *,
// Leerzeilen und # Kommentare zählen nicht.
func parsePatterns(s string) ([]string, error) {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if i := strings.Index(l, "#"); i >= 0 {
			l = strings.TrimSpace(l[:i])
		}
		if l == "" {
			continue
		}
		if _, err := path.Match(l, "X.y"); err != nil || !strings.Contains(l, ".") {
			return nil, crud.Invalid("Erlaubte Aufrufe: %q – Muster Object.action erwartet (z. B. *.list, Contract.*)", l)
		}
		out = append(out, l)
	}
	return out, nil
}

// --- Parameter (Skripte und Berichte) -----------------------------------------------

func (m *Module) paramEntity(object, table, owner, ownerLabel string, ref *crud.Ref) *crud.Entity {
	title := "Parameter der Skripte"
	if object == reportParamObject {
		title = "Parameter der Berichte"
	}
	return &crud.Entity{
		Object: object, Title: title, Icon: "icon-sliders", Table: table, Section: "Einstellungen",
		Keys: []string{owner, "name"}, Order: owner + ", position, name", Filters: []string{owner, "type"},
		Fields: []crud.Field{
			{Key: owner, Label: ownerLabel, Type: tText, Required: true, Listable: true, Immutable: true, Ref: ref},
			{Key: "name", Label: "Name (im Skript params.name, im Bericht {name})", Short: "Name", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "position", Label: "Reihenfolge", Short: "Pos.", Type: tNum, Listable: true},
			{Key: "label", Label: "Beschriftung in der Maske", Short: "Beschriftung", Type: tText, Required: true, Listable: true},
			{Key: "type", Label: "Typ", Type: tSel, Required: true, Listable: true, Options: paramTypes, Trigger: true},
			{Key: "required", Label: "Pflicht", Type: tBool, Listable: true},
			{Key: "default_value", Label: "Vorbelegung (Pflichtdatum leer = Stichtag)", Short: "Vorbelegung", Type: tText, Listable: true},
			{Key: "options", Label: "Auswahlwerte (je Zeile WERT=Text)", Type: tArea, ShowIf: &metamodel.Condition{Field: "type", Values: []string{"SELECT"}}},
			{Key: "lookup_object", Label: "Auswahl aus Object (z. B. CompanyCode)", Short: "Auswahl aus", Type: tText, Listable: true,
				ShowIf: &metamodel.Condition{Field: "type", Values: []string{"TEXT"}}},
			{Key: "lookup_value", Label: "Feld mit dem Wert (z. B. code)", Type: tText, ShowIf: &metamodel.Condition{Field: "type", Values: []string{"TEXT"}}},
			{Key: "lookup_label", Label: "Felder für den Text (Komma-Liste, z. B. name)", Type: tText, ShowIf: &metamodel.Condition{Field: "type", Values: []string{"TEXT"}}},
		},
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			n := strings.TrimSpace(crud.Str(rec["name"]))
			if !nameRe.MatchString(n) {
				return crud.Invalid("Name %q: Kleinbuchstaben, Ziffern, _ (beginnt mit einem Buchstaben)", n)
			}
			rec["name"] = n
			if err := checkOption(rec, "type", "Typ", paramTypes, "TEXT"); err != nil {
				return err
			}
			if toInt(rec["position"]) == 0 {
				rec["position"] = 10
			}
			if rec["type"] == "SELECT" {
				if len(parseOptions(crud.Str(rec["options"]))) == 0 {
					return crud.Invalid("Auswahl: Auswahlwerte fehlen (je Zeile WERT=Text)")
				}
			} else {
				rec["options"] = nil
			}
			if rec["type"] != "TEXT" {
				rec["lookup_object"], rec["lookup_value"], rec["lookup_label"] = nil, nil, nil
			}
			obj := strings.TrimSpace(crud.Str(rec["lookup_object"]))
			if obj != "" && (!objectRe.MatchString(obj) || obj == "*") {
				return crud.Invalid("Auswahl aus Object %q: Name eines Objects (z. B. CompanyCode)", obj)
			}
			rec["lookup_object"] = nilIfEmpty(obj)
			if obj != "" {
				rec["lookup_value"] = firstOf(crud.Str(rec["lookup_value"]), "code")
			}
			rec["default_value"] = nilIfEmpty(crud.Str(rec["default_value"]))
			return nil
		},
	}
}

// parseOptions: je Zeile WERT=Text (ohne = ist der Wert auch der Text).
func parseOptions(s string) []metamodel.Option {
	var out []metamodel.Option
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		v, t, ok := strings.Cut(l, "=")
		if !ok {
			t = v
		}
		out = append(out, metamodel.Option{Value: strings.TrimSpace(v), Label: strings.TrimSpace(t)})
	}
	return out
}

// --- Bindungen ------------------------------------------------------------------------

func (m *Module) bindingEntity() *crud.Entity {
	isEvent := &metamodel.Condition{Field: "kind", Values: []string{trigEvent}}
	isHook := &metamodel.Condition{Field: "kind", Values: []string{trigHook}}
	return &crud.Entity{
		Object: bindingObject, Title: "Bindungen", Icon: "icon-link", Table: "script__binding", Section: "Skripte",
		Keys: []string{"id"}, Surrogate: true, Order: "script, kind, priority, id", StatusField: "is_active",
		Filters: []string{"script", "kind", "event_object", "hook", "is_active"},
		Fields: []crud.Field{
			{Key: "id", Label: "Nummer", Type: tText, ReadOnly: true},
			{Key: "script", Label: "Skript", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refScript},
			{Key: "kind", Label: "Art", Type: tSel, Required: true, Listable: true, Options: bindingKinds, Trigger: true},
			{Key: "event_object", Label: "Object des Events (z. B. JournalEntry, * = alle)", Short: "Object", Type: tText, Listable: true,
				ShowIf: isEvent, RequiredIf: isEvent},
			{Key: "event_action", Label: "Action des Events (z. B. post, create; leer = alle)", Short: "Action", Type: tText, Listable: true, ShowIf: isEvent},
			{Key: "company_code", Label: "Buchungskreis (leer = alle)", Short: "BuKr", Type: tText, Listable: true, ShowIf: isEvent},
			{Key: "hook", Label: "Hook (z. B. ledger.posting)", Type: tText, Listable: true, ShowIf: isHook, RequiredIf: isHook,
				Lookup: &metamodel.Lookup{Object: "Hook", ValueField: "name", LabelFields: []string{"description"}}},
			{Key: "phase", Label: "Phase", Type: tSel, Listable: true, Options: phases, ShowIf: isHook, RequiredIf: isHook},
			{Key: "priority", Label: "Priorität (aufsteigend; Abo des Moduls mit der kleinsten)", Short: "Prio.", Type: tNum, Listable: true, ShowIf: isHook},
			{Key: "description", Label: "Beschreibung", Type: tText, Listable: true},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
		},
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			if err := checkOption(rec, "kind", "Art", bindingKinds, ""); err != nil {
				return err
			}
			if rec["kind"] == trigEvent {
				obj, act := firstOf(crud.Str(rec["event_object"]), "*"), firstOf(crud.Str(rec["event_action"]), "*")
				if !objectRe.MatchString(obj) || !actionRe.MatchString(act) {
					return crud.Invalid("Event: Object %q bzw. Action %q ungültig", obj, act)
				}
				rec["event_object"], rec["event_action"] = obj, act
				rec["company_code"] = firstOf(crud.Str(rec["company_code"]), "*")
				rec["hook"], rec["phase"], rec["priority"] = nil, nil, 100
				return nil
			}
			h := strings.TrimSpace(crud.Str(rec["hook"]))
			if !regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9_.-]*$`).MatchString(h) {
				return crud.Invalid("Hook %q: <modul>.<punkt>, z. B. ledger.posting", h)
			}
			rec["hook"] = h
			ph := strings.ToLower(strings.TrimSpace(crud.Str(rec["phase"])))
			if !slices.ContainsFunc(phases, func(o metamodel.Option) bool { return o.Value == ph }) {
				return crud.Invalid("Phase %q unbekannt", ph)
			}
			rec["phase"] = ph
			if toInt(rec["priority"]) <= 0 {
				rec["priority"] = 100
			}
			rec["event_object"], rec["event_action"], rec["company_code"] = nil, nil, nil
			return nil
		},
		AfterCreate: func(context.Context, crud.Record) error { m.requestSync(); return nil },
		AfterUpdate: func(context.Context, crud.Record) error { m.requestSync(); return nil },
	}
}

// --- Protokoll ------------------------------------------------------------------------

func (m *Module) runEntity() *crud.Entity {
	return &crud.Entity{
		Object: runObject, Title: "Protokoll", Icon: "icon-list", Table: "script__run", Section: "Skripte",
		Keys: []string{"run_id"}, Order: "created_at DESC, run_id DESC", NoCreate: true, TitleField: "run_id",
		Filters: []string{"script", "report", "trigger", "status", "keep"}, Search: []string{"run_id", "script", "report", "detail", "message"},
		Fields: []crud.Field{
			{Key: "run_id", Label: "Lauf", Type: tText, Listable: true, ReadOnly: true},
			{Key: "script", Label: "Skript", Type: tText, Listable: true, ReadOnly: true, Ref: refScript},
			{Key: "report", Label: "Bericht", Type: tText, Listable: true, ReadOnly: true, Ref: refReport},
			{Key: "trigger", Label: "Auslöser", Type: tSel, Listable: true, ReadOnly: true, Options: triggers},
			{Key: "detail", Label: "Event bzw. Hook", Type: tText, Listable: true, ReadOnly: true},
			{Key: "status", Label: "Status", Type: tSel, Listable: true, ReadOnly: true, Options: runStatus},
			{Key: "message", Label: "Meldung", Type: tText, Listable: true, ReadOnly: true},
			{Key: "duration_ms", Label: "Dauer (ms)", Type: tNum, Listable: true, ReadOnly: true},
			{Key: "keep", Label: "Nicht löschen", Short: "Behalten", Type: tBool, Listable: true},
			{Key: "created_at", Label: "Gestartet", Type: tText, Listable: true, ReadOnly: true},
			{Key: "created_by", Label: "Benutzer", Type: tText, ReadOnly: true, Lookup: crud.LookupUser},
			{Key: "expires_on", Label: "Verfällt am", Type: tText, ReadOnly: true},
			{Key: "log", Label: "Ausgaben (log)", Type: tArea, ReadOnly: true},
			{Key: "result", Label: "Ergebnis", Type: tArea, ReadOnly: true},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "lauf", Title: "Lauf", Fields: []string{"run_id", "script", "report", "trigger", "detail", "status", "message", "duration_ms",
				"keep", "created_at", "created_by", "expires_on"}},
			{Key: "ausgaben", Title: "Ausgaben", Fields: []string{"log"}},
			{Key: "ergebnis", Title: "Ergebnis", Collapsed: true, Fields: []string{"result"}},
		},
		ListScope: func(ctx context.Context) (string, []any, bool, error) { m.cleanup(ctx); return "", nil, false, nil },
	}
}

// --- Berichte -------------------------------------------------------------------------

func (m *Module) reportEntity() *crud.Entity {
	isObject := &metamodel.Condition{Field: "source_kind", Values: []string{"OBJECT"}}
	isScript := &metamodel.Condition{Field: "source_kind", Values: []string{"SCRIPT"}}
	return &crud.Entity{
		Object: reportObject, Title: "Berichte", Icon: "icon-bar-chart", Table: "script__report", Section: "Berichte",
		Keys: []string{"code"}, Order: "code", StatusField: "is_active", TitleField: "name",
		Filters: []string{"source_kind", "is_active"}, Search: []string{"code", "name", "description", "source_object"},
		Fields: []crud.Field{
			{Key: "code", Label: "Schlüssel", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			{Key: "description", Label: "Beschreibung", Type: tArea},
			{Key: "source_kind", Label: "Datenquelle", Type: tSel, Required: true, Listable: true, Options: sourceKinds, Trigger: true},
			{Key: "source_object", Label: "Object (z. B. JournalEntryItem, AccountBalance)", Short: "Object", Type: tText, Listable: true,
				ShowIf: isObject, RequiredIf: isObject},
			{Key: "source_action", Label: "Action (leer = list)", Short: "Action", Type: tText, ShowIf: isObject},
			{Key: "source_payload", Label: "Anfrage (JSON mit Platzhaltern {parameter}, z. B. {\"query\": {\"company_code_id\": \"{company_code}\"}})",
				Type: tArea, ShowIf: isObject},
			{Key: "stream", Label: "Als Datenstrom lesen (große Datenmengen, z. B. Einzelposten)", Short: "Strom", Type: tBool, ShowIf: isObject},
			{Key: "result_path", Label: "Zeilen in der Antwort unter (Pfad mit Punkten; leer = items bzw. rows)", Type: tText, ShowIf: isObject},
			{Key: "source_script", Label: "Skript (return liefert die Zeilen; Parameter wie beim Bericht)", Short: "Skript", Type: tText, Listable: true,
				Ref: refScript, ShowIf: isScript, RequiredIf: isScript},
			{Key: "group_by", Label: "Gruppieren nach (Felder, Komma-Liste)", Short: "Gruppen", Type: tText, Listable: true},
			{Key: "sort_by", Label: "Sortieren nach (Felder, Komma-Liste; -feld = absteigend)", Type: tText},
			{Key: "section_by", Label: "Abschnitte je (Feld; Zwischenüberschrift und -summe je Wert, z. B. Gebäude)", Short: "Abschnitte", Type: tText},
			{Key: "subtitle", Label: "Untertitel (Platzhalter {parameter}, z. B. Zeitraum {von} – {bis})", Type: tText},
			{Key: "row_limit", Label: "Höchstens Zeilen (leer = alle)", Type: tNum},
			{Key: "totals", Label: "Summenzeile", Type: tBool},
			{Key: "template", Label: "Vorlage (Modul Vorlagen, z. B. BERICHT für PDF; CSV gibt es immer dazu)", Short: "Vorlage", Type: tText, Listable: true,
				Lookup: &metamodel.Lookup{Object: "TemplateDefinition", ValueField: "code", LabelFields: []string{"name"}, Columns: []string{"code", "language", "name"}}},
			{Key: "template_language", Label: "Sprache der Vorlage (leer = de)", Type: tText},
			{Key: "company_param", Label: "Parameter mit dem Buchungskreis (für die Vorlage; leer = company_code)", Type: tText},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "bericht", Title: "Bericht", Fields: []string{"code", "name", "description", "is_active"}},
			{Key: "quelle", Title: "Datenquelle", Fields: []string{"source_kind", "source_object", "source_action", "source_payload", "stream", "result_path", "source_script"}},
			{Key: "aufbau", Title: "Aufbau und Ausgabe", Fields: []string{"group_by", "sort_by", "section_by", "row_limit", "totals", "subtitle", "template", "template_language", "company_param"}},
			{Key: "spalten", Title: "Spalten", Relation: &metamodel.Relation{Object: reportColObject, ForeignKey: "report",
				Columns: []string{"position", "field", "label", "aggregate", "format", "width"}}},
			{Key: "parameter", Title: "Parameter", Relation: &metamodel.Relation{Object: reportParamObject, ForeignKey: "report",
				Columns: []string{"position", "name", "label", "type", "required", "default_value", "lookup_object"}}},
			{Key: "protokoll", Title: "Protokoll", Collapsed: true, Relation: &metamodel.Relation{Object: runObject, ForeignKey: "report",
				Columns: []string{"run_id", "status", "message", "duration_ms", "created_at"}}},
		},
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			if err := checkCode(rec, "code", "Schlüssel"); err != nil {
				return err
			}
			if err := checkOption(rec, "source_kind", "Datenquelle", sourceKinds, "OBJECT"); err != nil {
				return err
			}
			if rec["source_kind"] == "OBJECT" {
				obj, act := strings.TrimSpace(crud.Str(rec["source_object"])), firstOf(crud.Str(rec["source_action"]), "list")
				if !objectRe.MatchString(obj) || obj == "*" || !actionRe.MatchString(act) || act == "*" {
					return crud.Invalid("Datenquelle: Object %q bzw. Action %q ungültig", obj, act)
				}
				rec["source_object"], rec["source_action"], rec["source_script"] = obj, act, nil
				if p := strings.TrimSpace(crud.Str(rec["source_payload"])); p != "" {
					var v any
					if err := json.Unmarshal([]byte(p), &v); err != nil {
						return crud.Invalid("Anfrage: kein gültiges JSON (%v)", err)
					}
				}
				rec["source_payload"], rec["result_path"] = nilIfEmpty(crud.Str(rec["source_payload"])), nilIfEmpty(crud.Str(rec["result_path"]))
			} else {
				rec["source_object"], rec["source_action"], rec["source_payload"], rec["result_path"], rec["stream"] = nil, nil, nil, nil, false
			}
			if toInt(rec["row_limit"]) < 0 {
				return crud.Invalid("Höchstens Zeilen: nicht negativ")
			}
			if toInt(rec["row_limit"]) == 0 {
				rec["row_limit"] = nil
			}
			for _, k := range []string{"group_by", "sort_by", "section_by", "subtitle", "template", "template_language", "company_param"} {
				rec[k] = nilIfEmpty(crud.Str(rec[k]))
			}
			return nil
		},
		Actions: []crud.Action{
			{ActionConfig: metamodel.ActionConfig{Name: "run", Label: "Ausführen …", Record: true, FormState: true}, Handle: m.reportRunAction},
		},
		FormState: m.reportFormState,
	}
}

func (m *Module) reportColumnEntity() *crud.Entity {
	return &crud.Entity{
		Object: reportColObject, Title: "Spalten der Berichte", Icon: "icon-columns", Table: "script__report_column", Section: "Einstellungen",
		Keys: []string{"report", "position"}, Order: "report, position", Filters: []string{"report"},
		Fields: []crud.Field{
			{Key: "report", Label: "Bericht", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refReport},
			{Key: "position", Label: "Reihenfolge", Short: "Pos.", Type: tNum, Required: true, Listable: true, Immutable: true},
			{Key: "field", Label: "Feld der Datenquelle", Short: "Feld", Type: tText, Required: true, Listable: true},
			{Key: "label", Label: "Überschrift (leer = Feld)", Short: "Überschrift", Type: tText, Listable: true},
			{Key: "aggregate", Label: "Verdichtung (bei Gruppen und in der Summenzeile)", Short: "Verdichtung", Type: tSel, Listable: true, Options: aggregates},
			{Key: "format", Label: "Format", Type: tSel, Listable: true, Options: formats},
			{Key: "width", Label: "Breite im PDF (relativ; leer = Text 3, Zahl 1)", Short: "Breite", Type: tNum, Listable: true},
		},
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`).MatchString(strings.TrimSpace(crud.Str(rec["field"]))) {
				return crud.Invalid("Feld %q: Name eines Felds (Punkte für eingebettete Werte)", crud.Str(rec["field"]))
			}
			rec["field"] = strings.TrimSpace(crud.Str(rec["field"]))
			if err := checkOption(rec, "aggregate", "Verdichtung", aggregates, "NONE"); err != nil {
				return err
			}
			rec["label"] = nilIfEmpty(crud.Str(rec["label"]))
			if w, ok := toFloat(rec["width"]); ok && w < 0 {
				return crud.Invalid("Breite: nicht negativ")
			} else if !ok || w == 0 {
				rec["width"] = nil
			}
			return checkOption(rec, "format", "Format", formats, "TEXT")
		},
	}
}
