// Package pluginmgr startet, überwacht und beendet Plugin-Prozesse
// über HashiCorp go-plugin (gRPC).
package pluginmgr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/coremesh-labs/coremesh/internal/adapter"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Spec beschreibt einen zu startenden Plugin-Prozess.
type Spec struct {
	Name   string
	Path   string
	SHA256 string // hex; wird von go-plugin vor jedem Start geprüft
	Args   []string
	Env    map[string]string
}

type Manager struct {
	startTimeout time.Duration
	log          hclog.Logger
	reattach     map[string]*goplugin.ReattachConfig

	mu      sync.Mutex
	clients map[string]*goplugin.Client
}

// New liest zusätzlich COREMESH_REATTACH_PLUGINS: Plugins, die dort stehen,
// laufen bereits im Debug-Modus und werden angehängt statt gestartet.
func New(startTimeout time.Duration, log hclog.Logger) (*Manager, error) {
	ra, err := adapter.ReattachFromEnv()
	if err != nil {
		return nil, err
	}
	return &Manager{startTimeout: startTimeout, log: log, reattach: ra, clients: map[string]*goplugin.Client{}}, nil
}

// IsReattach meldet, ob name im Debug-Modus angehängt wird.
func (m *Manager) IsReattach(name string) bool {
	_, ok := m.reattach[name]
	return ok
}

// Start startet den Plugin-Prozess (oder hängt sich an) und liefert das
// Plugin als sdk.Plugin.
func (m *Manager) Start(s Spec) (sdk.Plugin, error) {
	cc := &goplugin.ClientConfig{
		HandshakeConfig:  adapter.Handshake,
		Plugins:          adapter.ClientPlugins(),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           m.log.Named(s.Name),
		StartTimeout:     m.startTimeout,
	}
	if rc, ok := m.reattach[s.Name]; ok {
		cc.Reattach = rc
	} else {
		cmd := exec.Command(s.Path, s.Args...)
		cmd.Env = envList(s.Env)
		cc.Cmd = cmd
		cc.SkipHostEnv = true // Umgebung des Hosts (Secrets!) nicht vererben
		cc.AutoMTLS = true    // gegenseitige TLS-Authentifizierung pro Start
		cc.Managed = true
		if s.SHA256 != "" {
			sum, err := hex.DecodeString(s.SHA256)
			if err != nil {
				return nil, fmt.Errorf("sha256: %w", err)
			}
			cc.SecureConfig = &goplugin.SecureConfig{Checksum: sum, Hash: sha256.New()}
		}
	}

	client := goplugin.NewClient(cc)
	rpc, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, err
	}
	raw, err := rpc.Dispense(adapter.PluginName)
	if err != nil {
		client.Kill()
		return nil, err
	}
	p, ok := raw.(sdk.Plugin)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("Plugin %s liefert kein sdk.Plugin", s.Name)
	}

	m.mu.Lock()
	m.clients[s.Name] = client
	m.mu.Unlock()
	return p, nil
}

// Exited meldet, ob der Prozess von name beendet ist.
func (m *Manager) Exited(name string) bool {
	m.mu.Lock()
	c, ok := m.clients[name]
	m.mu.Unlock()
	return !ok || c.Exited()
}

// Stop beendet den Prozess (Shutdown-RPC, nach 2 s hart). Angehängte
// Debug-Prozesse laufen weiter.
func (m *Manager) Stop(name string) {
	m.mu.Lock()
	c, ok := m.clients[name]
	delete(m.clients, name)
	m.mu.Unlock()
	if ok {
		c.Kill()
	}
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for _, k := range slices.Sorted(maps.Keys(env)) {
		out = append(out, k+"="+env[k])
	}
	return out
}
