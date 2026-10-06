// Package metamodel beschreibt Business-Objects deklarativ: Felder, Typen,
// Aktionen. Fachmodule liefern ihre Definitionen über die
// Lebenszyklus-Capability Catalog.Describe; das Catalog-Plugin hält sie vor,
// der WebServer baut daraus Tabellen und Formulare.
package metamodel

// ObjectDefinition beschreibt ein Business-Object.
type ObjectDefinition struct {
	Name    string            `json:"name"`           // z. B. "BusinessPartner" – Object im Dispatcher
	Title   string            `json:"title"`          // z. B. "Geschäftspartner"
	Icon    string            `json:"icon,omitempty"` // Icon-CSS-Klasse
	Fields  []FieldDefinition `json:"fields"`         // Spalten / Formularfelder
	Actions []ActionConfig    `json:"actions"`        // Erlaubte Aktionen
}

// FieldDefinition beschreibt ein Feld (Tabellenspalte, Formularfeld).
type FieldDefinition struct {
	Key      string    `json:"key"`               // z. B. "company_name"
	Label    string    `json:"label"`             // z. B. "Firmenname"
	Type     FieldType `json:"type"`              // siehe FieldType
	Required bool      `json:"required"`          // Pflichtfeld im Formular
	Listable bool      `json:"listable"`          // in der Übersichtstabelle anzeigen
	Editable bool      `json:"editable"`          // im Formular bearbeitbar
	Options  []Option  `json:"options,omitempty"` // nur für TypeSelect
}

// FieldType bestimmt Darstellung und Eingabe eines Felds.
type FieldType string

const (
	TypeText     FieldType = "text"
	TypeTextarea FieldType = "textarea"
	TypeNumber   FieldType = "number"
	TypeDate     FieldType = "date"
	TypeSelect   FieldType = "select"
	TypeEmail    FieldType = "email"
	TypeBoolean  FieldType = "boolean"
	// TypePassword: Eingabe verdeckt, wird nie angezeigt oder vorbelegt.
	TypePassword FieldType = "password"
)

// Option ist ein Eintrag eines Auswahlfelds.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ActionConfig beschreibt eine Aktion, die die Oberfläche anbietet.
// Name ist die Action im Dispatcher: {Name: "list"} → Object.list.
type ActionConfig struct {
	Name  string     `json:"name"`
	Kind  ActionKind `json:"kind"`
	Label string     `json:"label"`
	Icon  string     `json:"icon,omitempty"`
	// Sicherheitsabfrage vor dem Ausführen, z. B. bei delete.
	Confirm string `json:"confirm,omitempty"`
}

// ActionKind sagt dem WebServer, wie er eine Aktion einbindet.
type ActionKind string

const (
	KindList   ActionKind = "list"   // Übersicht: liefert eine Liste von Datensätzen
	KindItem   ActionKind = "item"   // Detail: liefert einen Datensatz (Payload {id})
	KindCreate ActionKind = "create" // Neuanlage aus Formulardaten
	KindUpdate ActionKind = "update" // Änderung aus Formulardaten (inkl. id)
	KindDelete ActionKind = "delete" // Löschen (Payload {id})
	KindCustom ActionKind = "custom" // sonstige Aktion (Button)
)

// DescribeRequest ist der Payload von Catalog.Describe.
type DescribeRequest struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

// DescribeResponse ist die Antwort von Catalog.Describe.
type DescribeResponse struct {
	Objects []ObjectDefinition `json:"objects"`
}
