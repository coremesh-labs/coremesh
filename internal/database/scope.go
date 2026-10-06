package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Schema-Isolation zur Laufzeit (PostgreSQL).
//
// Bei dbschema-Setting isolation: schema liegen die Tabellen eines Moduls in
// seinem eigenen DB-Schema. Damit Modul-SQL mit unqualifizierten Namen dort
// landet, bekommt jedes Modul einen eigenen Pool, dessen Verbindungen mit
// search_path=<schema> geöffnet werden. Ein SET search_path auf einem
// geteilten Pool wäre unsicher, weil Verbindungen wiederverwendet werden.
//
// Der search_path ist Komfort, keine Sicherheitsgrenze: Voll qualifizierte
// Namen (mod_other.tabelle) bleiben über denselben DB-Benutzer erreichbar.
// Eine harte Grenze ziehen erst eigene DB-Rollen pro Modul.

var searchPathRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// IsPostgres meldet, ob der Pool name einen PostgreSQL-Treiber nutzt.
func (m *Manager) IsPostgres(name string) bool {
	d := m.Driver(name)
	return d == "pgx" || d == "postgres"
}

// SetScope legt fest, dass alle Zugriffe von owner auf database mit
// search_path = searchPath laufen.
func (m *Manager) SetScope(database, owner, searchPath string) error {
	p, err := m.pool(database)
	if err != nil {
		return err
	}
	if !m.IsPostgres(database) {
		return fmt.Errorf("search_path pro Plugin braucht PostgreSQL (Datenbank %s: %s)", database, p.driver)
	}
	if !searchPathRe.MatchString(searchPath) {
		return fmt.Errorf("ungültiger search_path %q", searchPath)
	}
	p.mu.Lock()
	p.owners[owner] = searchPath
	p.mu.Unlock()
	return nil
}

// PoolFor liefert den Pool, über den owner auf database zugreift: den
// Modul-Pool mit eigenem search_path, falls gesetzt, sonst den Standard-Pool.
func (m *Manager) PoolFor(database, owner string) (*sql.DB, error) {
	p, err := m.pool(database)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	sp, ok := p.owners[owner]
	if !ok {
		return p.db, nil
	}
	if db, ok := p.scoped[sp]; ok {
		return db, nil
	}
	db, err := sql.Open(p.driver, withSearchPath(p.cfg.DSN, sp))
	if err != nil {
		return nil, err
	}
	if p.cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(p.cfg.MaxOpenConns)
	}
	if p.cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(p.cfg.MaxIdleConns)
	}
	if p.cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(p.cfg.ConnMaxLifetime)
	}
	p.scoped[sp] = db
	return db, nil
}

// withSearchPath ergänzt den search_path als Laufzeitparameter der
// Verbindung – für URL-DSNs (postgres://…) und Key/Value-DSNs (host=… …).
func withSearchPath(dsn, searchPath string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if u, err := url.Parse(dsn); err == nil {
			q := u.Query()
			q.Set("search_path", searchPath)
			u.RawQuery = q.Encode()
			return u.String()
		}
	}
	return dsn + " search_path=" + searchPath
}

func (p *pool) close() []error {
	errs := []error{p.db.Close()}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, db := range p.scoped {
		errs = append(errs, db.Close())
	}
	return errs
}
