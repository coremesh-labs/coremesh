package sdk

import (
	"context"
	"fmt"
)

// Reader liefert das Ergebnis einer Anfrage als Datenstrom – das Gegenstück
// zu Handler für große Datenmengen (z. B. alle Einzelposten eines Jahres für
// ein Rechenmodul). Wie Handler ist es die gemeinsame Aufrufform in beide
// Richtungen:
//
//   - Ein Plugin, das Reader implementiert, liefert die Actions aus
//     Capability.ReadActions als Strom.
//   - Der Host ist ein Reader (HostFrom(ctx)): Read leitet die Anfrage über den
//     Dispatcher an das zuständige Plugin weiter und reicht dessen Strom an w
//     durch.
//
// Routing, Mandant, Benutzer und Berechtigung sind dieselben wie bei Handle
// mit derselben (Object, Action).
//
// Der Anbieter schreibt in w: genau einmal Header, dann beliebig oft Rows.
// Liefert w einen Fehler (Empfänger bricht ab), hört der Anbieter auf und gibt
// ihn zurück. Rows blockiert, solange der Empfänger nicht nachkommt
// (Gegendruck) – große Ergebnisse also Block für Block schreiben, nicht erst
// im Speicher sammeln.
type Reader interface {
	Read(ctx context.Context, req Request, w RowWriter) (ReadEnd, error)
}

// RowWriter nimmt einen Datenstrom entgegen.
type RowWriter interface {
	// Header kommt genau einmal, vor der ersten Zeile.
	Header(h ReadHeader) error
	// Rows liefert einen Block von Zeilen; jede Zeile hat ihre Werte in der
	// Reihenfolge von ReadHeader.Columns. Werte wie im Payload (string,
	// float64/int64, bool, nil, …).
	Rows(rows [][]any) error
}

// ReadHeader beschreibt die Spalten des Stroms.
type ReadHeader struct {
	Columns  []string
	Metadata map[string]string
}

// ReadEnd schließt einen erfolgreichen Strom ab.
type ReadEnd struct {
	// Rows ist die Anzahl gelieferter Zeilen. Auf dem Transportweg ergänzt
	// das SDK den Wert, wenn der Anbieter ihn nicht setzt.
	Rows int64
	// Cursor ist gesetzt, wenn der Anbieter vor dem Ende aufgehört hat (z. B.
	// wegen limit); ein neuer Read mit {"after": Cursor} liefert den Rest.
	Cursor   string
	Metadata map[string]string
}

// ReadFunc passt eine Funktion an Reader an.
type ReadFunc func(ctx context.Context, req Request, w RowWriter) (ReadEnd, error)

func (f ReadFunc) Read(ctx context.Context, req Request, w RowWriter) (ReadEnd, error) {
	return f(ctx, req, w)
}

// ReadAll liest einen Strom vollständig in den Speicher – für Tests und
// kleine Ergebnisse.
func ReadAll(ctx context.Context, r Reader, req Request) (*QueryResult, ReadEnd, error) {
	var c collector
	end, err := r.Read(ctx, req, &c)
	return &QueryResult{Columns: c.h.Columns, Rows: c.rows}, end, err
}

type collector struct {
	h    ReadHeader
	rows [][]any
}

func (c *collector) Header(h ReadHeader) error { c.h = h; return nil }
func (c *collector) Rows(rows [][]any) error   { c.rows = append(c.rows, rows...); return nil }

// StrictWriter erzwingt die Reihenfolge des Stroms (genau ein Header vor den
// Zeilen) und zählt die Zeilen. Das SDK legt ihn um jeden Anbieter; Plugins
// brauchen ihn nur in eigenen Tests.
type StrictWriter struct {
	W         RowWriter
	header    bool
	columns   int
	Delivered int64
}

func (s *StrictWriter) Header(h ReadHeader) error {
	if s.header {
		return fmt.Errorf("Read: Header doppelt")
	}
	s.header, s.columns = true, len(h.Columns)
	return s.W.Header(h)
}

func (s *StrictWriter) Rows(rows [][]any) error {
	if !s.header {
		return fmt.Errorf("Read: Zeilen vor dem Header")
	}
	for i, r := range rows {
		if len(r) != s.columns {
			return fmt.Errorf("Read: Zeile %d hat %d Werte, Header %d Spalten", s.Delivered+int64(i)+1, len(r), s.columns)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	if err := s.W.Rows(rows); err != nil {
		return err
	}
	s.Delivered += int64(len(rows))
	return nil
}

// Finish sorgt für einen Header auch bei leerem Ergebnis und ergänzt
// end.Rows.
func (s *StrictWriter) Finish(end ReadEnd) (ReadEnd, error) {
	if !s.header {
		if err := s.Header(ReadHeader{}); err != nil {
			return end, err
		}
	}
	if end.Rows == 0 {
		end.Rows = s.Delivered
	}
	return end, nil
}
