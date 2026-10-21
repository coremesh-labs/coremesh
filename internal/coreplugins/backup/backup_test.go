package backup

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/coremesh-labs/coremesh/internal/config"
	"github.com/coremesh-labs/coremesh/internal/coreplugins/dbschema"
	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

func setup(t *testing.T, settings map[string]any) *Plugin {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, map[string]config.Database{
		"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "main.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"},
	}, database.Options{TxTimeout: time.Minute}, slog.New(slog.DiscardHandler))
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
	if err := p.Configure(ctx, sdk.Config{Settings: settings}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p
}

var testCtx = sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r1", UserID: "u1"})

func must(t *testing.T, p *Plugin, object, action string, payload any) sdk.Response {
	t.Helper()
	resp, err := p.Handle(testCtx, sdk.Request{Object: object, Action: action, Payload: payload})
	if err != nil {
		t.Fatalf("%s.%s: %v", object, action, err)
	}
	return resp
}

func items(r sdk.Response) []any { return r.Payload.(map[string]any)["items"].([]any) }

// TestSnapshotSQLite: Vorgaben aus den Settings, Jetzt sichern, Kopie lesbar, Anzahl behalten.
func TestSnapshotSQLite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "snaps")
	p := setup(t, map[string]any{"directory": dir, "keep": 2, "interval_hours": 6})
	rec := must(t, p, "BackupSetting", "get", map[string]any{"id": "main"}).Payload.(map[string]any)
	if rec["directory"] != dir || rec["interval_hours"] != 6 || rec["keep_count"] != 2 || rec["active"] != true || rec["database"] != "SQLite" {
		t.Fatalf("Vorgaben: %v", rec)
	}
	clock := time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return clock }
	for i := 0; i < 3; i++ {
		must(t, p, "BackupSetting", "run", map[string]any{"id": "main"})
		clock = clock.Add(time.Hour)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "coremesh-*.db"))
	if len(files) != 2 || !strings.HasSuffix(files[0], "coremesh-20261022-090000.db") {
		t.Fatalf("Dateien: %v", files)
	}
	snap, err := sql.Open("sqlite", "file:"+files[1]+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	var n int
	if err := snap.QueryRow(`SELECT COUNT(*) FROM backup__setting`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("Kopie: %d %v", n, err)
	}
	log := items(must(t, p, "BackupSnapshot", "list", nil))
	if len(log) != 3 || log[2].(map[string]any)["status"] != statusGone || log[0].(map[string]any)["user_id"] != "u1" ||
		log[0].(map[string]any)["trigger_kind"] != trigManual {
		t.Fatalf("Protokoll: %v", log)
	}
	// fällig: letzter Lauf 10:00 + 6 h
	st, _ := p.loadSetting(testCtx)
	if got := p.nextRun(st).Format("15:04"); got != "16:00" {
		t.Fatalf("nächster Lauf: %s", got)
	}
}

func TestUpdateSetting(t *testing.T) {
	p := setup(t, map[string]any{"directory": t.TempDir()})
	dir := filepath.Join(t.TempDir(), "neu", "snaps")
	rec := must(t, p, "BackupSetting", "update", map[string]any{"id": "main", "data": map[string]any{
		"directory": dir, "interval_hours": "12", "keep_count": "0", "active": false}}).Payload.(map[string]any)
	if rec["directory"] != dir || rec["interval_hours"] != 12 || rec["keep_count"] != 0 || rec["active"] != false || rec["next_run_at"] != nil {
		t.Fatalf("geändert: %v", rec)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatal("Verzeichnis nicht angelegt")
	}
	if _, err := p.Handle(testCtx, sdk.Request{Object: "BackupSetting", Action: "update",
		Payload: map[string]any{"id": "main", "data": map[string]any{"interval_hours": 0}}}); err == nil {
		t.Fatal("Abstand 0 angenommen")
	}
}

// TestPgConn: Passwort nie auf der Kommandozeile.
func TestPgConn(t *testing.T) {
	dsn, env, err := pgConn("postgres://cm:geheim@db:5432/coremesh?sslmode=disable")
	if err != nil || strings.Contains(dsn, "geheim") || len(env) != 1 || env[0] != "PGPASSWORD=geheim" || dsn != "postgres://cm@db:5432/coremesh?sslmode=disable" {
		t.Fatalf("%s %v %v", dsn, env, err)
	}
	dsn, env, _ = pgConn("host=db user=cm password='x y' dbname=coremesh")
	if strings.Contains(dsn, "password") || len(env) != 1 {
		t.Fatalf("%s %v", dsn, env)
	}
}

// TestPgSnapshot: pg_dump mit Verbindung ohne Passwort, Datei erst am Ende.
func TestPgSnapshot(t *testing.T) {
	p := setup(t, map[string]any{"pg_dump": "/usr/bin/pg_dump"})
	var gotArgs, gotEnv []string
	p.runCmd = func(_ context.Context, name string, args, env []string) ([]byte, error) {
		gotArgs, gotEnv = append([]string{name}, args...), env
		for _, a := range args {
			if f, ok := strings.CutPrefix(a, "--file="); ok {
				return nil, os.WriteFile(f, []byte("dump"), 0o600)
			}
		}
		return nil, nil
	}
	path := filepath.Join(t.TempDir(), "x.dump")
	p.db = nil // nicht gebraucht: DSN direkt
	if err := p.pgDump(context.Background(), "postgres://cm:pw@db/coremesh", path); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "dump" || gotArgs[0] != "/usr/bin/pg_dump" || strings.Contains(strings.Join(gotArgs, " "), "pw@") ||
		len(gotEnv) != 1 {
		t.Fatalf("%v %v", gotArgs, gotEnv)
	}
}

func texts(d metamodel.DescribeResponse) map[string]string {
	out := map[string]string{}
	for _, o := range d.Objects {
		out[o.TitleKey] = o.Title
		for _, f := range o.Fields {
			out[f.LabelKey] = f.Label
			if f.ShortKey != "" {
				out[f.ShortKey] = f.Short
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
	p := setup(t, nil)
	return must(t, p, sdk.ObjectCatalog, sdk.ActionDescribe, nil).Payload.(metamodel.DescribeResponse)
}

// TestDumpGermanTexts schreibt i18n/de.json aus dem Metamodell (BK_DUMP_I18N=1).
func TestDumpGermanTexts(t *testing.T) {
	if os.Getenv("BK_DUMP_I18N") == "" {
		t.Skip("nur mit BK_DUMP_I18N=1")
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
	}
}

// TestChanged: Snapshot nur nach Arbeit – Sitzungen und eigenes Protokoll zählen nicht.
func TestChanged(t *testing.T) {
	p := setup(t, map[string]any{"directory": t.TempDir(), "ignore_tables": "backup__*, scratch"})
	if _, err := p.pool().Exec(`CREATE TABLE work (v TEXT); CREATE TABLE scratch (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	must(t, p, "BackupSetting", "run", map[string]any{"id": "main"})
	st, _ := p.loadSetting(testCtx)
	if st.ChangeMark == "" || p.changedSince(testCtx, st) {
		t.Fatalf("direkt nach dem Snapshot geändert: %q", st.ChangeMark)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := p.pool().Exec(`INSERT INTO scratch VALUES ('x')`); err != nil {
		t.Fatal(err)
	}
	if p.changedSince(testCtx, st) {
		t.Fatal("ignorierte Tabelle zählt als Änderung")
	}
	st, _ = p.loadSetting(testCtx)
	if _, err := p.pool().Exec(`INSERT INTO work VALUES ('x')`); err != nil {
		t.Fatal(err)
	}
	if !p.changedSince(testCtx, st) {
		t.Fatal("Änderung nicht erkannt")
	}
}

// TestPruneDays: Verweildauer und Höchstanzahl, der jüngste bleibt.
func TestPruneDays(t *testing.T) {
	p := setup(t, map[string]any{"directory": t.TempDir(), "keep": 3, "keep_days": 5})
	clock := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return clock }
	for _, d := range []int{0, 1, 7, 8, 9} { // 1., 2., 8., 9., 10. Oktober
		clock = time.Date(2026, 10, 1+d, 8, 0, 0, 0, time.UTC)
		must(t, p, "BackupSetting", "run", map[string]any{"id": "main"})
	}
	var ok []string
	for _, r := range items(must(t, p, "BackupSnapshot", "list", nil)) {
		if m := r.(map[string]any); m["status"] == statusOK {
			ok = append(ok, m["file_name"].(string))
		}
	}
	if strings.Join(ok, ",") != "coremesh-20261010-080000.db,coremesh-20261009-080000.db,coremesh-20261008-080000.db" {
		t.Fatalf("übrig: %v", ok)
	}
	// 30 Tage nichts gearbeitet: der jüngste bleibt trotz Verweildauer
	clock = clock.AddDate(0, 0, 30)
	st, _ := p.loadSetting(testCtx)
	p.prune(testCtx, st)
	n := 0
	for _, r := range items(must(t, p, "BackupSnapshot", "list", nil)) {
		if r.(map[string]any)["status"] == statusOK {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("nach 30 Tagen: %d", n)
	}
}
