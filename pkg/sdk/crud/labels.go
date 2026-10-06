package crud

import (
	"context"
	"strings"
)

// WithLabels ergänzt Datensätze um "_labels": {<feld>: <lesbarer Text>} für
// alle Verweise mit LabelFields – je Feld eine Abfrage für alle Datensätze
// (kein N+1). Die Oberfläche zeigt den Text statt des Schlüssels, z. B.
// „Hauptanschrift“ statt MAIN oder die Anschrift statt der Adress-ID.
func (e *Entity) WithLabels(ctx context.Context, recs ...Record) error {
	for i := range e.Fields {
		f := &e.Fields[i]
		if f.Ref == nil || len(f.Ref.LabelFields) == 0 {
			continue
		}
		var values []any
		seen := map[string]bool{}
		for _, rec := range recs {
			if v := Str(rec[f.Key]); v != "" && !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
		if len(values) == 0 {
			continue
		}
		parts := make([]string, len(f.Ref.LabelFields))
		for j, c := range f.Ref.LabelFields {
			parts[j] = "COALESCE(CAST(" + c + " AS TEXT), '')" // portabel: SQLite und PostgreSQL
		}
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(values)), ", ")
		sql := "SELECT " + f.Ref.Column + ", " + strings.Join(parts, " || ' ' || ") +
			" FROM " + f.Ref.Table + " WHERE " + f.Ref.Column + " IN (" + marks + ")"
		if f.Ref.TimeSliced {
			// Mehrere Zeitscheiben je Schlüssel: Die zuletzt gelesene gewinnt – die
			// heute gültige, sonst die jüngste.
			sql += " ORDER BY CASE WHEN valid_from <= ? AND valid_to >= ? THEN 1 ELSE 0 END, valid_from"
			values = append(values, Today(), Today())
		}
		res, err := e.DB().Query(ctx, sql, values...)
		if err != nil {
			return err
		}
		texts := map[string]string{}
		for _, r := range res.Rows {
			texts[Str(r[0])] = strings.Join(strings.Fields(Str(r[1])), " ")
		}
		for _, rec := range recs {
			if t := texts[Str(rec[f.Key])]; t != "" {
				labels, _ := rec["_labels"].(map[string]any)
				if labels == nil {
					labels = map[string]any{}
					rec["_labels"] = labels
				}
				labels[f.Key] = t
			}
		}
	}
	return nil
}
