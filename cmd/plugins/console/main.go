// Command console ist das externe Plugin für den Kommandozeilen-Zugang zu
// CoreMesh. Es stellt einen gRPC-Server bereit (consolev1.ConsoleService) –
// standardmäßig auf einem Unix-Socket, per Konfiguration auch über TCP (mit
// optionalem TLS). Client ist die CLI cmd/console. Arbeitsweise: README.md.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	consolev1 "github.com/coremesh-lab/coremesh/pkg/consoleapi/console/v1"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/plugin"
)

const (
	name    = "console"
	version = "0.2.0"

	maxMessage = 64 << 20 // max. Größe einer gRPC-Nachricht (CLI <-> Plugin)
)

// settings aus plugins.console.settings in der Host-Konfiguration.
type settings struct {
	// unix://<pfad> (Standard unix://./data/console.sock) oder tcp://<host>:<port>
	Listen  string `json:"listen"`
	TLSCert string `json:"tls_cert"` // nur TCP; beide gesetzt = TLS
	TLSKey  string `json:"tls_key"`
	// Gültigkeit einer Anmeldung (Standard 8h)
	TokenTTL string `json:"token_ttl"`
	// Grenzen beim Entpacken
	MaxExtractBytes int64 `json:"max_extract_bytes"` // Standard 1 GiB
	MaxExtractFiles int   `json:"max_extract_files"` // Standard 10000
	// Zusätzlich gesperrte Zielverzeichnisse (zu den Systemverzeichnissen)
	BlockedDirectories []string `json:"blocked_directories"`

	tokenTTL time.Duration
}

type console struct {
	mu     sync.Mutex
	server *grpc.Server
	addr   string
	socket string // Pfad des Unix-Sockets (zum Aufräumen)
}

func (c *console) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name:        name,
		Version:     version,
		Description: "Kommandozeilen-Zugang (gRPC über Unix-Socket/TCP)",
		Capabilities: []sdk.Capability{
			{Object: "Console", Actions: []string{"Status"}, Description: "Status des Console-Zugangs"},
		},
	}, nil
}

func (c *console) Configure(ctx context.Context, cfg sdk.Config) error {
	s := settings{Listen: "unix://./data/console.sock", TokenTTL: "8h", MaxExtractBytes: 1 << 30, MaxExtractFiles: 10000}
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	ttl, err := time.ParseDuration(s.TokenTTL)
	if err != nil || ttl <= 0 {
		return fmt.Errorf("%w: token_ttl %q", sdk.ErrInvalidArgument, s.TokenTTL)
	}
	s.tokenTTL = ttl

	ln, socket, err := listen(s.Listen)
	if err != nil {
		return fmt.Errorf("%w: listen %s: %v", sdk.ErrUnavailable, s.Listen, err)
	}
	svc := newService(cfg.Host, s)
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(svc.authInterceptor),
		grpc.MaxRecvMsgSize(maxMessage),
		grpc.MaxSendMsgSize(maxMessage),
	}
	secure := false
	if s.TLSCert != "" || s.TLSKey != "" {
		if socket != "" {
			return fmt.Errorf("%w: TLS nur mit tcp://", sdk.ErrInvalidArgument)
		}
		cert, err := tls.LoadX509KeyPair(s.TLSCert, s.TLSKey)
		if err != nil {
			return fmt.Errorf("%w: TLS: %v", sdk.ErrInvalidArgument, err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})))
		secure = true
	}
	srv := grpc.NewServer(opts...)
	consolev1.RegisterConsoleServiceServer(srv, svc)
	go srv.Serve(ln)

	c.mu.Lock()
	old := c.server
	c.server, c.addr, c.socket = srv, s.Listen, socket
	c.mu.Unlock()
	if old != nil {
		old.Stop()
	}

	_ = cfg.Host.Log(ctx, sdk.LogInfo, "Console läuft", map[string]string{"listen": s.Listen})
	if socket == "" && !secure && !isLoopback(ln.Addr()) {
		_ = cfg.Host.Log(ctx, sdk.LogWarn, "Console über TCP im Netz ohne TLS – Passwörter und Tokens gehen unverschlüsselt über die Leitung",
			map[string]string{"listen": s.Listen})
	}
	return nil
}

// listen öffnet unix://<pfad> oder tcp://<adresse>.
func listen(addr string) (net.Listener, string, error) {
	switch {
	case strings.HasPrefix(addr, "unix://"):
		path, err := filepath.Abs(strings.TrimPrefix(addr, "unix://"))
		if err != nil {
			return nil, "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, "", err
		}
		// Verwaisten Socket eines früheren Laufs entfernen – aber nie eine normale Datei.
		if fi, err := os.Lstat(path); err == nil {
			if fi.Mode()&os.ModeSocket == 0 && runtime.GOOS != "windows" {
				return nil, "", fmt.Errorf("%s existiert und ist kein Socket", path)
			}
			_ = os.Remove(path)
		}
		ln, err := net.Listen("unix", path)
		if err != nil {
			return nil, "", err
		}
		_ = os.Chmod(path, 0o600) // nur der Benutzer des Host-Prozesses
		return ln, path, nil
	case strings.HasPrefix(addr, "tcp://"):
		ln, err := net.Listen("tcp", strings.TrimPrefix(addr, "tcp://"))
		return ln, "", err
	}
	return nil, "", fmt.Errorf("listen muss mit unix:// oder tcp:// beginnen")
}

func isLoopback(a net.Addr) bool {
	if t, ok := a.(*net.TCPAddr); ok {
		return t.IP.IsLoopback()
	}
	return false
}

func (c *console) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	if req.Object == "Console" && req.Action == "Status" {
		c.mu.Lock()
		defer c.mu.Unlock()
		return sdk.Response{Payload: map[string]any{"listen": c.addr}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func main() {
	plugin.Main(&console{})
}
