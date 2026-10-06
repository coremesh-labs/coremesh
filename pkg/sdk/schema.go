package sdk

import "strings"

// Lebenszyklus-Capability für Datenbankschemata.
//
// Ein Modul, das eigene Tabellen braucht, meldet im Manifest die Capability
// {Object: ObjectDBSchema, Actions: []string{ActionInit}}. Sie ist reserviert:
// Der Dispatcher routet sie nicht, nur der Host ruft sie beim Start auf – und
// nur, wenn diese Modulversion noch nicht migriert ist.
//
// Init liefert den Modulnamen und das vollständige Soll-Schema des Moduls als
// Atlas-HCL (SchemaInitResponse). Das Core-Plugin DBSchema vergleicht es mit
// dem Ist-Zustand der Modul-Tabellen und wendet nur die Differenz an – isoliert
// auf das Modul und ohne destruktive Änderungen. Details: README.md in
// internal/coreplugins/dbschema.
const (
	ObjectDBSchema = "DBSchema"
	ActionInit     = "Init"
)

// SchemaInitRequest ist der Payload von DBSchema.Init.
type SchemaInitRequest struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	// Bereits migrierte Versionen des Moduls (älteste zuerst).
	Installed []string `json:"installed"`
}

// SchemaInitResponse ist die Antwort von DBSchema.Init.
type SchemaInitResponse struct {
	// Name des Moduls; muss Manifest.Name entsprechen.
	Module string `json:"module"`
	// Vollständiges Soll-Schema als Atlas-HCL. Alle Tabellen, Indizes und
	// Fremdschlüssel-Namen beginnen mit TablePrefix(Module).
	Schema string `json:"schema"`
	// Optionale Stammdaten (z. B. Katalogwerte). DBSchema fügt jede Zeile in
	// derselben Transaktion wie die Migration ein – nur wenn ihr Primärschlüssel
	// noch nicht existiert (bestehende Zeilen bleiben unverändert). Nur Tabellen
	// aus Schema; Reihenfolge = Einfügereihenfolge (wichtig für Fremdschlüssel).
	Seed []SchemaSeed `json:"seed,omitempty"`
}

// SchemaSeed sind Stammdaten-Zeilen für eine Tabelle des Moduls.
type SchemaSeed struct {
	Table string           `json:"table"`
	Rows  []map[string]any `json:"rows"`
}

// TablePrefix liefert das Pflicht-Präfix für Tabellen, Indizes und
// Constraints eines Moduls: Modulname mit "_" statt "-", gefolgt von "__".
//
//	TablePrefix("partner")         == "partner__"
//	TablePrefix("partner-service") == "partner_service__"
//
// Das doppelte "_" verhindert Überschneidungen: "partner__" ist kein Präfix
// von "partner_service__…".
func TablePrefix(module string) string {
	return strings.ReplaceAll(module, "-", "_") + "__"
}

// SchemaName liefert das DB-Schema eines Moduls bei Schema-Isolation
// (PostgreSQL, dbschema-Setting isolation: schema):
//
//	SchemaName("partner-service") == "mod_partner_service"
//
// Der Host setzt den search_path der Modul-Verbindungen auf dieses Schema;
// unqualifizierte Tabellennamen im Modul-SQL landen also automatisch dort.
func SchemaName(module string) string {
	return "mod_" + strings.ReplaceAll(module, "-", "_")
}
