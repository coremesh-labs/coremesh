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

	// TitleField benennt das Feld, das einen Datensatz in Überschriften
	// vertritt (z. B. "name1"). Optional.
	TitleField string `json:"title_field,omitempty"`
	// Sections gliedern die Detailansicht in aufklappbare Abschnitte: Felder
	// des Objects oder eingebettete Unter-Objekte (Relation). Felder ohne
	// Abschnitt erscheinen in einem vorangestellten Abschnitt „Allgemein“.
	Sections []SectionDefinition `json:"sections,omitempty"`
	// Lifecycle bestimmt, ob und wie ein Datensatz enden kann (kein
	// physisches Löschen). Leer = immutable.
	Lifecycle Lifecycle `json:"lifecycle"`
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
	// Lookup: Der Wert ist der Schlüssel eines Datensatzes eines anderen
	// Objects (Fremdschlüssel). Die Oberfläche bietet einen Auswahldialog an.
	Lookup *Lookup `json:"lookup,omitempty"`
}

// Lookup beschreibt die Auswahl eines Werts aus einem Nachschlage-Object
// (Stammdatentabelle, Katalog). Der Dialog ruft dessen Action vom Kind list
// mit {"query": {"q": <Suche>}} auf.
//
// Anzeige: Liefert das Modul in einem Datensatz "_labels": {<key>: <Text>},
// zeigt die Oberfläche den Text statt des Schlüssels.
type Lookup struct {
	Object      string   `json:"object"`            // Nachschlage-Object, z. B. "PartnerAddressRole"
	ValueField  string   `json:"value_field"`       // Feld des Ziels, dessen Wert übernommen wird (z. B. "code")
	LabelFields []string `json:"label_fields"`      // Felder des Ziels für den lesbaren Text (mit Leerzeichen verbunden)
	Columns     []string `json:"columns,omitempty"` // Spalten im Dialog (Standard: listable Felder des Ziels)
}

// SectionDefinition ist ein aufklappbarer Abschnitt der Detailansicht.
// Genau eines von Fields und Relation ist gesetzt.
type SectionDefinition struct {
	Key       string    `json:"key"` // eindeutig im Object, [a-z][a-z0-9_]*
	Title     string    `json:"title"`
	Collapsed bool      `json:"collapsed,omitempty"` // anfangs zugeklappt
	Fields    []string  `json:"fields,omitempty"`    // Feld-Keys dieses Objects
	Relation  *Relation `json:"relation,omitempty"`  // eingebettete Unter-Objekte (Master-Detail)
}

// Relation verbindet ein Object (Master) mit Datensätzen eines Unter-Objects
// (Detail), die über ForeignKey auf die id des Masters verweisen. Eine
// n:m-Beziehung ist eine Relation auf die Zwischentabelle, deren zweiter
// Schlüssel ein Lookup ist – z. B. BusinessPartner → PartnerAddress
// (bp_id, address_role_code) → PartnerAddressData über address_id.
//
// Konvention für das Unter-Object: list filtert mit {"query": {<ForeignKey>: <id>}},
// create/update/delete wie im WebServer üblich. Unter-Object und Master gehören
// zum selben Plugin und Modul.
type Relation struct {
	Object     string   `json:"object"`            // Unter-Object, z. B. "PartnerAddress"
	ForeignKey string   `json:"foreign_key"`       // Feld des Unter-Objects mit der id des Masters, z. B. "bp_id"
	Columns    []string `json:"columns,omitempty"` // Spalten der eingebetteten Tabelle (Standard: listable ohne ForeignKey)
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
	// Physisches Löschen gibt es nicht. Wie ein Datensatz endet, bestimmt der
	// Lebenszyklus des Objects (siehe Lifecycle):
	KindExpire     ActionKind = "expire"     // Typ timeslice: Gültigkeit beenden (Payload {id, valid_to})
	KindDeactivate ActionKind = "deactivate" // Typ status: inaktivieren (Payload {id})
	KindCustom     ActionKind = "custom"     // sonstige Aktion (Button)
)

// DescribeRequest ist der Payload von Catalog.Describe.
type DescribeRequest struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

// DescribeResponse ist die Antwort von Catalog.Describe.
type DescribeResponse struct {
	Objects []ObjectDefinition `json:"objects"`
	// Module dieses Plugins. Jedes Object, das in der Oberfläche erscheinen
	// soll, gehört zu genau einem Modul (siehe ModuleDefinition).
	Modules []ModuleDefinition `json:"modules,omitempty"`
}

// LifecycleType unterscheidet, wie ein Datensatz eines Objects endet.
// Physisches Löschen ist im System nicht vorgesehen.
type LifecycleType string

const (
	// LifecycleTimeSlice: Datensätze mit Zeitscheibe (ValidFrom/ValidTo). Sie
	// enden, indem der Benutzer ein Enddatum wählt (Action vom Kind expire,
	// Payload {id, valid_to}) – nie automatisch mit dem Tagesdatum.
	LifecycleTimeSlice LifecycleType = "timeslice"
	// LifecycleStatus: Datensätze mit Status-Flag. Sie werden nach Bestätigung
	// inaktiviert (Action vom Kind deactivate, Payload {id}: Flag = false).
	LifecycleStatus LifecycleType = "status"
	// LifecycleImmutable: weder Zeitscheibe noch Status-Flag. Datensätze
	// können weder gelöscht noch deaktiviert werden.
	LifecycleImmutable LifecycleType = "immutable"
)

// Lifecycle beschreibt den Lebenszyklus eines Objects.
type Lifecycle struct {
	Type        LifecycleType `json:"type"`                   // timeslice | status | immutable (leer = immutable)
	ValidFrom   string        `json:"valid_from,omitempty"`   // timeslice: Feld „gültig ab“ (TypeDate)
	ValidTo     string        `json:"valid_to,omitempty"`     // timeslice: Feld „gültig bis“ (TypeDate)
	StatusField string        `json:"status_field,omitempty"` // status: Feld (TypeBoolean), true = aktiv
}

// Kind liefert den Typ; ein leerer Lebenszyklus ist immutable.
func (l Lifecycle) Kind() LifecycleType {
	if l.Type == "" {
		return LifecycleImmutable
	}
	return l.Type
}

// EndAction liefert das Kind der Action, mit der ein Datensatz endet
// (expire bzw. deactivate), oder "" bei immutable.
func (l Lifecycle) EndAction() ActionKind {
	switch l.Kind() {
	case LifecycleTimeSlice:
		return KindExpire
	case LifecycleStatus:
		return KindDeactivate
	}
	return ""
}
