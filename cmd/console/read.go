package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/coremesh-labs/coremesh/pkg/consoleapi/console/v1"
)

// readStream ruft --object/--action als Datenstrom ab und schreibt die Zeilen
// fortlaufend als CSV (Kopfzeile = Spalten) oder JSON-Zeilen.
func readStream(ctx context.Context, c consolev1.ConsoleServiceClient, o options) error {
	format := o.format
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "jsonl" {
		return fmt.Errorf("--read: --format csv oder jsonl, nicht %q", format)
	}
	if o.command != "" {
		return errors.New("--read nur mit --object und --action")
	}
	p, err := structpb.NewStruct(o.params)
	if err != nil {
		return err
	}
	stream, err := c.Read(ctx, &consolev1.ExecuteRequest{TargetObject: o.object, TargetAction: o.action, Parameters: p})
	if err != nil {
		return err
	}
	// Erst ab der ersten Nachricht schreiben: Ein Fehler beim Anmelden oder
	// eine fehlende Berechtigung legt so keine leere Datei an.
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	out, closeOut, err := openOut(o.out)
	if err != nil {
		return err
	}
	defer closeOut()
	sink := newSink(format, out)
	var columns []string
	for msg := first; ; {
		switch part := msg.GetPart().(type) {
		case *consolev1.ReadChunk_Columns:
			columns = part.Columns.GetNames()
			if err := sink.header(columns); err != nil {
				return err
			}
		case *consolev1.ReadChunk_Rows:
			for _, r := range part.Rows.GetRows() {
				if err := sink.row(columns, r.AsSlice()); err != nil {
					return err
				}
			}
		case *consolev1.ReadChunk_End:
			if err := sink.flush(); err != nil {
				return err
			}
			summary := fmt.Sprintf("%d Zeilen", part.End.GetRows())
			if o.out != "" {
				summary += " nach " + o.out
			}
			if cur := part.End.GetCursor(); cur != "" {
				summary += fmt.Sprintf("; weitere mit --param after=%s", cur)
			}
			fmt.Fprintln(os.Stderr, summary)
			return nil
		}
		if msg, err = stream.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("Datenstrom ohne Abschluss")
			}
			return err
		}
	}
}

func openOut(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

type sink struct {
	csv  *csv.Writer
	buf  *bufio.Writer
	json *json.Encoder
}

func newSink(format string, w io.Writer) *sink {
	buf := bufio.NewWriterSize(w, 1<<16)
	if format == "csv" {
		return &sink{csv: csv.NewWriter(buf), buf: buf}
	}
	return &sink{json: json.NewEncoder(buf), buf: buf}
}

func (s *sink) header(cols []string) error {
	if s.csv != nil {
		return s.csv.Write(cols)
	}
	return nil
}

func (s *sink) row(cols []string, values []any) error {
	if s.csv != nil {
		rec := make([]string, len(values))
		for i, v := range values {
			rec[i] = csvValue(v)
		}
		return s.csv.Write(rec)
	}
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		if i < len(values) {
			m[c] = values[i]
		}
	}
	return s.json.Encode(m)
}

func (s *sink) flush() error {
	if s.csv != nil {
		s.csv.Flush()
		if err := s.csv.Error(); err != nil {
			return err
		}
	}
	return s.buf.Flush()
}

// csvValue: Ganzzahlen ohne Exponent, NULL leer, Strukturen als JSON.
func csvValue(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprint(int64(v))
		}
		return fmt.Sprint(v)
	case bool:
		return fmt.Sprint(v)
	}
	b, _ := json.Marshal(v)
	return string(b)
}
