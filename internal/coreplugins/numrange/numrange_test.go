package numrange

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/coremesh-labs/coremesh/internal/config"
	"github.com/coremesh-labs/coremesh/internal/coreplugins/dbschema"
	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	api "github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

func setup(t *testing.T) *Plugin {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, map[string]config.Database{
		"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "nr.db") + "?_pragma=busy_timeout(5000)"},
	}, database.Options{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	schema := dbschema.New(db)
	if err := schema.Configure(ctx, sdk.Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := schema.Handle(ctx, sdk.Request{Object: "DBSchema", Action: "Activate",
		Payload: map[string]any{"module": Name, "version": Version, "schema": schemaHCL}}); err != nil {
		t.Fatalf("Schema: %v", err)
	}
	p := New(db)
	if err := p.Configure(ctx, sdk.Config{}); err != nil {
		t.Fatal(err)
	}
	return p
}

var testCtx = sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r1", UserID: "u1"})

func do(p *Plugin, object, action string, payload any) (sdk.Response, error) {
	return p.Handle(testCtx, sdk.Request{Object: object, Action: action, Payload: payload})
}

func must(t *testing.T, p *Plugin, object, action string, payload any) sdk.Response {
	t.Helper()
	resp, err := do(p, object, action, payload)
	if err != nil {
		t.Fatalf("%s.%s: %v", object, action, err)
	}
	return resp
}

func next(t *testing.T, p *Plugin, r api.Request) api.Result {
	t.Helper()
	return must(t, p, api.Object, api.ActionNext, r).Payload.(api.Result)
}

func defineContract(t *testing.T, p *Plugin) {
	t.Helper()
	must(t, p, api.Object, api.ActionDefine, api.Definition{Object: "Contract", Description: "Vertragsnummern", Owner: "contract",
		PerCompanyCode: true, PerYear: true, Pattern: "{KEY}-{YYYY}-{N}", Width: 4})
}

func update(t *testing.T, p *Plugin, id string, data map[string]any) error {
	t.Helper()
	_, err := do(p, api.Object, "update", map[string]any{"id": id, "data": data})
	return err
}

// TestNextAndYears: Format, Schlüssel je Vertragsart, neues Jahr übernimmt die
// Einstellungen des Vorjahrs, Protokoll.
func TestNextAndYears(t *testing.T) {
	p := setup(t)
	defineContract(t, p)
	mv := api.Request{Object: "Contract", CompanyCode: "1000", Key: "mv", Year: 2026, Reference: "Mietvertrag Müller"}
	if r := next(t, p, mv); r.Number != "MV-2026-0001" || r.Interval != "Contract|1000|MV|2026" {
		t.Fatalf("erste Nummer: %+v", r)
	}
	if r := next(t, p, mv); r.Number != "MV-2026-0002" {
		t.Fatalf("zweite: %v", r.Number)
	}
	if r := next(t, p, api.Request{Object: "Contract", CompanyCode: "1000", Key: "VS", Year: 2026}); r.Number != "VS-2026-0001" {
		t.Fatalf("anderer Schlüssel: %v", r.Number)
	}
	if r := next(t, p, api.Request{Object: "Contract", CompanyCode: "2000", Key: "MV", Year: 2026}); r.Number != "MV-2026-0001" {
		t.Fatalf("anderer Buchungskreis: %v", r.Number)
	}
	// 2026 auf fünf Stellen ab 100 umgestellt: 2027 übernimmt das.
	if err := update(t, p, "Contract|1000|MV|2026", map[string]any{"width": 5, "from_number": 100, "to_number": 99999, "current_number": 0}); err != nil {
		t.Fatal(err)
	}
	if r := next(t, p, api.Request{Object: "Contract", CompanyCode: "1000", Key: "MV", Year: 2027}); r.Number != "MV-2027-00100" {
		t.Fatalf("Folgejahr: %v", r.Number)
	}
	logs := must(t, p, "NumberRangeLog", "list", map[string]any{"query": map[string]any{"interval_id": "Contract|1000|MV|2026"}}).Payload.(map[string]any)["items"].([]any)
	if len(logs) != 2 || logs[0].(map[string]any)["reference"] != "Mietvertrag Müller" || logs[0].(map[string]any)["user_id"] != "u1" {
		t.Fatalf("Protokoll: %v", logs)
	}
	got := must(t, p, api.Object, "get", map[string]any{"id": "Contract|1000|MV|2027"}).Payload.(map[string]any)
	if got["next_number"] != "MV-2027-00101" || got["current_number"] != int64(100) {
		t.Fatalf("Intervall: %v", got)
	}

	_, err := do(p, api.Object, api.ActionNext, api.Request{Object: "Contract", Key: "MV"})
	expect(t, err, sdk.ErrInvalidArgument, "ohne Buchungskreis")
	_, err = do(p, api.Object, api.ActionNext, api.Request{Object: "Gibtsnicht"})
	expect(t, err, sdk.ErrNotFound, "Objekt nicht angemeldet")
}

// TestOverflow: Fehler, Neubeginn, Folgeintervall; Warnschwelle.
func TestOverflow(t *testing.T) {
	p := setup(t)
	defineContract(t, p)
	r := api.Request{Object: "Contract", CompanyCode: "1000", Key: "MV", Year: 2026}
	next(t, p, r)
	id := "Contract|1000|MV|2026"
	if err := update(t, p, id, map[string]any{"to_number": 4, "warn_percent": 75}); err != nil {
		t.Fatal(err)
	}
	next(t, p, r)
	if w := next(t, p, r); !strings.Contains(w.Warning, "75 %") {
		t.Fatalf("Warnschwelle: %+v", w)
	}
	next(t, p, r) // 4 = bis
	_, err := do(p, api.Object, api.ActionNext, r)
	expect(t, err, sdk.ErrResourceExhausted, "erschöpft")

	if err := update(t, p, id, map[string]any{"overflow": api.OverflowNext}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("NEXT ohne Folgeintervall: %v", err)
	}
	if err := update(t, p, id, map[string]any{"overflow": api.OverflowNext, "next_key": "mv2"}); err != nil {
		t.Fatal(err)
	}
	if res := next(t, p, r); res.Number != "MV2-2026-0001" || !strings.Contains(res.Warning, "weiter in MV2") {
		t.Fatalf("Folgeintervall: %+v", res)
	}
	if err := update(t, p, id, map[string]any{"overflow": api.OverflowRestart}); err != nil {
		t.Fatal(err)
	}
	if res := next(t, p, r); res.Number != "MV-2026-0001" || !strings.Contains(res.Warning, "beginnt wieder") {
		t.Fatalf("Neubeginn: %+v", res)
	}
}

// TestIntervalMaintenance: Pflege prüft die Einstellungen; inaktive Intervalle
// vergeben nichts; Objekte ohne Buchungskreis nutzen *.
func TestIntervalMaintenance(t *testing.T) {
	p := setup(t)
	defineContract(t, p)
	must(t, p, api.Object, api.ActionDefine, api.Definition{Object: "Settlement", Pattern: "AB{N}", Width: 6})
	if res := next(t, p, api.Request{Object: "Settlement", CompanyCode: "1000"}); res.Number != "AB000001" || res.Interval != "Settlement|*|01|0" {
		t.Fatalf("ohne Buchungskreis/Jahr: %+v", res)
	}
	create := func(data map[string]any) error {
		_, err := do(p, api.Object, "create", map[string]any{"data": data})
		return err
	}
	base := func(kv ...any) map[string]any {
		d := map[string]any{"object": "Contract", "company_code": "1000", "range_key": "GW", "year": 2026, "from_number": 1, "to_number": 999,
			"width": 3, "pattern": "{KEY}{YY}{N}", "overflow": api.OverflowError}
		for i := 0; i < len(kv); i += 2 {
			d[kv[i].(string)] = kv[i+1]
		}
		return d
	}
	for name, d := range map[string]map[string]any{
		"Format ohne {N}":          base("pattern", "{KEY}"),
		"bis länger als Stellen":   base("to_number", 5000),
		"bis vor von":              base("from_number", 10, "to_number", 5),
		"ohne Jahr":                base("year", 0),
		"Settlement je Buchungskr": base("object", "Settlement", "year", 0, "pattern", "{N}"),
		"unbekanntes Objekt":       base("object", "Foo"),
	} {
		if err := create(d); !errors.Is(err, sdk.ErrInvalidArgument) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := create(base()); err != nil {
		t.Fatal(err)
	}
	expect(t, create(base()), sdk.ErrAlreadyExists, "doppelt")
	if res := next(t, p, api.Request{Object: "Contract", CompanyCode: "1000", Key: "GW", Year: 2026}); res.Number != "GW26001" {
		t.Fatalf("gepflegtes Intervall: %v", res.Number)
	}
	must(t, p, api.Object, "deactivate", map[string]any{"id": "Contract|1000|GW|2026"})
	_, err := do(p, api.Object, api.ActionNext, api.Request{Object: "Contract", CompanyCode: "1000", Key: "GW", Year: 2026})
	expect(t, err, sdk.ErrFailedPrecondition, "inaktiv")
	// Intervall für alle Buchungskreise greift, wenn es keins für den Buchungskreis gibt.
	if err := create(base("company_code", "*", "range_key", "XX")); err != nil {
		t.Fatal(err)
	}
	if res := next(t, p, api.Request{Object: "Contract", CompanyCode: "3000", Key: "XX", Year: 2026}); res.Interval != "Contract|*|XX|2026" {
		t.Fatalf("Rückfall auf *: %+v", res)
	}
}

// TestConcurrentNext: gleichzeitige Abrufe vergeben keine Nummer doppelt.
func TestConcurrentNext(t *testing.T) {
	p := setup(t)
	defineContract(t, p)
	r := api.Request{Object: "Contract", CompanyCode: "1000", Key: "MV", Year: 2026}
	next(t, p, r) // Intervall anlegen
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := do(p, api.Object, api.ActionNext, r)
			if err != nil {
				t.Error(err)
				return
			}
			n := resp.Payload.(api.Result).Number
			mu.Lock()
			defer mu.Unlock()
			if seen[n] {
				t.Errorf("doppelt: %s", n)
			}
			seen[n] = true
		}()
	}
	wg.Wait()
	if len(seen) != 20 {
		t.Fatalf("vergeben: %d", len(seen))
	}
}

func expect(t *testing.T, err, target error, what string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: erwartet %v, bekommen %v", what, target, err)
	}
}

// --- Übersetzungen ---------------------------------------------------------------------

func texts(d metamodel.DescribeResponse) map[string]string {
	out := map[string]string{}
	for _, o := range d.Objects {
		out[o.TitleKey] = o.Title
		for _, f := range o.Fields {
			out[f.LabelKey] = f.Label
			if f.GroupKey != "" {
				out[f.GroupKey] = f.Group
			}
			for _, opt := range f.Options {
				out[opt.LabelKey] = opt.Label
			}
		}
		for _, s := range o.Sections {
			out[s.TitleKey] = s.Title
		}
		for _, a := range o.Actions {
			if a.ConfirmKey != "" {
				out[a.ConfirmKey] = a.Confirm
			}
		}
	}
	for _, m := range d.Modules {
		out[m.TitleKey], out[m.DescriptionKey] = m.Title, m.Description
		for _, o := range m.Objects {
			if o.SectionKey != "" {
				out[o.SectionKey] = o.Section
			}
		}
	}
	delete(out, "")
	return out
}

func describe(t *testing.T) metamodel.DescribeResponse {
	p := setup(t)
	return must(t, p, sdk.ObjectCatalog, sdk.ActionDescribe, nil).Payload.(metamodel.DescribeResponse)
}

// TestDumpGermanTexts schreibt i18n/de.json aus dem Metamodell (NR_DUMP_I18N=1).
func TestDumpGermanTexts(t *testing.T) {
	if os.Getenv("NR_DUMP_I18N") == "" {
		t.Skip("nur mit NR_DUMP_I18N=1")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(texts(describe(t))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("i18n/de.json", buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMetamodelTranslations(t *testing.T) {
	d := describe(t)
	for _, o := range d.Objects {
		if err := o.Validate(); err != nil {
			t.Error(err)
		}
	}
	keys := texts(d)
	for _, loc := range metamodel.Locales {
		for k := range keys {
			if d.Translations[loc][k] == "" {
				t.Errorf("%s: %s fehlt", loc, k)
			}
		}
		for k := range d.Translations[loc] {
			if _, ok := keys[k]; !ok {
				t.Errorf("%s: verwaister Schlüssel %s", loc, k)
			}
		}
	}
	_ = fmt.Sprint
}
