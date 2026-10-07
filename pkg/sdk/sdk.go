// Package sdk enthält die öffentlichen Abstraktionen für Plugin-Entwickler:
// Plugin- und Host-Interface, Request/Response, CallContext und Fehler.
//
// Das Paket ist bewusst frei von gRPC, Protobuf und go-plugin. Die Übersetzung
// auf das Wire-Protokoll übernimmt pkg/sdk/plugin. Ein Plugin sieht so aus:
//
//	import (
//		"github.com/coremesh-labs/coremesh/pkg/sdk"
//		"github.com/coremesh-labs/coremesh/pkg/sdk/plugin"
//	)
//
//	func main() { plugin.Serve(&myPlugin{}) }
package sdk

import "context"

// Manifest beschreibt Identität und Fähigkeiten eines Plugins.
type Manifest struct {
	Name         string
	Version      string
	Description  string
	Capabilities []Capability
}

// Capability ist ein Business-Object mit den darauf unterstützten Aktionen.
// Jedes Paar (Object, Actions[i]) ist ein Routing-Eintrag im Dispatcher.
type Capability struct {
	Object      string
	Actions     []string
	Description string
}

// Config enthält die Einstellungen aus der Host-Config und den Rückkanal zum Host.
// Host ist derselbe, den HostFrom(ctx) in jedem Handle-Aufruf liefert; Plugins
// müssen ihn also nicht speichern.
type Config struct {
	Settings map[string]any
	Host     Host
}

// Request ist eine vom Dispatcher weitergeleitete Aktion auf einem Business-Object.
// Mandant, Benutzer und Korrelations-ID liefert CallFromContext(ctx).
type Request struct {
	Object  string
	Action  string
	Payload any
}

// Response ist die Antwort des Plugins. Payload muss sich als JSON darstellen
// lassen (Struct, Map, Slice, Skalar oder nil).
type Response struct {
	Payload  any
	Metadata map[string]string
}

// Handler verarbeitet eine Anfrage auf einem Business-Object. Es ist die
// gemeinsame Aufrufform im ganzen System:
//
//   - Jedes Plugin ist ein Handler (es bedient seine Capabilities).
//   - Der Host ist ein Handler (HostFrom(ctx)): Handle leitet die Anfrage über
//     den Dispatcher an das für (Object, Action) zuständige Plugin weiter.
//
// Ein Plugin ruft ein anderes Plugin also genauso auf, wie der Host es selbst
// aufruft – ohne zu wissen, welches Plugin die Anfrage bedient.
//
// Fehler werden bevorzugt mit den Err*-Sentinels gemeldet, z. B.
// fmt.Errorf("%w: partner %s", sdk.ErrNotFound, id).
type Handler interface {
	Handle(ctx context.Context, req Request) (Response, error)
}

// Plugin ist das Interface, das jedes Plugin implementiert.
//
// Nebenläufigkeit: Handle wird gleichzeitig aus mehreren Goroutinen
// aufgerufen – jede Anfrage läuft in einer eigenen Goroutine, auch mehrere
// Anfragen an dasselbe Plugin. Gemeinsamer Zustand im Plugin muss daher
// geschützt sein (sync.Mutex, sync/atomic) oder unveränderlich nach
// Configure. Configure läuft vor dem ersten Handle.
//
// Auf Host-Seite implementiert der gRPC-Client dasselbe Interface, sodass der
// Host einen Plugin-Prozess wie ein lokales Objekt anspricht.
type Plugin interface {
	Handler
	Manifest(ctx context.Context) (Manifest, error)
	Configure(ctx context.Context, cfg Config) error
}

// Shutdowner ist optional: Implementiert ein Plugin es, ruft das SDK Shutdown
// auf, wenn der Host das Plugin beendet (nach dem letzten Handle), bzw. der
// Host selbst bei internen Plugins. Hier werden Hintergrund-Goroutinen
// gestoppt und Puffer geschrieben; ctx begrenzt die Dauer.
type Shutdowner interface {
	Shutdown(ctx context.Context) error
}
