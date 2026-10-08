// Package numrange ist die Schnittstelle zu den Nummernkreisen (Core-Plugin
// numrange, Object NumberRange): fortlaufende Nummern für Verträge, Belege,
// Abrechnungen … – ohne Doppelvergabe, auch bei gleichzeitigen Anfragen.
//
// Ein Modul meldet beim Start an, welches Nummernkreis-Objekt es braucht, und
// gibt Standardwerte vor. Die Intervalle pflegt der Administrator
// (Nummernkreise → Intervalle): je Objekt, Buchungskreis (oder *),
// Intervallschlüssel und Jahr mit von/bis, Stellenzahl, Format, Verhalten bei
// Überlauf und Warnschwelle. Fehlt ein Intervall, legt der erste Abruf es an –
// aus dem Vorjahr desselben Schlüssels, sonst aus den Standardwerten.
//
//	numrange.Define(ctx, env.Services, numrange.Definition{Object: "Contract",
//		Description: "Vertragsnummern", Owner: "contract", PerCompanyCode: true, PerYear: true,
//		Pattern: "{KEY}-{YYYY}-{N}", Width: 4, From: 1, To: 9999})
//
//	res, err := numrange.Next(ctx, env.Services, numrange.Request{Object: "Contract",
//		CompanyCode: "1000", Key: "MV", Year: 2026, Reference: "Mietvertrag Müller"})
//	// res.Number = "MV-2026-0001"; res.Warning bei erreichter Warnschwelle
//
// Welches Intervall ein Vorgang nutzt (z. B. je Vertragsart), legt das Modul in
// seinen eigenen Katalogen fest (Key) – numrange bleibt allgemein.
//
// Die Nummer wird in einer eigenen Transaktion vergeben. Scheitert danach das
// Speichern im Modul, bleibt eine Lücke; jede vergebene Nummer steht mit
// Zeitpunkt, Benutzer, Request und Referenz im Protokoll (NumberRangeLog).
// Lückenlose Nummernkreise (Definition.GapFree) vergeben stattdessen in der
// Transaktion des Aufrufers (sdk.InTx) – ein Rollback gibt die Nummer zurück.
package numrange

import (
	"context"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Object und Actions der Nummernkreise.
const (
	Object       = "NumberRange"
	ActionDefine = "Define"
	ActionNext   = "Next"
	ActionAssign = "Assign" // intern: nächste Nummer; extern: mitgegebene Nummer prüfen
	ActionInfo   = "Info"   // Vergabeart und Bereich des Intervalls (ohne Anlage)
)

// Verhalten, wenn das Intervall erschöpft ist.
const (
	OverflowError   = "ERROR"   // Fehler – nichts wird vergeben (Standard)
	OverflowRestart = "RESTART" // wieder bei von beginnen (nur, wenn alte Nummern frei sind)
	OverflowNext    = "NEXT"    // im Folgeintervall (NextKey) weiterzählen
)

// Platzhalter im Format.
const (
	PlaceKey         = "{KEY}"  // Intervallschlüssel
	PlaceCompanyCode = "{CC}"   // Buchungskreis
	PlaceYear        = "{YYYY}" // Jahr, vierstellig
	PlaceYear2       = "{YY}"   // Jahr, zweistellig
	PlaceNumber      = "{N}"    // laufende Nummer, mit Nullen auf Width aufgefüllt
)

// Definition: Nummernkreis-Objekt eines Moduls mit Standardwerten für neue
// Intervalle.
type Definition struct {
	Object         string `json:"object"`           // z. B. Contract
	Description    string `json:"description"`      // wofür
	Owner          string `json:"owner"`            // Modul
	PerCompanyCode bool   `json:"per_company_code"` // Intervalle je Buchungskreis
	PerYear        bool   `json:"per_year"`         // Intervalle je Jahr (beginnen jährlich neu)
	Pattern        string `json:"pattern"`          // Standard "{N}"
	Width          int    `json:"width"`            // Stellen von {N}; 0 = ohne Auffüllen
	From           int64  `json:"from"`             // Standard 1
	To             int64  `json:"to"`               // Standard 10^Width-1 bzw. 999999999
	Overflow       string `json:"overflow"`         // Standard OverflowError
	WarnPercent    int    `json:"warn_percent"`     // Standard 90; 0 = keine Warnung
	// GapFree: Vergabe in der Transaktion des Aufrufers (lückenlos, z. B.
	// Buchungsbelege) – ein Rollback gibt die Nummer zurück. Next braucht dann
	// eine laufende Transaktion auf der Datenbank des Nummernkreises.
	GapFree bool `json:"gap_free"`
	// Disjoint: Intervalle verschiedener Schlüssel eines Buchungskreises und
	// Jahres dürfen sich nicht überschneiden (Nummern bleiben über alle
	// Schlüssel eindeutig, z. B. Belegnummern je Ledger).
	Disjoint bool `json:"disjoint"`
}

// Request: nächste Nummer eines Intervalls.
type Request struct {
	Object      string `json:"object"`
	CompanyCode string `json:"company_code"` // bei PerCompanyCode Pflicht
	Key         string `json:"key"`          // Intervallschlüssel; leer = "01"
	Year        int    `json:"year"`         // bei PerYear; 0 = laufendes Jahr
	Reference   string `json:"reference"`    // für das Protokoll, z. B. Bezeichnung des Vorgangs
}

// Result: vergebene Nummer.
type Result struct {
	Number   string `json:"number"`             // formatiert, z. B. MV-2026-0001
	Value    int64  `json:"value"`              // laufende Nummer (extern mit Muster: 0)
	Interval string `json:"interval"`           // ID des Intervalls
	Warning  string `json:"warning,omitempty"`  // Warnschwelle erreicht, Neubeginn …
	External bool   `json:"external,omitempty"` // extern vergeben (Assign mit Wert)
}

// AssignRequest: Nummer nach der Vergabeart des Intervalls. Intern bleibt
// Value leer (nächste Nummer wie Next), extern ist Value die eingegebene
// Nummer – geprüft am Muster des Intervalls bzw. an von–bis. Ob sie frei ist,
// prüft das Modul.
type AssignRequest struct {
	Request
	Value string `json:"value"`
}

// Info: Vergabeart des Intervalls, das ein Abruf nähme (Exists = schon angelegt).
type Info struct {
	Interval        string `json:"interval"`
	Exists          bool   `json:"exists"`
	External        bool   `json:"external"`
	ExternalPattern string `json:"external_pattern,omitempty"`
	From            int64  `json:"from"`
	To              int64  `json:"to"`
	Active          bool   `json:"active"`
}

// Define meldet ein Nummernkreis-Objekt an (beim Start; wiederholbar). Gepflegte
// Intervalle bleiben unverändert.
func Define(ctx context.Context, s module.Services, d Definition) error {
	_, err := s.Call(ctx, Object, ActionDefine, d)
	return err
}

// Next vergibt die nächste Nummer.
func Next(ctx context.Context, s module.Services, r Request) (Result, error) {
	resp, err := s.Call(ctx, Object, ActionNext, r)
	if err != nil {
		return Result{}, err
	}
	var out Result
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return Result{}, err
	}
	return out, nil
}

// Assign vergibt eine Nummer nach der Vergabeart des Intervalls (intern oder
// extern, siehe AssignRequest).
func Assign(ctx context.Context, s module.Services, r AssignRequest) (Result, error) {
	resp, err := s.Call(ctx, Object, ActionAssign, r)
	if err != nil {
		return Result{}, err
	}
	var out Result
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return Result{}, err
	}
	return out, nil
}

// IntervalInfo liefert die Vergabeart des Intervalls, das ein Abruf nähme.
func IntervalInfo(ctx context.Context, s module.Services, r Request) (Info, error) {
	resp, err := s.Call(ctx, Object, ActionInfo, r)
	if err != nil {
		return Info{}, err
	}
	var out Info
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return Info{}, err
	}
	return out, nil
}
