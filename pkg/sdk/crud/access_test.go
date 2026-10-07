package crud

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// sqlDB: module.DB über eine SQLite-Verbindung (ohne Transaktionen).
type sqlDB struct{ db *sql.DB }

func (d sqlDB) Name() string { return "main" }
func (d sqlDB) Query(ctx context.Context, q string, args ...any) (*sdk.QueryResult, error) {
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := &sdk.QueryResult{Columns: cols}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, vals)
	}
	return out, rows.Err()
}
func (d sqlDB) Exec(ctx context.Context, q string, args ...any) (sdk.ExecResult, error) {
	res, err := d.db.ExecContext(ctx, q, args...)
	if err != nil {
		return sdk.ExecResult{}, err
	}
	n, _ := res.RowsAffected()
	return sdk.ExecResult{RowsAffected: n}, nil
}
func (d sqlDB) InTx(ctx context.Context, _ *sql.TxOptions, fn func(context.Context) error) error {
	return fn(ctx)
}

// grantHost spielt iam: Account.Granted liefert die Regeln je Action.
type grantHost struct {
	sdk.Host
	rules map[string][]sdk.GrantRule
}

func (h grantHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	if req.Object+"."+req.Action != "Account.Granted" {
		return sdk.Response{}, sdk.ErrUnimplemented
	}
	action := req.Payload.(map[string]any)["action"].(string)
	return sdk.Response{Payload: sdk.GrantSet{Rules: h.rules[action]}}, nil
}

func cc(codes ...string) []string { return codes }

func contracts(t *testing.T) (*Entity, context.Context, context.Context) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE contract (id TEXT PRIMARY KEY, company_code TEXT, property TEXT, iban TEXT, rent TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range [][]any{{"c1", "1000", "P1", "DE01", "800"}, {"c2", "2000", "P1", "DE02", "900"}, {"c3", "2000", "P2", "DE03", "700"}} {
		db.Exec(`INSERT INTO contract VALUES (?, ?, ?, ?, ?)`, r...)
	}
	e := &Entity{Object: "Contract", Title: "Mietvertrag", Table: "contract", Keys: []string{"id"}, Order: "id",
		Fields: []Field{
			{Key: "id", Label: "ID", Type: metamodel.TypeText, Required: true},
			{Key: "company_code", Label: "Buchungskreis", Type: metamodel.TypeText},
			{Key: "property", Label: "Objekt", Type: metamodel.TypeText},
			{Key: "iban", Label: "IBAN", Type: metamodel.TypeText},
			{Key: "rent", Label: "Miete", Type: metamodel.TypeText},
		},
		Access: &Access{Records: true, CompanyCode: "company_code", Fields: []string{"property"},
			FieldGroups: []metamodel.FieldGroup{{Key: "bank", Label: "Bankverbindung", Fields: []string{"iban"}}}},
	}
	NewSet(e).Bind(sqlDB{db})
	// Verwalter: alles in 1000, in 2000 nur Objekt P1; Bankverbindung sehen nur in 1000, ändern nie.
	h := grantHost{rules: map[string][]sdk.GrantRule{
		metamodel.ActionRead: {
			{CompanyCodes: cc("1000")},
			{CompanyCodes: cc("2000"), Fields: map[string][]sdk.ValueRange{"property": {{Low: "P1"}}}},
		},
		metamodel.ActionReadFields: {{CompanyCodes: cc("1000"), Fields: map[string][]sdk.ValueRange{"field_group": {{Low: "bank"}}}}},
	}}
	user := sdk.WithCall(sdk.WithHost(context.Background(), h), sdk.CallContext{UserID: "u1"})
	system := sdk.WithHost(context.Background(), h)
	return e, user, system
}

func ids(t *testing.T, resp sdk.Response) []string {
	t.Helper()
	var out []string
	for _, it := range resp.Payload.(map[string]any)["items"].([]any) {
		out = append(out, it.(Record)["id"].(string))
	}
	return out
}

func TestRecordAccess(t *testing.T) {
	e, user, system := contracts(t)

	// Liste: nur abgedeckte Datensätze – in der Datenbank gefiltert.
	resp, err := e.List(user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(t, resp); !slices.Equal(got, []string{"c1", "c2"}) {
		t.Fatalf("Liste: %v", got)
	}
	if resp, _ := e.List(system, nil); len(ids(t, resp)) != 3 {
		t.Fatal("System sieht alles")
	}
	// Detail, Ändern: nicht abgedeckt = nicht vorhanden.
	if _, err := e.Get(user, map[string]any{"id": "c3"}); !errors.Is(err, sdk.ErrNotFound) {
		t.Fatalf("Get c3: %v", err)
	}
	if _, err := e.Update(user, map[string]any{"id": "c3", "data": map[string]any{"rent": "1"}}); !errors.Is(err, sdk.ErrNotFound) {
		t.Fatalf("Update c3: %v", err)
	}
	// Nicht aus dem eigenen Bereich hinaus verschieben, nicht außerhalb anlegen.
	if _, err := e.Update(user, map[string]any{"id": "c2", "data": map[string]any{"property": "P2"}}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("verschieben: %v", err)
	}
	if _, err := e.Create(user, map[string]any{"data": map[string]any{"id": "c4", "company_code": "2000", "property": "P2"}}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("anlegen außerhalb: %v", err)
	}
	if _, err := e.Create(user, map[string]any{"data": map[string]any{"id": "c4", "company_code": "1000", "property": "P9"}}); err != nil {
		t.Fatalf("anlegen in 1000: %v", err)
	}
}

func TestFieldGroupAccess(t *testing.T) {
	e, user, system := contracts(t)
	get := func(ctx context.Context, id string) Record {
		t.Helper()
		resp, err := e.Get(ctx, map[string]any{"id": id})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Payload.(Record)
	}
	// 1000: sichtbar, aber nicht änderbar.
	c1 := get(user, "c1")
	if c1["iban"] != "DE01" || !slices.Equal(c1[readonlyFieldsKey].([]any), []any{"iban"}) || c1[hiddenFieldsKey] != nil {
		t.Fatalf("c1: %v", c1)
	}
	// 2000: ausgeblendet – das Feld fehlt in der Antwort.
	c2 := get(user, "c2")
	if _, ok := c2["iban"]; ok || !slices.Equal(c2[hiddenFieldsKey].([]any), []any{"iban"}) {
		t.Fatalf("c2: %v", c2)
	}
	// Auch in der Liste.
	resp, _ := e.List(user, nil)
	for _, it := range resp.Payload.(map[string]any)["items"].([]any) {
		if r := it.(Record); r["id"] == "c2" && r["iban"] != nil {
			t.Fatal("Liste zeigt IBAN in 2000")
		}
	}
	// Ändern der Gruppe abgelehnt, anderes Feld erlaubt, unveränderter Wert erlaubt.
	if _, err := e.Update(user, map[string]any{"id": "c1", "data": map[string]any{"iban": "XX99"}}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("IBAN ändern: %v", err)
	}
	if _, err := e.Update(user, map[string]any{"id": "c1", "data": map[string]any{"iban": "DE01", "rent": "850"}}); err != nil {
		t.Fatalf("Miete ändern: %v", err)
	}
	// Fehlt das Feld im Formular (ausgeblendet), bleibt der Wert erhalten.
	if _, err := e.Update(user, map[string]any{"id": "c2", "data": map[string]any{"rent": "950"}}); err != nil {
		t.Fatal(err)
	}
	if got := get(system, "c2"); got["iban"] != "DE02" || got["rent"] != "950" {
		t.Fatalf("Werte nach Update: %v", got)
	}
	if _, err := e.Create(user, map[string]any{"data": map[string]any{"id": "c5", "company_code": "1000", "iban": "DE05"}}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("anlegen mit IBAN: %v", err)
	}
	// System: keine Marker.
	if s := get(system, "c2"); s[hiddenFieldsKey] != nil || s[readonlyFieldsKey] != nil {
		t.Fatalf("System: %v", s)
	}
}

func TestAccessDefinition(t *testing.T) {
	e, _, _ := contracts(t)
	d := e.Definition()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	az := d.Authorization
	var actions []string
	for _, a := range az.Actions {
		actions = append(actions, a.Name)
	}
	if !slices.Equal(az.Fields, []string{"property", "field_group"}) || !slices.Equal(actions, []string{"read", "readFields", "changeFields"}) ||
		len(az.FieldGroups) != 1 {
		t.Fatalf("Authorization: %+v", az)
	}
}
