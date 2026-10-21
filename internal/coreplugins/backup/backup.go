// Package backup ist das Core-Plugin "backup": Snapshots der Datenbank in
// einstellbarem Abstand.
//
// Object BackupSetting (ein Datensatz "main"): Verzeichnis, Abstand in Stunden,
// nur bei Änderungen (ein Snapshot je Arbeitstag), Verweildauer, Höchstanzahl, aktiv; list, get, update, run („Jetzt sichern“). Die
// Vorgaben beim ersten Start kommen aus den Settings (configs/02-backup-plugin.yaml),
// danach gilt, was in der Oberfläche steht.
//
// Object BackupSnapshot: list, get – Protokoll der Snapshots (Datei, Größe,
// Dauer, Auslöser, Ergebnis; Snapshots über Verweildauer oder Höchstanzahl hinaus
// werden gelöscht, der jüngste bleibt immer).
//
// SQLite: VACUUM INTO – eine konsistente Kopie im laufenden Betrieb (eine
// Lesetransaktion, Schreiber warten nicht). PostgreSQL: pg_dump im Custom-Format
// (wiederherstellen mit pg_restore); das Passwort geht als PGPASSWORD an pg_dump,
// nicht auf die Kommandozeile.
package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

const (
	Name    = "backup"
	Version = "0.1.1"

	settingID   = "main"
	tick        = time.Minute
	startDelay  = 2 * time.Minute // nach dem Start: erst Schema und Plugins
	runTimeout  = 2 * time.Hour
	statusOK    = "OK"
	statusError = "FAILED"
	statusGone  = "REMOVED"
	trigAuto    = "AUTO"
	trigManual  = "MANUAL"
)

type settings struct {
	Database string `json:"database"`
	// Vorgaben für den Datensatz BackupSetting beim ersten Start
	Directory     string `json:"directory"`
	IntervalHours int    `json:"interval_hours"`
	Keep          int    `json:"keep"`
	KeepDays      int    `json:"keep_days"`
	OnlyChanges   *bool  `json:"only_if_changed"`
	IgnoreTables  string `json:"ignore_tables"`
	Active        *bool  `json:"active"`
	// PostgreSQL: Programm pg_dump (leer = pg_dump im Suchpfad)
	PgDump string `json:"pg_dump"`
}

type Plugin struct {
	db       *database.Manager
	host     sdk.Host
	settings settings

	mu     sync.Mutex // ein Snapshot zur Zeit
	stop   chan struct{}
	done   chan struct{}
	now    func() time.Time
	runCmd func(ctx context.Context, name string, args, env []string) ([]byte, error)
}

var (
	_ sdk.Plugin     = (*Plugin)(nil)
	_ sdk.Shutdowner = (*Plugin)(nil)
)

func New(db *database.Manager) *Plugin {
	return &Plugin{db: db, now: time.Now, runCmd: func(ctx context.Context, name string, args, env []string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = append(os.Environ(), env...)
		return cmd.CombinedOutput()
	}}
}

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name: Name, Version: Version, Description: "Datensicherung: Snapshots der Datenbank in einstellbarem Abstand",
		Capabilities: []sdk.Capability{
			{Object: "BackupSetting", Actions: []string{"list", "get", "update", "run"}, Description: "Einstellungen der Sicherung, jetzt sichern"},
			{Object: "BackupSnapshot", Actions: []string{"list", "get"}, Description: "Protokoll der Snapshots"},
			{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
			{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}},
		},
	}, nil
}

func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	s := settings{Database: "main", Directory: "data/snapshots", IntervalHours: 24, Keep: 14, KeepDays: 30,
		IgnoreTables: "backup__*, webserver__sessions"}
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	if _, err := p.db.DB(s.Database); err != nil {
		return err
	}
	p.settings, p.host = s, cfg.Host
	if p.stop == nil {
		p.stop, p.done = make(chan struct{}), make(chan struct{})
		go p.loop()
	}
	return nil
}

// Shutdown beendet den Zeitgeber (ein laufender Snapshot läuft zu Ende).
func (p *Plugin) Shutdown(ctx context.Context) error {
	if p.stop == nil {
		return nil
	}
	close(p.stop)
	select {
	case <-p.done:
	case <-ctx.Done():
	}
	p.stop = nil
	return nil
}

func (p *Plugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch req.Object + "." + req.Action {
	case sdk.ObjectDBSchema + "." + sdk.ActionInit:
		var in sdk.SchemaInitRequest
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: sdk.SchemaInitResponse{Module: in.Module, Schema: schemaHCL}}, nil
	case sdk.ObjectCatalog + "." + sdk.ActionDescribe:
		return sdk.Response{Payload: metamodel.DescribeResponse{
			Objects:      []metamodel.ObjectDefinition{metamodel.WithKeys("backup", settingDef), metamodel.WithKeys("backup", snapshotDef)},
			Modules:      []metamodel.ModuleDefinition{metamodel.ModuleKeys(backupModule)},
			Translations: translations,
		}}, nil
	case "BackupSetting.list":
		rec, err := p.settingRecord(ctx)
		if err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: map[string]any{"items": []any{rec}}}, nil
	case "BackupSetting.get":
		rec, err := p.settingRecord(ctx)
		return sdk.Response{Payload: rec}, err
	case "BackupSetting.update":
		return p.update(ctx, req.Payload)
	case "BackupSetting.run":
		snap, err := p.snapshot(ctx, trigManual, sdk.CallFromContext(ctx).UserID)
		if err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: map[string]any{"id": settingID,
			"message": fmt.Sprintf("Snapshot %s gesichert (%s)", snap.File, sizeText(snap.Size))}}, nil
	case "BackupSnapshot.list":
		return p.snapshotList(ctx, req.Payload)
	case "BackupSnapshot.get":
		return p.snapshotGet(ctx, req.Payload)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

// --- Einstellungen ------------------------------------------------------------

type setting struct {
	Directory     string
	IntervalHours int
	Keep          int // höchstens so viele Snapshots (0 = beliebig viele)
	KeepDays      int // Verweildauer in Tagen (0 = unbegrenzt)
	OnlyChanges   bool
	IgnoreTables  string // Tabellen ohne Bedeutung für „geändert“ (Muster mit *, Komma-Liste)
	Active        bool
	LastRunAt     string
	ChangeMark    string
}

func (p *Plugin) pool() *sql.DB {
	db, _ := p.db.DB(p.settings.Database)
	return db
}

func (p *Plugin) q(query string) string {
	return database.Rebind(p.db.Driver(p.settings.Database), query)
}

// loadSetting: Datensatz main, beim ersten Mal aus den Settings angelegt.
func (p *Plugin) loadSetting(ctx context.Context) (setting, error) {
	var st setting
	var active, only int
	var last, mark sql.NullString
	err := p.pool().QueryRowContext(ctx, p.q(`SELECT directory, interval_hours, keep_count, keep_days, only_if_changed, ignore_tables, active, last_run_at, change_mark
		FROM backup__setting WHERE id = ?`), settingID).
		Scan(&st.Directory, &st.IntervalHours, &st.Keep, &st.KeepDays, &only, &st.IgnoreTables, &active, &last, &mark)
	if errors.Is(err, sql.ErrNoRows) {
		st = setting{Directory: p.settings.Directory, IntervalHours: p.settings.IntervalHours, Keep: p.settings.Keep, KeepDays: p.settings.KeepDays,
			OnlyChanges: p.settings.OnlyChanges == nil || *p.settings.OnlyChanges, IgnoreTables: p.settings.IgnoreTables, Active: p.settings.Active == nil || *p.settings.Active}
		_, err = p.pool().ExecContext(ctx, p.q(`INSERT INTO backup__setting (id, directory, interval_hours, keep_count, keep_days, only_if_changed, ignore_tables, active, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`), settingID, st.Directory, st.IntervalHours, st.Keep, st.KeepDays, boolInt(st.OnlyChanges),
			st.IgnoreTables, boolInt(st.Active), ts(p.now()))
		return st, err
	}
	st.OnlyChanges, st.Active, st.LastRunAt, st.ChangeMark = only == 1, active == 1, last.String, mark.String
	return st, err
}

func (p *Plugin) settingRecord(ctx context.Context) (map[string]any, error) {
	st, err := p.loadSetting(ctx)
	if err != nil {
		return nil, err
	}
	abs, _ := filepath.Abs(st.Directory)
	rec := map[string]any{"id": settingID, "directory": st.Directory, "directory_abs": abs, "interval_hours": st.IntervalHours,
		"keep_count": st.Keep, "keep_days": st.KeepDays, "only_if_changed": st.OnlyChanges, "ignore_tables": st.IgnoreTables, "active": st.Active, "last_run_at": localTime(st.LastRunAt), "database": p.dbKind()}
	if st.Active {
		rec["next_run_at"] = localTime(ts(p.nextRun(st)))
	}
	var file, status, msg sql.NullString
	if err := p.pool().QueryRowContext(ctx, `SELECT file_name, status, message FROM backup__snapshot ORDER BY created_at DESC LIMIT 1`).
		Scan(&file, &status, &msg); err == nil {
		rec["last_file"], rec["last_status"], rec["last_message"] = file.String, status.String, msg.String
	}
	return rec, nil
}

func (p *Plugin) update(ctx context.Context, payload any) (sdk.Response, error) {
	var in struct {
		Data map[string]any `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	st, err := p.loadSetting(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	d := in.Data
	if v, ok := d["directory"]; ok {
		st.Directory = strings.TrimSpace(fmt.Sprint(v))
	}
	if v, ok := d["interval_hours"]; ok {
		if st.IntervalHours, err = intOf(v); err != nil || st.IntervalHours < 1 {
			return sdk.Response{}, fmt.Errorf("%w: Abstand: ganze Zahl ab 1 (Stunden)", sdk.ErrInvalidArgument)
		}
	}
	if v, ok := d["keep_count"]; ok {
		if st.Keep, err = intOf(v); err != nil || st.Keep < 0 {
			return sdk.Response{}, fmt.Errorf("%w: Anzahl behalten: ganze Zahl ab 0", sdk.ErrInvalidArgument)
		}
	}
	if v, ok := d["keep_days"]; ok {
		if st.KeepDays, err = intOf(v); err != nil || st.KeepDays < 0 {
			return sdk.Response{}, fmt.Errorf("%w: Verweildauer: ganze Zahl ab 0 (Tage)", sdk.ErrInvalidArgument)
		}
	}
	if v, ok := d["only_if_changed"]; ok {
		st.OnlyChanges = truthy(v)
	}
	if v, ok := d["ignore_tables"]; ok {
		st.IgnoreTables = ""
		if v != nil {
			st.IgnoreTables = strings.TrimSpace(fmt.Sprint(v))
		}
	}
	if v, ok := d["active"]; ok {
		st.Active = truthy(v)
	}
	if st.Directory == "" {
		return sdk.Response{}, fmt.Errorf("%w: Verzeichnis ist Pflicht", sdk.ErrInvalidArgument)
	}
	if err := os.MkdirAll(st.Directory, 0o750); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Verzeichnis %s nicht anlegbar: %v", sdk.ErrInvalidArgument, st.Directory, err)
	}
	if _, err := p.pool().ExecContext(ctx, p.q(`UPDATE backup__setting SET directory = ?, interval_hours = ?, keep_count = ?, keep_days = ?,
		only_if_changed = ?, ignore_tables = ?, active = ?, updated_at = ? WHERE id = ?`), st.Directory, st.IntervalHours, st.Keep, st.KeepDays,
		boolInt(st.OnlyChanges), st.IgnoreTables, boolInt(st.Active), ts(p.now()), settingID); err != nil {
		return sdk.Response{}, err
	}
	rec, err := p.settingRecord(ctx)
	return sdk.Response{Payload: rec}, err
}

// nextRun: letzter Lauf + Abstand (noch keiner: jetzt).
func (p *Plugin) nextRun(st setting) time.Time {
	last, err := time.Parse(time.RFC3339, st.LastRunAt)
	if err != nil {
		return p.now()
	}
	return last.Add(time.Duration(st.IntervalHours) * time.Hour)
}

// --- Zeitgeber ------------------------------------------------------------------

func (p *Plugin) loop() {
	defer close(p.done)
	wait := time.NewTimer(startDelay)
	defer wait.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-wait.C:
		}
		p.due()
		wait.Reset(tick)
	}
}

// due: Snapshot, wenn aktiv und fällig (Fehler stehen im Protokoll und im Log).
func (p *Plugin) due() {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	st, err := p.loadSetting(ctx)
	if err != nil || !st.Active || p.now().Before(p.nextRun(st)) {
		return
	}
	if st.OnlyChanges && st.LastRunAt != "" && !p.changedSince(ctx, st) {
		return // kein Arbeitstag: nichts geändert seit dem letzten Snapshot
	}
	if _, err := p.snapshot(ctx, trigAuto, ""); err != nil {
		p.log(ctx, sdk.LogWarn, "Snapshot fehlgeschlagen", map[string]string{"err": err.Error()})
	}
}

func (p *Plugin) log(ctx context.Context, lvl sdk.LogLevel, msg string, fields map[string]string) {
	if p.host != nil {
		_ = p.host.Log(ctx, lvl, msg, fields)
	}
}

// --- Snapshot ---------------------------------------------------------------------

type snapshotRec struct {
	File string
	Size int64
}

func (p *Plugin) dbKind() string {
	d := p.db.Driver(p.settings.Database)
	switch {
	case strings.HasPrefix(d, "sqlite"):
		return "SQLite"
	case d == "pgx" || d == "postgres":
		return "PostgreSQL"
	}
	return d
}

// snapshot sichert die Datenbank, protokolliert und löscht überzählige Snapshots.
// Auch ein Fehlschlag zählt als Lauf (sonst versuchte es jede Minute erneut).
func (p *Plugin) snapshot(ctx context.Context, trigger, user string) (snapshotRec, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st, err := p.loadSetting(ctx)
	if err != nil {
		return snapshotRec{}, err
	}
	start := p.now()
	stamp := start.Format("20060102-150405")
	var out snapshotRec
	var path string
	err = os.MkdirAll(st.Directory, 0o750)
	if err == nil {
		switch p.dbKind() {
		case "SQLite":
			out.File = "coremesh-" + stamp + ".db"
			path = filepath.Join(st.Directory, out.File)
			err = p.sqliteSnapshot(ctx, path)
		case "PostgreSQL":
			out.File = "coremesh-" + stamp + ".dump"
			path = filepath.Join(st.Directory, out.File)
			err = p.pgSnapshot(ctx, path)
		default:
			err = fmt.Errorf("Datenbank %s: Snapshot nur für SQLite und PostgreSQL", p.dbKind())
		}
	}
	status, msg := statusOK, ""
	if err != nil {
		status, msg = statusError, err.Error()
	} else if fi, serr := os.Stat(path); serr == nil {
		out.Size = fi.Size()
	}
	abs, _ := filepath.Abs(path)
	if _, lerr := p.pool().ExecContext(ctx, p.q(`INSERT INTO backup__snapshot (id, setting_id, file_name, path, created_at, size_bytes, duration_ms,
		trigger_kind, user_id, status, message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		stamp, settingID, out.File, abs, ts(start), out.Size, p.now().Sub(start).Milliseconds(), trigger, nullable(user), status, nullable(msg)); lerr != nil && err == nil {
		err = lerr
	}
	if _, lerr := p.pool().ExecContext(ctx, p.q(`UPDATE backup__setting SET last_run_at = ?, change_mark = ? WHERE id = ?`),
		ts(start), p.changeMark(ctx, st), settingID); lerr != nil && err == nil {
		err = lerr
	}
	if err != nil {
		return out, fmt.Errorf("Snapshot nicht gesichert: %w", err)
	}
	p.log(ctx, sdk.LogInfo, "Snapshot gesichert", map[string]string{"file": abs, "bytes": strconv.FormatInt(out.Size, 10), "trigger": trigger})
	p.prune(ctx, st)
	return out, nil
}

// sqliteSnapshot: VACUUM INTO in eine Zwischendatei, dann umbenennen (nie halbe Snapshots).
func (p *Plugin) sqliteSnapshot(ctx context.Context, path string) error {
	tmp := path + ".partial"
	_ = os.Remove(tmp)
	if _, err := p.pool().ExecContext(ctx, `VACUUM INTO ?`, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// pgSnapshot: pg_dump -Fc; Passwort aus der Verbindungsangabe als PGPASSWORD.
func (p *Plugin) pgSnapshot(ctx context.Context, path string) error {
	return p.pgDump(ctx, p.db.DSN(p.settings.Database), path)
}

func (p *Plugin) pgDump(ctx context.Context, conn, path string) error {
	dsn, env, err := pgConn(conn)
	if err != nil {
		return err
	}
	bin := p.settings.PgDump
	if bin == "" {
		bin = "pg_dump"
	}
	tmp := path + ".partial"
	_ = os.Remove(tmp)
	out, err := p.runCmd(ctx, bin, []string{"--format=custom", "--no-password", "--file=" + tmp, "--dbname=" + dsn}, env)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%s: %v %s", bin, err, strings.TrimSpace(string(out)))
	}
	return os.Rename(tmp, path)
}

// pgConn: Verbindungsangabe ohne Passwort (URL- oder key=value-Form) und PGPASSWORD.
func pgConn(dsn string) (string, []string, error) {
	if dsn == "" {
		return "", nil, errors.New("keine Verbindungsangabe")
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", nil, fmt.Errorf("Verbindungsangabe: %v", err)
		}
		var env []string
		if u.User != nil {
			if pw, ok := u.User.Password(); ok {
				env = append(env, "PGPASSWORD="+pw)
				u.User = url.User(u.User.Username())
			}
		}
		return u.String(), env, nil
	}
	var parts, env []string
	for _, f := range strings.Fields(dsn) {
		if k, v, ok := strings.Cut(f, "="); ok && k == "password" {
			env = append(env, "PGPASSWORD="+strings.Trim(v, "'"))
			continue
		}
		parts = append(parts, f)
	}
	return strings.Join(parts, " "), env, nil
}

// prune: eigene, erfolgreiche Snapshots löschen – älter als die Verweildauer oder über
// die Höchstanzahl hinaus; der jüngste bleibt immer.
func (p *Plugin) prune(ctx context.Context, st setting) {
	if st.Keep <= 0 && st.KeepDays <= 0 {
		return
	}
	rows, err := p.pool().QueryContext(ctx, p.q(`SELECT id, path, created_at FROM backup__snapshot WHERE status = ? ORDER BY created_at DESC`), statusOK)
	if err != nil {
		return
	}
	type old struct{ id, path, why string }
	var olds []old
	limit := p.now().AddDate(0, 0, -st.KeepDays)
	n := 0
	for rows.Next() {
		var id, path, at string
		if rows.Scan(&id, &path, &at) != nil {
			continue
		}
		n++
		created, _ := time.Parse(time.RFC3339, at)
		switch {
		case n == 1:
		case st.Keep > 0 && n > st.Keep:
			olds = append(olds, old{id, path, fmt.Sprintf("mehr als %d Snapshots", st.Keep)})
		case st.KeepDays > 0 && created.Before(limit):
			olds = append(olds, old{id, path, fmt.Sprintf("älter als %d Tage", st.KeepDays)})
		}
	}
	rows.Close()
	for _, o := range olds {
		if err := os.Remove(o.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			p.log(ctx, sdk.LogWarn, "Snapshot nicht gelöscht", map[string]string{"file": o.path, "err": err.Error()})
			continue
		}
		_, _ = p.pool().ExecContext(ctx, p.q(`UPDATE backup__snapshot SET status = ?, message = ? WHERE id = ?`),
			statusGone, "gelöscht am "+p.now().Format("02.01.2006 15:04")+" ("+o.why+")", o.id)
	}
}

// --- Änderungen erkennen ------------------------------------------------------------

// Ob seit dem letzten Snapshot gearbeitet wurde, entscheidet der Inhalt ohne die
// Tabellen aus „ignore_tables“ (eigenes Protokoll, Sitzungen – die ändern sich auch
// ohne Arbeit). change_mark hält den Stand nach dem Snapshot:
//   SQLite:     „<Dateizeit>|<Prüfsumme>“ – Prüfsumme über alle übrigen Zeilen; neu
//               gerechnet nur, wenn Datei oder WAL jünger als die Dateizeit sind.
//   PostgreSQL: „<Serverstart>|<Zahl geänderter Zeilen>“ aus pg_stat_user_tables.
// Leer oder nicht ermittelbar = geändert (im Zweifel sichern).

// changeMark: Stand der Datenbank jetzt.
func (p *Plugin) changeMark(ctx context.Context, st setting) string {
	switch p.dbKind() {
	case "SQLite":
		sum, err := p.sqliteChecksum(ctx, st.IgnoreTables)
		if err != nil {
			return ""
		}
		return p.sqliteMtime().UTC().Format(time.RFC3339Nano) + "|" + sum
	case "PostgreSQL":
		var start string
		if err := p.pool().QueryRowContext(ctx, `SELECT pg_postmaster_start_time()::text`).Scan(&start); err != nil {
			return ""
		}
		rows, err := p.pool().QueryContext(ctx, `SELECT relname, n_tup_ins + n_tup_upd + n_tup_del FROM pg_stat_user_tables`)
		if err != nil {
			return ""
		}
		defer rows.Close()
		var n int64
		for rows.Next() {
			var rel string
			var c int64
			if rows.Scan(&rel, &c) == nil && !ignored(rel, st.IgnoreTables) {
				n += c
			}
		}
		return start + "|" + strconv.FormatInt(n, 10)
	}
	return ""
}

// changedSince: Änderung seit dem letzten Snapshot? SQLite: unveränderter Inhalt
// trotz jüngerer Datei (Sitzungen, Protokoll) schiebt nur die Dateizeit der Marke nach.
func (p *Plugin) changedSince(ctx context.Context, st setting) bool {
	if st.ChangeMark == "" {
		return true
	}
	if p.dbKind() == "SQLite" {
		at, sum, _ := strings.Cut(st.ChangeMark, "|")
		mark, err := time.Parse(time.RFC3339Nano, at)
		if err == nil && !p.sqliteMtime().After(mark) {
			return false
		}
		now := p.changeMark(ctx, st)
		if _, nowSum, _ := strings.Cut(now, "|"); now == "" || nowSum != sum {
			return true
		}
		_, _ = p.pool().ExecContext(ctx, p.q(`UPDATE backup__setting SET change_mark = ? WHERE id = ?`), now, settingID)
		return false
	}
	return p.changeMark(ctx, st) != st.ChangeMark
}

// sqliteMtime: jüngste Änderungszeit von Datei und WAL (Nullzeit, wenn unbekannt).
func (p *Plugin) sqliteMtime() time.Time {
	var t time.Time
	path := sqlitePath(p.db.DSN(p.settings.Database))
	if path == "" {
		return t
	}
	for _, f := range []string{path, path + "-wal"} {
		if fi, err := os.Stat(f); err == nil && fi.ModTime().After(t) {
			t = fi.ModTime()
		}
	}
	return t
}

// sqliteChecksum: SHA-256 über Namen und Zeilen aller Tabellen außer den ignorierten.
func (p *Plugin) sqliteChecksum(ctx context.Context, ignore string) (string, error) {
	rows, err := p.pool().QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		return "", err
	}
	var tables []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil && !ignored(t, ignore) {
			tables = append(tables, t)
		}
	}
	rows.Close()
	h := sha256.New()
	for _, t := range tables {
		fmt.Fprintf(h, "\x00%s\x00", t)
		r, err := p.pool().QueryContext(ctx, `SELECT * FROM "`+strings.ReplaceAll(t, `"`, `""`)+`"`)
		if err != nil {
			return "", err
		}
		cols, _ := r.Columns()
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		for r.Next() {
			if err := r.Scan(ptrs...); err != nil {
				r.Close()
				return "", err
			}
			for _, v := range vals {
				fmt.Fprintf(h, "%T:%v\x1f", v, v)
			}
			h.Write([]byte{'\n'})
		}
		err = r.Err()
		r.Close()
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ignored: Tabelle passt auf eines der Muster (Komma-Liste, * und ? als Platzhalter).
func ignored(table, patterns string) bool {
	for _, pat := range strings.Split(patterns, ",") {
		if pat = strings.TrimSpace(pat); pat == "" {
			continue
		}
		if ok, _ := path.Match(strings.ToLower(pat), strings.ToLower(table)); ok {
			return true
		}
	}
	return false
}

// sqlitePath: Datei aus der Verbindungsangabe (file:pfad?…, pfad?…); leer bei :memory:.
func sqlitePath(dsn string) string {
	path, _, _ := strings.Cut(strings.TrimPrefix(dsn, "file:"), "?")
	if path == "" || strings.Contains(path, ":memory:") {
		return ""
	}
	return path
}

// --- Protokoll ----------------------------------------------------------------------

const snapshotCols = `id, file_name, path, created_at, size_bytes, duration_ms, trigger_kind, user_id, status, message`

func (p *Plugin) snapshotList(ctx context.Context, payload any) (sdk.Response, error) {
	q := query(payload)
	where, args := "", []any{}
	if v := q["status"]; v != "" {
		where, args = "WHERE status = ?", append(args, v)
	}
	rows, err := p.pool().QueryContext(ctx, p.q(`SELECT `+snapshotCols+` FROM backup__snapshot `+where+` ORDER BY created_at DESC LIMIT 500`), args...)
	if err != nil {
		return sdk.Response{}, err
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		rec, err := scanSnapshot(rows)
		if err != nil {
			return sdk.Response{}, err
		}
		items = append(items, rec)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, rows.Err()
}

func (p *Plugin) snapshotGet(ctx context.Context, payload any) (sdk.Response, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	rows, err := p.pool().QueryContext(ctx, p.q(`SELECT `+snapshotCols+` FROM backup__snapshot WHERE id = ?`), in.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return sdk.Response{}, fmt.Errorf("%w: Snapshot %q", sdk.ErrNotFound, in.ID)
	}
	rec, err := scanSnapshot(rows)
	return sdk.Response{Payload: rec}, err
}

func scanSnapshot(rows *sql.Rows) (map[string]any, error) {
	var id, file, path, at, trig, status string
	var size, dur int64
	var user, msg sql.NullString
	if err := rows.Scan(&id, &file, &path, &at, &size, &dur, &trig, &user, &status, &msg); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "setting_id": settingID, "file_name": file, "path": path, "created_at": localTime(at), "size_bytes": size,
		"size": sizeText(size), "duration_ms": dur, "trigger_kind": trig, "user_id": user.String, "status": status, "message": msg.String}, nil
}

// --- Hilfen -------------------------------------------------------------------------

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// localTime: gespeicherte Zeit (UTC) zur Anzeige in Ortszeit, z. B. 22.10.2026 08:00.
func localTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("02.01.2006 15:04")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func truthy(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "on" || v == "1"
	case float64:
		return v != 0
	}
	return false
}

func intOf(v any) (int, error) {
	switch v := v.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	}
	return strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
}

func sizeText(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func query(payload any) map[string]string {
	m, _ := payload.(map[string]any)
	if q, ok := m["query"].(map[string]any); ok {
		m = q
	}
	out := map[string]string{}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
