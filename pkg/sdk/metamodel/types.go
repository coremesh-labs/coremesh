// Package metamodel beschreibt Business-Objects deklarativ: Felder, Typen,
// Aktionen. Fachmodule liefern ihre Definitionen über die
// Lebenszyklus-Capability Catalog.Describe; das Catalog-Plugin hält sie vor,
// der WebServer baut daraus Tabellen und Formulare.
package metamodel

// ObjectDefinition beschreibt ein Business-Object.
type ObjectDefinition struct {
	Name     string            `json:"name"`                // z. B. "BusinessPartner" – Object im Dispatcher
	Title    string            `json:"title"`               // z. B. "Geschäftspartner"
	TitleKey string            `json:"title_key,omitempty"` // Übersetzungsschlüssel für Title (i18n)
	Icon     string            `json:"icon,omitempty"`      // Icon-CSS-Klasse
	Fields   []FieldDefinition `json:"fields"`              // Spalten / Formularfelder
	Actions  []ActionConfig    `json:"actions"`             // Erlaubte Aktionen

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

	// FormState: Action, die die Maske für die aktuellen Formularwerte bestimmt
	// (sichtbar, Pflicht, schreibgeschützt, Vorbelegung, Auswahlwerte je Feld).
	// Der WebServer ruft sie beim Öffnen des Formulars und bei jeder Änderung
	// eines Felds mit Trigger auf (Payload FormStateRequest, Antwort FormState).
	FormState string `json:"form_state,omitempty"`
	// Filters sind die Felder, nach denen die Übersicht filtern kann (Parameter
	// der list-Action); Search: list versteht den Suchparameter q.
	Filters []string `json:"filters,omitempty"`
	Search  bool     `json:"search,omitempty"`
	// Authorization beschreibt die Berechtigungsfelder des Objects und
	// Actions, die nur als Berechtigung existieren (z. B. FiscalPeriod.post).
	// Die Rollenpflege (iam) bietet sie aus dem Catalog an. Optional.
	Authorization *Authorization `json:"authorization,omitempty"`
}

// Authorization: Berechtigungen eines Objects bis auf Feldwerte.
//
// Eine Rolle erlaubt Object.Action in Buchungskreisen und – optional – nur
// für bestimmte Werte der Berechtigungsfelder (Einzelwerte, Bereiche, *).
// Das Modul prüft mit sdk.Authorize bzw. sdk.Grants und übergibt dabei die
// Feldwerte unter diesen Schlüsseln; der Buchungskreis heißt immer
// "company_code" und ist kein Berechtigungsfeld.
type Authorization struct {
	Fields  []string     `json:"fields,omitempty"`  // Feld-Keys des Objects
	Actions []AuthAction `json:"actions,omitempty"` // zusätzliche Actions ohne Route
	// FieldGroups: Felder, die nur mit Recht readFields bzw. changeFields
	// (Berechtigungsfeld field_group) sichtbar bzw. änderbar sind.
	FieldGroups []FieldGroup `json:"field_groups,omitempty"`
}

// FieldGroup: sensible Felder eines Objects, z. B. Bankverbindung.
type FieldGroup struct {
	Key      string   `json:"key"` // [a-z][a-z0-9_]*, Wert des Berechtigungsfelds field_group
	Label    string   `json:"label"`
	LabelKey string   `json:"label_key,omitempty"`
	Fields   []string `json:"fields"`
}

// Feldgruppen: Berechtigungsfeld und Actions.
const (
	FieldGroupAttr     = "field_group"
	ActionReadFields   = "readFields"
	ActionChangeFields = "changeFields"
	ActionRead         = "read" // Datensatzberechtigung (Liste, Detail, Lookup)
)

// AuthAction ist eine Action, die nur geprüft, nie aufgerufen wird
// (z. B. "post" auf FiscalPeriod: in dieser Periode buchen).
type AuthAction struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	LabelKey string `json:"label_key,omitempty"` // Übersetzungsschlüssel (i18n)
}

// FieldDefinition beschreibt ein Feld (Tabellenspalte, Formularfeld).
type FieldDefinition struct {
	Key      string    `json:"key"`                 // z. B. "company_name"
	Label    string    `json:"label"`               // z. B. "Firmenname"
	LabelKey string    `json:"label_key,omitempty"` // Übersetzungsschlüssel, z. B. "businesspartner.BusinessPartner.fields.name1"
	Type     FieldType `json:"type"`                // siehe FieldType
	Required bool      `json:"required"`            // Pflichtfeld im Formular
	Listable bool      `json:"listable"`            // in der Übersichtstabelle anzeigen
	Editable bool      `json:"editable"`            // im Formular bearbeitbar
	Options  []Option  `json:"options,omitempty"`   // nur für TypeSelect
	// Lookup: Der Wert ist der Schlüssel eines Datensatzes eines anderen
	// Objects (Fremdschlüssel). Die Oberfläche bietet einen Auswahldialog an.
	Lookup *Lookup `json:"lookup,omitempty"`
	// Group gliedert das Formular (Feldgruppe, z. B. "Kontierung"); leere bzw.
	// ausgeblendete Gruppen entfallen.
	Group    string `json:"group,omitempty"`
	GroupKey string `json:"group_key,omitempty"` // Übersetzungsschlüssel der Gruppe
	// Trigger: Eine Änderung wertet die Maske neu aus (FormState, ShowIf, RequiredIf).
	Trigger bool `json:"trigger,omitempty"`
	// Deklarative Regeln ohne Plugin-Aufruf: sichtbar bzw. Pflicht, wenn das Feld
	// Field einen der Werte hat.
	ShowIf     *Condition `json:"show_if,omitempty"`
	RequiredIf *Condition `json:"required_if,omitempty"`
	// ActionOnly: Eingabe nur in Formularen eigener Aktionen (ActionConfig.Fields),
	// z. B. „Buchen bis“ an einem Vertrag. Erscheint nicht in Liste, Detail,
	// Neu oder Bearbeiten und ist kein Datenfeld des Objects.
	ActionOnly bool `json:"action_only,omitempty"`
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
	// Filters schränkt die Auswahl ein: Zielfeld → Quelle. Quelle ist ein Feld des
	// Formulars ("company_code_id") oder ein Feld des Datensatzes, auf den ein
	// Lookup-Feld des Formulars zeigt ("draft_id.company_code_id"), "=wert" ein fester Wert.
	Filters map[string]string `json:"filters,omitempty"`
}

// SectionDefinition ist ein aufklappbarer Abschnitt der Detailansicht.
// Genau eines von Fields und Relation ist gesetzt.
type SectionDefinition struct {
	Key       string    `json:"key"` // eindeutig im Object, [a-z][a-z0-9_]*
	Title     string    `json:"title"`
	TitleKey  string    `json:"title_key,omitempty"` // Übersetzungsschlüssel (i18n)
	Collapsed bool      `json:"collapsed,omitempty"` // anfangs zugeklappt
	Fields    []string  `json:"fields,omitempty"`    // Feld-Keys dieses Objects
	Relation  *Relation `json:"relation,omitempty"`  // eingebettete Unter-Objekte (Master-Detail)
	// Tags: Abschnitt mit den Tags des Datensatzes (Plugin tag, TagService) –
	// Eingabefelder entstehen aus den Tag Sets, die dem Object zugewiesen sind.
	Tags bool `json:"tags,omitempty"`
	// Documents: Abschnitt mit den Dokumentverweisen des Datensatzes (Object
	// Documents, pkg/sdk/docservice – Plugin document oder ein DMS-Adapter).
	Documents bool `json:"documents,omitempty"`
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
	// Match: Felder des Unter-Objects → Felder des Masters, wenn der Master einen
	// zusammengesetzten Schlüssel hat (z. B. {"company_code": "company_code",
	// "building_id": "building_id"}). Gesetzt muss es ForeignKey enthalten; ohne
	// Match erhält ForeignKey die id des Masters.
	Match map[string]string `json:"match,omitempty"`
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
	Value    string `json:"value"`
	Label    string `json:"label"`
	LabelKey string `json:"label_key,omitempty"` // Übersetzungsschlüssel (i18n)
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
	// Übersetzungsschlüssel für Label und Confirm (i18n).
	LabelKey   string `json:"label_key,omitempty"`
	ConfirmKey string `json:"confirm_key,omitempty"`
	// Nur Kind custom: Fields sind die Felder des Formulars (leer = alle editierbaren
	// Felder des Objects). Record: die Aktion gilt einem Datensatz (Payload mit id)
	// und erscheint in der Detailansicht, sonst in der Übersicht.
	Fields []string `json:"fields,omitempty"`
	Record bool     `json:"record,omitempty"`
	// FormState: Beim Öffnen des Formulars fragt der WebServer die Maske des
	// Objects ab (ObjectDefinition.FormState, Mode "action", Action = Name) –
	// Vorbelegung und Hinweis, z. B. „Buchen bis“ = heute und der letzte Lauf.
	FormState bool `json:"form_state,omitempty"`
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
	// Übersetzungen der Texte dieses Plugins (Locale → Schlüssel → Text).
	// Jeder Schlüssel beginnt mit dem Namen eines eigenen Moduls und ".".
	Translations Translations `json:"translations,omitempty"`
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
	StatusField string        `json:"status_field,omitempty"` // status: Feld – TypeBoolean (true = aktiv) oder TypeSelect
	// InactiveValue: bei einem Status-Feld vom Typ select der Wert für „inaktiv“
	// (z. B. "DEPRECATED"); deactivate setzt ihn. Leer bei TypeBoolean.
	InactiveValue string `json:"inactive_value,omitempty"`
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

// Condition: Das Feld Field hat einen der Werte Values.
type Condition struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// Holds wertet die Bedingung für Formularwerte aus.
func (c *Condition) Holds(values map[string]string) bool {
	if c == nil {
		return true
	}
	for _, v := range c.Values {
		if values[c.Field] == v {
			return true
		}
	}
	return false
}

// FormStateRequest fragt die Maske eines Formulars ab (ObjectDefinition.FormState).
type FormStateRequest struct {
	Mode   string            `json:"mode"`             // create | edit | action
	Action string            `json:"action,omitempty"` // action: Name der Aktion (ActionConfig.FormState)
	ID     string            `json:"id,omitempty"`     // edit, action: Datensatz
	Values map[string]string `json:"values"`           // aktuelle Formularwerte
	Locked []string          `json:"locked,omitempty"` // feste Felder (Master-Detail)
}

// FormState ist die Maske für die aktuellen Werte. Felder ohne Eintrag bleiben
// wie im Metamodell.
type FormState struct {
	Fields  map[string]FieldState `json:"fields"`
	Message string                `json:"message,omitempty"` // Hinweis über dem Formular
}

// FieldState überschreibt die Eigenschaften eines Felds; nil = unverändert.
type FieldState struct {
	Visible  *bool    `json:"visible,omitempty"`
	Required *bool    `json:"required,omitempty"`
	ReadOnly *bool    `json:"readonly,omitempty"`
	Value    *string  `json:"value,omitempty"`   // gesetzter Wert (z. B. abgeleitet)
	Options  []Option `json:"options,omitempty"` // erlaubte Werte (Auswahlfeld)
}
