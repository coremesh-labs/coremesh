// Command webserver ist das generische Web-Frontend von CoreMesh: ein
// externes Plugin, das HTTP-Anfragen annimmt (Ingress), Benutzer anmeldet,
// Anfragen in (object, action)-Aufrufe an die Fachmodule übersetzt und die
// Antworten mit html/template und HTMX darstellt. Arbeitsweise: README.md.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/plugin"
)

const (
	name    = "webserver"
	version = "0.17.0"
)

// settings aus plugins.webserver.settings in der Host-Konfiguration.
type settings struct {
	Listen        string `json:"listen"`         // Standard 127.0.0.1:8080; 0.0.0.0:8080 = alle Schnittstellen
	Title         string `json:"title"`          // Anwendungstitel
	Tenant        string `json:"tenant"`         // Mandant für Benutzer ohne eigenen Mandanten
	DefaultLocale string `json:"default_locale"` // Standardsprache (de, en, zh-CN), wenn nichts anderes greift
	TemplatesDir  string `json:"templates_dir"`  // *.html, die eingebettete Blöcke überschreiben
	StaticDir     string `json:"static_dir"`     // Dateien, die eingebettete /static/-Dateien überschreiben

	// Authentifizierung
	Database     string `json:"database"`      // Datenbank der Benutzertabellen (Standard main)
	SessionTTL   string `json:"session_ttl"`   // Gültigkeit einer Anmeldung (Standard 12h)
	CookieSecure string `json:"cookie_secure"` // auto (bei TLS) | true (z. B. hinter TLS-Proxy) | false

	// TLS: beide gesetzt = HTTPS
	TLSCert string `json:"tls_cert"`
	TLSKey  string `json:"tls_key"`
}

type webserver struct {
	mu     sync.Mutex
	http   *http.Server
	cancel context.CancelFunc
	url    string
	cfg    settings
}

func (w *webserver) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name:        name,
		Version:     version,
		Description: "Generisches Web-Frontend (HTTP-Ingress, HTMX, Anmeldung über iam)",
		Capabilities: []sdk.Capability{
			{Object: "WebServer", Actions: []string{"Status"}, Description: "Status des Web-Frontends"},
			// Reservierte Lebenszyklus-Capability: Session-Tabelle.
			{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
		},
	}, nil
}

// Configure startet den HTTP(S)-Server und – im Hintergrund – das Anlegen des
// ersten Benutzers. Ein belegter Port lässt den Start mit klarer Meldung scheitern.
func (w *webserver) Configure(ctx context.Context, cfg sdk.Config) error {
	s := settings{Listen: "127.0.0.1:8080", Title: "CoreMesh", Database: "main", SessionTTL: "12h", CookieSecure: "auto"}
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	ttl, err := time.ParseDuration(s.SessionTTL)
	if err != nil || ttl <= 0 {
		return fmt.Errorf("%w: session_ttl %q", sdk.ErrInvalidArgument, s.SessionTTL)
	}
	if (s.TLSCert == "") != (s.TLSKey == "") {
		return fmt.Errorf("%w: tls_cert und tls_key nur gemeinsam", sdk.ErrInvalidArgument)
	}
	views, err := newRenderer(s.TemplatesDir)
	if err != nil {
		return fmt.Errorf("%w: Templates: %v", sdk.ErrInvalidArgument, err)
	}

	// Benutzer und Rollen: Core-Plugin iam (Account.*); Sessions: eigene Tabelle.
	auth := newAuthService(&hostIdentity{host: cfg.Host}, &sqlStore{host: cfg.Host, db: s.Database}, ttl)
	ln, err := net.Listen("tcp", s.Listen)
	if err != nil {
		return fmt.Errorf("%w: listen %s: %v", sdk.ErrUnavailable, s.Listen, err)
	}
	srv := &http.Server{
		Handler:           newServer(cfg.Host, views, s, auth),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	scheme := "http"
	if s.TLSCert != "" {
		scheme = "https"
		go srv.ServeTLS(ln, s.TLSCert, s.TLSKey)
	} else {
		go srv.Serve(ln)
	}

	bg, cancel := context.WithCancel(context.Background())
	go cleanupSessions(bg, auth)

	w.mu.Lock()
	oldSrv, oldCancel := w.http, w.cancel
	w.http, w.cancel, w.url, w.cfg = srv, cancel, scheme+"://"+ln.Addr().String(), s
	w.mu.Unlock()
	if oldSrv != nil {
		oldCancel()
		_ = oldSrv.Close()
	}

	_ = cfg.Host.Log(ctx, sdk.LogInfo, "WebServer läuft", map[string]string{"url": w.url})
	if scheme == "http" && !isLoopback(ln.Addr()) {
		_ = cfg.Host.Log(ctx, sdk.LogWarn, "WebServer im Netz ohne TLS – Passwörter gehen unverschlüsselt über die Leitung; tls_cert/tls_key oder einen TLS-Proxy verwenden",
			map[string]string{"listen": s.Listen})
	}
	return nil
}

// cleanupSessions löscht stündlich abgelaufene Sessions.
func cleanupSessions(ctx context.Context, auth *authService) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = auth.store.DeleteExpiredSessions(ctx, time.Now())
		}
	}
}

func (w *webserver) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	switch {
	case req.Object == sdk.ObjectDBSchema && req.Action == sdk.ActionInit:
		var in sdk.SchemaInitRequest
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: sdk.SchemaInitResponse{Module: in.Module, Schema: schemaHCL}}, nil
	case req.Object == "WebServer" && req.Action == "Status":
		w.mu.Lock()
		defer w.mu.Unlock()
		return sdk.Response{Payload: map[string]any{
			"url": w.url, "templates_dir": w.cfg.TemplatesDir, "tenant": w.cfg.Tenant,
		}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func isLoopback(a net.Addr) bool {
	if t, ok := a.(*net.TCPAddr); ok {
		return t.IP.IsLoopback()
	}
	return false
}

func main() {
	plugin.Main(&webserver{})
}
