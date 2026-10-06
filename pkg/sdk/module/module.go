// Package module ist die Modul-Schicht des SDK: Ein Modul bündelt fachlich
// zusammengehörige Business-Objects, ihre Logik und ihre Endpunkte unter
// einem eigenen Namensraum und kapselt sie nach außen.
//
//	Plugin (Prozess, DB-Präfix)        z. B. "partner"
//	└── Modul (Namensraum, Navigation)  z. B. "businesspartner" → /m/businesspartner, /api/v1/businesspartner
//	    └── Business-Objects            BusinessPartner, PartnerRole, …
//	        └── Actions                 list, get, create, …
//
// Ein Modul implementiert Module. NewPlugin macht aus einem oder mehreren
// Modulen ein sdk.Plugin; das Manifest, DBSchema.Init und Catalog.Describe
// entstehen dabei automatisch aus den registrierten Routen:
//
//	func main() {
//		plugin.Main(module.NewPlugin(module.Info{Name: "partner", Version: "0.2.0"},
//			businesspartner.New()))
//	}
//
// Lebenszyklus eines Moduls:
//
//  1. RegisterRoutes – beim Erzeugen des Plugins, vor dem Start. Rein
//     deklarativ: Objects, Actions und Metamodelle anmelden. Der Host
//     braucht die Routen schon für das Manifest, also vor Initialize.
//  2. Initialize – einmal beim Start (Configure), vor der ersten Anfrage.
//     Das Modul erhält seine Ressourcen über Env (Logger, Datenbank,
//     Konfiguration, Aufrufe anderer Module) und speichert sie.
//  3. Handler der Routen – nebenläufig, eine Goroutine je Anfrage.
//  4. Shutdown – wenn der Host das Plugin beendet. Kurz halten (ca. 1,5 s).
package module

import (
	"context"

	"github.com/camel/coremesh/pkg/sdk"
)

// Module ist die Schnittstelle jedes fachlichen Moduls.
type Module interface {
	// Descriptor liefert Namensraum und Darstellung des Moduls.
	Descriptor() Descriptor
	// RegisterRoutes meldet die Objects und Actions des Moduls an. Der Router
	// ist auf das Modul beschränkt (gekapseltes Sub-Routing).
	RegisterRoutes(r *Router)
	// Initialize übergibt die Ressourcen des Moduls (Dependency Injection).
	Initialize(ctx context.Context, env Env) error
	// Shutdown gibt Ressourcen frei.
	Shutdown(ctx context.Context) error
}

// Descriptor beschreibt ein Modul. Name ist der Namensraum: eindeutig im
// ganzen System, Kleinbuchstaben, Ziffern und "-" (URL-Segment).
type Descriptor struct {
	Name        string
	Title       string
	Icon        string
	Description string
}

// SchemaProvider ist optional: Ein Modul mit eigenen Tabellen liefert sein
// Soll-Schema (Atlas-HCL) und Stammdaten. Das Plugin reicht beides über
// DBSchema.Init weiter. Tabellen müssen das Präfix sdk.TablePrefix(<Plugin>)
// tragen – DBSchema lehnt alles andere ab, auch Fremdschlüssel auf Tabellen
// fremder Plugins. Bedient ein Plugin mehrere Module mit Schema, prüft
// NewPlugin zusätzlich das Modul-Präfix (siehe TablePrefix).
type SchemaProvider interface {
	Schema() Schema
}

// Schema ist das Soll-Schema eines Moduls. HCL enthält nur table-Blöcke mit
// schema = schema.main; den schema-Block ergänzt das Plugin.
type Schema struct {
	HCL  string
	Seed []sdk.SchemaSeed
}

// Base implementiert Initialize und Shutdown ohne Wirkung; zum Einbetten in
// Module ohne eigene Ressourcen.
type Base struct{}

func (Base) Initialize(context.Context, Env) error { return nil }
func (Base) Shutdown(context.Context) error        { return nil }
