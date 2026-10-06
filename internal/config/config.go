// Package config lädt, verschmilzt und validiert die Host-Konfiguration.
package config

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

// Config ist die zusammengeführte Konfiguration aller Dateien in configs/.
type Config struct {
	Host      Host                `yaml:"host"`
	Databases map[string]Database `yaml:"databases"`
	Resolver  Resolver            `yaml:"resolver"`
	Plugins   map[string]Plugin   `yaml:"plugins"`
}

type Host struct {
	// Zentrales Plugin-Verzeichnis (relativ zum Arbeitsverzeichnis).
	PluginDir          string        `yaml:"plugin_dir"`
	PluginStartTimeout time.Duration `yaml:"plugin_start_timeout"`
	ShutdownTimeout    time.Duration `yaml:"shutdown_timeout"`
	// Maximale Zahl gleichzeitig verschachtelter Plugin-Aufrufe pro Anfrage.
	MaxCallDepth   int           `yaml:"max_call_depth"`
	TxTimeout      time.Duration `yaml:"tx_timeout"`
	MaxTxPerPlugin int           `yaml:"max_tx_per_plugin"`
}

type Database struct {
	Driver          string        `yaml:"driver"`
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type Resolver struct {
	Download Download `yaml:"download"`
}

// Download beschreibt, woher fehlende Plugin-Binaries geladen werden.
// Platzhalter in den Templates: {name} {version} {os} {arch} {binary} {shard}.
type Download struct {
	// Leer = kein Download, nur lokale Suche.
	URLTemplate string `yaml:"url_template"`
	// Optional: Datei mit der SHA-256-Prüfsumme (sha256sum-Format), falls
	// das Plugin keine sha256 in der Konfiguration hat.
	ChecksumURLTemplate string `yaml:"checksum_url_template"`
	// Umgebungsvariable des Hosts mit dem Zugriffstoken (z. B. GitLab).
	TokenEnv    string        `yaml:"token_env"`
	TokenHeader string        `yaml:"token_header"`
	Timeout     time.Duration `yaml:"timeout"`
	MaxBytes    int64         `yaml:"max_bytes"`
	// Nur für Entwicklung: Download ohne Prüfsumme bzw. über HTTP erlauben.
	AllowUnverified   bool `yaml:"allow_unverified"`
	AllowInsecureHTTP bool `yaml:"allow_insecure_http"`
}

const (
	KindExternal = "external"
	KindInternal = "internal"
)

type Plugin struct {
	// external (Default): eigener Prozess via go-plugin; internal: im Host.
	Kind    string `yaml:"kind"`
	Enabled *bool  `yaml:"enabled"`
	// Optional: Startfehler werden geloggt statt den Host abzubrechen.
	Optional bool `yaml:"optional"`
	// Ingress: Das Plugin nimmt Anfragen von außen an (z. B. HTTP) und darf
	// neue Wurzelanfragen starten. Mandant und Benutzer, die es setzt, gelten
	// als vertrauenswürdig – nur für Plugins mit eigener Authentifizierung.
	Ingress bool   `yaml:"ingress"`
	Version string `yaml:"version"`
	// SHA-256 der Binary (hex). Geprüft beim Download und bei jedem Start.
	SHA256    string            `yaml:"sha256"`
	Args      []string          `yaml:"args"`
	Env       map[string]string `yaml:"env"`
	Settings  map[string]any    `yaml:"settings"`
	Databases map[string]Grant  `yaml:"databases"`
}

// Grant ist die Freigabe eines Plugins für eine Datenbank.
type Grant struct {
	Access string `yaml:"access"` // read | write
}

func (p Plugin) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

// Names liefert die Namen aller aktivierten Plugins der Art kind, sortiert.
func (c *Config) Names(kind string) []string {
	var names []string
	for name, p := range c.Plugins {
		if p.Kind == kind && p.IsEnabled() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (c *Config) applyDefaults() {
	h := &c.Host
	setDefault(&h.PluginDir, "plugins")
	setDefault(&h.PluginStartTimeout, 10*time.Second)
	setDefault(&h.ShutdownTimeout, 10*time.Second)
	setDefault(&h.MaxCallDepth, 8)
	setDefault(&h.TxTimeout, 30*time.Second)
	setDefault(&h.MaxTxPerPlugin, 16)

	d := &c.Resolver.Download
	setDefault(&d.TokenHeader, "PRIVATE-TOKEN")
	setDefault(&d.Timeout, 60*time.Second)
	setDefault(&d.MaxBytes, int64(200<<20))

	for name, p := range c.Plugins {
		setDefault(&p.Kind, KindExternal)
		c.Plugins[name] = p
	}
}

func setDefault[T comparable](v *T, def T) {
	var zero T
	if *v == zero {
		*v = def
	}
}

var (
	// Bindestriche nur einzeln: so bleibt sdk.TablePrefix (mit "__") eindeutig.
	pluginNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	versionRe    = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]*$`)
	sha256Re     = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// ValidPluginName meldet, ob name der Konvention [a-z0-9-] folgt: min. 2
// Zeichen, keine doppelten oder randständigen Bindestriche.
func ValidPluginName(name string) bool { return len(name) >= 2 && pluginNameRe.MatchString(name) }

// ValidVersion meldet, ob v als Teil eines Dateinamens zulässig ist.
func ValidVersion(v string) bool { return versionRe.MatchString(v) }

func (c *Config) validate() error {
	var errs []error
	for name, db := range c.Databases {
		if db.Driver == "" || db.DSN == "" {
			errs = append(errs, fmt.Errorf("databases.%s: driver und dsn sind Pflicht", name))
		}
	}
	for name, p := range c.Plugins {
		if !ValidPluginName(name) {
			errs = append(errs, fmt.Errorf("plugins.%s: ungültiger Name (erlaubt: [a-z0-9-], min. 2 Zeichen, keine doppelten Bindestriche)", name))
		}
		if name == "coremesh" {
			errs = append(errs, fmt.Errorf("plugins.%s: Name ist dem Host vorbehalten", name))
		}
		switch p.Kind {
		case KindExternal:
			if !ValidVersion(p.Version) {
				errs = append(errs, fmt.Errorf("plugins.%s: version fehlt oder ist ungültig", name))
			}
		case KindInternal:
		default:
			errs = append(errs, fmt.Errorf("plugins.%s: kind muss external oder internal sein", name))
		}
		if p.SHA256 != "" && !sha256Re.MatchString(p.SHA256) {
			errs = append(errs, fmt.Errorf("plugins.%s: sha256 muss 64 Hex-Zeichen haben", name))
		}
		for db, g := range p.Databases {
			if _, ok := c.Databases[db]; !ok {
				errs = append(errs, fmt.Errorf("plugins.%s.databases.%s: Datenbank nicht konfiguriert", name, db))
			}
			if g.Access != "read" && g.Access != "write" {
				errs = append(errs, fmt.Errorf("plugins.%s.databases.%s: access muss read oder write sein", name, db))
			}
		}
	}
	return errors.Join(errs...)
}
