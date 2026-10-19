package script

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Export und Import der Skripte und Berichte über die Fachobjekte – die Datei
// übersteht Releasewechsel (keine Tabellen, Format mit Version).

const (
	transferFormat  = "coremesh.scripts"
	transferVersion = 1
)

type transferFile struct {
	Format  string           `yaml:"format"`
	Version int              `yaml:"version"`
	Scripts []scriptTransfer `yaml:"scripts"`
	Reports []reportTransfer `yaml:"reports"`
}

type scriptTransfer struct {
	Code          string            `yaml:"code"`
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description,omitempty"`
	Source        string            `yaml:"source"`
	AllowedCalls  string            `yaml:"allowed_calls,omitempty"`
	TimeoutSec    int64             `yaml:"timeout_sec,omitempty"`
	LogRuns       string            `yaml:"log_runs,omitempty"`
	RetentionDays int64             `yaml:"retention_days,omitempty"`
	Active        bool              `yaml:"active"`
	Parameters    []paramTransfer   `yaml:"parameters,omitempty"`
	Bindings      []bindingTransfer `yaml:"bindings,omitempty"`
}

type paramTransfer struct {
	Name         string `yaml:"name"`
	Position     int64  `yaml:"position"`
	Label        string `yaml:"label"`
	Type         string `yaml:"type"`
	Required     bool   `yaml:"required,omitempty"`
	Default      string `yaml:"default_value,omitempty"`
	Options      string `yaml:"options,omitempty"`
	LookupObject string `yaml:"lookup_object,omitempty"`
	LookupValue  string `yaml:"lookup_value,omitempty"`
	LookupLabel  string `yaml:"lookup_label,omitempty"`
}

type bindingTransfer struct {
	Kind        string `yaml:"kind"`
	EventObject string `yaml:"event_object,omitempty"`
	EventAction string `yaml:"event_action,omitempty"`
	CompanyCode string `yaml:"company_code,omitempty"`
	Hook        string `yaml:"hook,omitempty"`
	Phase       string `yaml:"phase,omitempty"`
	Priority    int64  `yaml:"priority,omitempty"`
	Description string `yaml:"description,omitempty"`
	Active      bool   `yaml:"active"`
}

type reportTransfer struct {
	Code         string           `yaml:"code"`
	Name         string           `yaml:"name"`
	Description  string           `yaml:"description,omitempty"`
	SourceKind   string           `yaml:"source_kind"`
	SourceObject string           `yaml:"source_object,omitempty"`
	SourceAction string           `yaml:"source_action,omitempty"`
	Payload      string           `yaml:"source_payload,omitempty"`
	Stream       bool             `yaml:"stream,omitempty"`
	ResultPath   string           `yaml:"result_path,omitempty"`
	SourceScript string           `yaml:"source_script,omitempty"`
	GroupBy      string           `yaml:"group_by,omitempty"`
	SortBy       string           `yaml:"sort_by,omitempty"`
	RowLimit     int64            `yaml:"row_limit,omitempty"`
	Totals       bool             `yaml:"totals,omitempty"`
	Template     string           `yaml:"template,omitempty"`
	Language     string           `yaml:"template_language,omitempty"`
	CompanyParam string           `yaml:"company_param,omitempty"`
	Active       bool             `yaml:"active"`
	Columns      []columnTransfer `yaml:"columns,omitempty"`
	Parameters   []paramTransfer  `yaml:"parameters,omitempty"`
}

type columnTransfer struct {
	Position  int64  `yaml:"position"`
	Field     string `yaml:"field"`
	Label     string `yaml:"label,omitempty"`
	Aggregate string `yaml:"aggregate,omitempty"`
	Format    string `yaml:"format,omitempty"`
}

// transferIn: Payload aus der Maske ({data: {…}}) oder der Konsole (Parameter direkt).
func transferIn(payload any) map[string]any {
	in, _ := payload.(map[string]any)
	out := map[string]any{}
	for k, v := range in {
		out[strings.ReplaceAll(k, "-", "_")] = v
	}
	if d, ok := in["data"].(map[string]any); ok {
		for k, v := range d {
			out[k] = v
		}
	}
	return out
}

func (m *Module) exportAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	in := transferIn(req.Payload)
	only := map[string]bool{}
	for _, c := range splitList(crud.Str(in["codes"])) {
		only[strings.ToUpper(c)] = true
	}
	want := func(code string) bool { return len(only) == 0 || only[code] }
	f := transferFile{Format: transferFormat, Version: transferVersion, Scripts: []scriptTransfer{}, Reports: []reportTransfer{}}
	res, err := m.db.Query(ctx, `SELECT code, name, description, source, allowed_calls, timeout_sec, log_runs, retention_days, is_active FROM script__script ORDER BY code`)
	if err != nil {
		return sdk.Response{}, err
	}
	for _, r := range res.Rows {
		s := scriptTransfer{Code: crud.Str(r[0]), Name: crud.Str(r[1]), Description: crud.Str(r[2]), Source: crud.Str(r[3]), AllowedCalls: crud.Str(r[4]),
			TimeoutSec: toInt(r[5]), LogRuns: crud.Str(r[6]), RetentionDays: toInt(r[7]), Active: crud.AsBool(r[8])}
		if !want(s.Code) {
			continue
		}
		if s.Parameters, err = m.exportParams(ctx, "script__parameter", "script", s.Code); err != nil {
			return sdk.Response{}, err
		}
		b, err := m.db.Query(ctx, `SELECT kind, event_object, event_action, company_code, hook, phase, priority, description, is_active
			FROM script__binding WHERE script = ? ORDER BY kind, priority, id`, s.Code)
		if err != nil {
			return sdk.Response{}, err
		}
		for _, x := range b.Rows {
			s.Bindings = append(s.Bindings, bindingTransfer{Kind: crud.Str(x[0]), EventObject: crud.Str(x[1]), EventAction: crud.Str(x[2]),
				CompanyCode: crud.Str(x[3]), Hook: crud.Str(x[4]), Phase: crud.Str(x[5]), Priority: toInt(x[6]), Description: crud.Str(x[7]), Active: crud.AsBool(x[8])})
		}
		f.Scripts = append(f.Scripts, s)
	}
	res, err = m.db.Query(ctx, `SELECT code, name, description, source_kind, source_object, source_action, source_payload, stream, result_path, source_script,
		group_by, sort_by, row_limit, totals, template, template_language, company_param, is_active FROM script__report ORDER BY code`)
	if err != nil {
		return sdk.Response{}, err
	}
	for _, r := range res.Rows {
		s := func(i int) string { return crud.Str(r[i]) }
		rep := reportTransfer{Code: s(0), Name: s(1), Description: s(2), SourceKind: s(3), SourceObject: s(4), SourceAction: s(5), Payload: s(6),
			Stream: crud.AsBool(r[7]), ResultPath: s(8), SourceScript: s(9), GroupBy: s(10), SortBy: s(11), RowLimit: toInt(r[12]), Totals: crud.AsBool(r[13]),
			Template: s(14), Language: s(15), CompanyParam: s(16), Active: crud.AsBool(r[17])}
		if !want(rep.Code) {
			continue
		}
		cols, err := m.db.Query(ctx, `SELECT position, field, label, aggregate, format FROM script__report_column WHERE report = ? ORDER BY position`, rep.Code)
		if err != nil {
			return sdk.Response{}, err
		}
		for _, c := range cols.Rows {
			rep.Columns = append(rep.Columns, columnTransfer{Position: toInt(c[0]), Field: crud.Str(c[1]), Label: crud.Str(c[2]), Aggregate: crud.Str(c[3]), Format: crud.Str(c[4])})
		}
		if rep.Parameters, err = m.exportParams(ctx, "script__report_parameter", "report", rep.Code); err != nil {
			return sdk.Response{}, err
		}
		f.Reports = append(f.Reports, rep)
	}
	body, err := yaml.Marshal(f)
	if err != nil {
		return sdk.Response{}, err
	}
	text := "# Skripte und Berichte – Import: Skripte → „Importieren …“ bzw. console script:import --file=<datei>\n" + string(body)
	return sdk.Response{Payload: map[string]any{"message": fmt.Sprintf("%d Skripte, %d Berichte", len(f.Scripts), len(f.Reports)),
		"file_content": text,
		"download":     metamodel.Content{Content: base64.StdEncoding.EncodeToString([]byte(text)), ContentType: "application/x-yaml; charset=utf-8", FileName: "skripte.yaml"}}}, nil
}

func (m *Module) exportParams(ctx context.Context, table, owner, code string) ([]paramTransfer, error) {
	res, err := m.db.Query(ctx, `SELECT name, position, label, type, required, default_value, options, lookup_object, lookup_value, lookup_label
		FROM `+table+` WHERE `+owner+` = ? ORDER BY position, name`, code)
	if err != nil {
		return nil, err
	}
	var out []paramTransfer
	for _, r := range res.Rows {
		out = append(out, paramTransfer{Name: crud.Str(r[0]), Position: toInt(r[1]), Label: crud.Str(r[2]), Type: crud.Str(r[3]), Required: crud.AsBool(r[4]),
			Default: crud.Str(r[5]), Options: crud.Str(r[6]), LookupObject: crud.Str(r[7]), LookupValue: crud.Str(r[8]), LookupLabel: crud.Str(r[9])})
	}
	return out, nil
}

var errDryRun = errors.New("nur prüfen")

func (m *Module) importAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	in := transferIn(req.Payload)
	raw := crud.Str(in["file"])
	if raw == "" {
		return sdk.Response{}, crud.Invalid("Datei fehlt")
	}
	// Hochgeladene Dateien kommen als Text; Base64 (Binary) wird erkannt.
	if b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw)); err == nil && utf8.Valid(b) && strings.Contains(string(b), "format:") {
		raw = string(b)
	}
	var f transferFile
	if err := yaml.Unmarshal([]byte(raw), &f); err != nil {
		return sdk.Response{}, crud.Invalid("Datei: kein YAML im Format %s (%v)", transferFormat, err)
	}
	if f.Format != transferFormat {
		return sdk.Response{}, crud.Invalid("Datei: format %q erwartet, gefunden %q", transferFormat, f.Format)
	}
	if f.Version < 1 || f.Version > transferVersion {
		return sdk.Response{}, crud.Invalid("Datei: Version %d wird nicht unterstützt (bis %d) – neueres Release verwenden", f.Version, transferVersion)
	}
	dry := crud.AsBool(in["dry_run"])
	var stats importStats
	err := m.db.InTx(ctx, nil, func(ctx context.Context) error {
		for _, s := range f.Scripts {
			if err := m.importScript(ctx, s, &stats); err != nil {
				return fmt.Errorf("Skript %s: %w", s.Code, err)
			}
		}
		for _, r := range f.Reports {
			if err := m.importReport(ctx, r, &stats); err != nil {
				return fmt.Errorf("Bericht %s: %w", r.Code, err)
			}
		}
		if dry {
			return errDryRun
		}
		return nil
	})
	if err != nil && !errors.Is(err, errDryRun) {
		return sdk.Response{}, err
	}
	msg := fmt.Sprintf("%d neu, %d geändert (Skripte, Berichte, Parameter, Spalten, Bindungen)", stats.created, stats.updated)
	if dry {
		msg = "Prüfung ohne Fehler – " + msg + " – nichts gespeichert"
	}
	return sdk.Response{Payload: map[string]any{"message": msg, "created": stats.created, "updated": stats.updated}}, nil
}

type importStats struct{ created, updated int }

// upsert legt einen Datensatz an oder ändert ihn (über die Entity: dieselben Prüfungen wie in der Maske).
func (m *Module) upsert(ctx context.Context, object string, data map[string]any, active bool, st *importStats) (string, error) {
	e := m.set.Entity(object)
	if e.Surrogate {
		return "", fmt.Errorf("upsert: %s hat keinen fachlichen Schlüssel", object)
	}
	id := e.RecordID(data)
	if _, err := e.Load(ctx, e.KeyOf(data)); err == nil {
		upd := map[string]any{}
		for k, v := range data {
			if !e.Field(k).Immutable && !containsStr(e.Keys, k) {
				upd[k] = v
			}
		}
		if _, err := e.Update(ctx, map[string]any{"id": id, "data": upd}); err != nil {
			return "", err
		}
		st.updated++
	} else if errors.Is(err, sdk.ErrNotFound) {
		if _, err := e.Create(ctx, map[string]any{"data": data}); err != nil {
			return "", err
		}
		st.created++
	} else {
		return "", err
	}
	return id, m.setActive(ctx, object, id, active)
}

func (m *Module) setActive(ctx context.Context, object, id string, active bool) error {
	e := m.set.Entity(object)
	if e.StatusField == "" {
		return nil
	}
	rec, err := e.Load(ctx, mustKey(e, id))
	if err != nil {
		return err
	}
	switch is := crud.AsBool(rec[e.StatusField]); {
	case is && !active:
		_, err = e.Deactivate(ctx, map[string]any{"id": id})
	case !is && active:
		_, err = e.Reactivate(ctx, map[string]any{"id": id})
	}
	return err
}

func mustKey(e *crud.Entity, id string) crud.Record {
	k, _ := e.ParseID(id)
	return k
}

func containsStr(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (m *Module) importScript(ctx context.Context, s scriptTransfer, st *importStats) error {
	data := map[string]any{"code": s.Code, "name": s.Name, "description": s.Description, "source": s.Source, "allowed_calls": s.AllowedCalls,
		"timeout_sec": s.TimeoutSec, "log_runs": s.LogRuns, "retention_days": s.RetentionDays}
	if _, err := m.upsert(ctx, scriptObject, data, s.Active, st); err != nil {
		return err
	}
	for _, p := range s.Parameters {
		if _, err := m.upsert(ctx, paramObject, paramData("script", s.Code, p), true, st); err != nil {
			return fmt.Errorf("Parameter %s: %w", p.Name, err)
		}
	}
	for _, b := range s.Bindings {
		if err := m.importBinding(ctx, s.Code, b, st); err != nil {
			return fmt.Errorf("Bindung %s: %w", b.Kind, err)
		}
	}
	return nil
}

func paramData(owner, code string, p paramTransfer) map[string]any {
	return map[string]any{owner: strings.ToUpper(code), "name": p.Name, "position": p.Position, "label": p.Label, "type": p.Type, "required": p.Required,
		"default_value": p.Default, "options": p.Options, "lookup_object": p.LookupObject, "lookup_value": p.LookupValue, "lookup_label": p.LookupLabel}
}

// importBinding: gleiche Bindung (Art, Event bzw. Hook und Phase) ändern, sonst anlegen.
func (m *Module) importBinding(ctx context.Context, script string, b bindingTransfer, st *importStats) error {
	e := m.set.Entity(bindingObject)
	data := map[string]any{"script": strings.ToUpper(script), "kind": b.Kind, "event_object": b.EventObject, "event_action": b.EventAction,
		"company_code": b.CompanyCode, "hook": b.Hook, "phase": b.Phase, "priority": b.Priority, "description": b.Description}
	res, err := m.db.Query(ctx, `SELECT id FROM script__binding WHERE script = ? AND kind = ? AND COALESCE(event_object, '') = ? AND COALESCE(event_action, '') = ?
		AND COALESCE(company_code, '') = ? AND COALESCE(hook, '') = ? AND COALESCE(phase, '') = ?`,
		strings.ToUpper(script), b.Kind, b.EventObject, b.EventAction, b.CompanyCode, b.Hook, b.Phase)
	if err != nil {
		return err
	}
	var id string
	if len(res.Rows) > 0 {
		id = crud.Str(res.Rows[0][0])
		upd := map[string]any{}
		for k, v := range data {
			if k != "script" {
				upd[k] = v
			}
		}
		if _, err := e.Update(ctx, map[string]any{"id": id, "data": upd}); err != nil {
			return err
		}
		st.updated++
	} else {
		resp, err := e.Create(ctx, map[string]any{"data": data})
		if err != nil {
			return err
		}
		var rec struct {
			ID string `json:"id"`
		}
		b, _ := json.Marshal(resp.Payload)
		_ = json.Unmarshal(b, &rec)
		id = rec.ID
		st.created++
	}
	return m.setActive(ctx, bindingObject, id, b.Active)
}

func (m *Module) importReport(ctx context.Context, r reportTransfer, st *importStats) error {
	data := map[string]any{"code": r.Code, "name": r.Name, "description": r.Description, "source_kind": r.SourceKind, "source_object": r.SourceObject,
		"source_action": r.SourceAction, "source_payload": r.Payload, "stream": r.Stream, "result_path": r.ResultPath, "source_script": r.SourceScript,
		"group_by": r.GroupBy, "sort_by": r.SortBy, "row_limit": r.RowLimit, "totals": r.Totals, "template": r.Template,
		"template_language": r.Language, "company_param": r.CompanyParam}
	if _, err := m.upsert(ctx, reportObject, data, r.Active, st); err != nil {
		return err
	}
	for _, c := range r.Columns {
		if _, err := m.upsert(ctx, reportColObject, map[string]any{"report": strings.ToUpper(r.Code), "position": c.Position, "field": c.Field,
			"label": c.Label, "aggregate": c.Aggregate, "format": c.Format}, true, st); err != nil {
			return fmt.Errorf("Spalte %d: %w", c.Position, err)
		}
	}
	for _, p := range r.Parameters {
		if _, err := m.upsert(ctx, reportParamObject, paramData("report", r.Code, p), true, st); err != nil {
			return fmt.Errorf("Parameter %s: %w", p.Name, err)
		}
	}
	return nil
}
