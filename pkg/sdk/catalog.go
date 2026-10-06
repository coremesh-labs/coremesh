package sdk

// Lebenszyklus-Capability für Metamodell-Definitionen.
//
// Ein Modul, das seine Business-Objects für die Oberfläche beschreibt, meldet
// im Manifest {Object: ObjectCatalog, Actions: []string{ActionDescribe}}. Sie
// ist reserviert wie DBSchema.Init: Der Dispatcher routet sie nicht. Der Host
// ruft Describe bei jedem Start auf, nachdem die Routen des Moduls
// registriert sind (Payload metamodel.DescribeRequest, Antwort
// metamodel.DescribeResponse), und reicht die Definitionen mit dem geprüften
// Modulnamen an Catalog.Register weiter.
const (
	ObjectCatalog  = "Catalog"
	ActionDescribe = "Describe"
)
