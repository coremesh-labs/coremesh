package adapter

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	goplugin "github.com/hashicorp/go-plugin"
)

// ReattachEnv ist die Umgebungsvariable, über die sich der Host an Plugins
// anhängt, die im Debug-Modus bereits laufen, statt sie selbst zu starten.
// Wert: JSON-Objekt Plugin-Name -> ReattachInfo.
const ReattachEnv = "COREMESH_REATTACH_PLUGINS"

// ReattachInfo ist die JSON-Form von goplugin.ReattachConfig
// (dessen net.Addr lässt sich nicht direkt serialisieren).
type ReattachInfo struct {
	Protocol        string       `json:"protocol"`
	ProtocolVersion int          `json:"protocol_version"`
	Pid             int          `json:"pid"`
	Test            bool         `json:"test"`
	Addr            ReattachAddr `json:"addr"`
}

type ReattachAddr struct {
	Network string `json:"network"`
	String  string `json:"string"`
}

func NewReattachInfo(c *goplugin.ReattachConfig) ReattachInfo {
	return ReattachInfo{
		Protocol:        string(c.Protocol),
		ProtocolVersion: c.ProtocolVersion,
		Pid:             c.Pid,
		Test:            c.Test,
		Addr:            ReattachAddr{Network: c.Addr.Network(), String: c.Addr.String()},
	}
}

// Config wandelt die JSON-Form zurück in eine goplugin.ReattachConfig.
func (r ReattachInfo) Config() (*goplugin.ReattachConfig, error) {
	var addr net.Addr
	var err error
	switch r.Addr.Network {
	case "tcp":
		addr, err = net.ResolveTCPAddr("tcp", r.Addr.String)
	case "unix":
		addr, err = net.ResolveUnixAddr("unix", r.Addr.String)
	default:
		err = fmt.Errorf("unbekanntes Netzwerk %q", r.Addr.Network)
	}
	if err != nil {
		return nil, err
	}
	return &goplugin.ReattachConfig{
		Protocol:        goplugin.Protocol(r.Protocol),
		ProtocolVersion: r.ProtocolVersion,
		Addr:            addr,
		Pid:             r.Pid,
		Test:            r.Test,
	}, nil
}

// ReattachFromEnv liest ReattachEnv. Ohne gesetzte Variable ist das Ergebnis leer.
func ReattachFromEnv() (map[string]*goplugin.ReattachConfig, error) {
	raw := os.Getenv(ReattachEnv)
	if raw == "" {
		return nil, nil
	}
	var infos map[string]ReattachInfo
	if err := json.Unmarshal([]byte(raw), &infos); err != nil {
		return nil, fmt.Errorf("%s: %w", ReattachEnv, err)
	}
	out := make(map[string]*goplugin.ReattachConfig, len(infos))
	for name, info := range infos {
		cfg, err := info.Config()
		if err != nil {
			return nil, fmt.Errorf("%s[%s]: %w", ReattachEnv, name, err)
		}
		out[name] = cfg
	}
	return out, nil
}
