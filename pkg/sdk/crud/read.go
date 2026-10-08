package crud

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// readPageSize: Zeilen je Datenbankabfrage von ReadList.
const readPageSize = 1000

// ReadList liefert list als Datenstrom (sdk.Reader, Action "list"): dieselben
// Filter, Suche, Gültigkeit, ListScope und Leserechte wie List, aber alle
// Treffer blockweise und ohne Labels und Decorate – Rohdaten für Rechen- und
// Auswertungsmodule.
//
//   - Spalten: die Tabellenspalten der Entity (Columns); Felder ohne Leserecht
//     (Feldgruppen) sind null.
//   - Reihenfolge: nach dem Schlüssel (Keys), unabhängig von Order – so lässt
//     sich ein Strom fortsetzen.
//   - Payload zusätzlich: limit (höchstens so viele Zeilen; gibt es mehr, steht
//     in ReadEnd.Cursor die Fortsetzungsmarke) und after (Marke eines früheren
//     Read: nur Datensätze danach).
//   - Alle Seiten laufen in einer nur lesenden Transaktion (einheitlicher Stand;
//     Zeitlimit read_tx_timeout des Hosts) – oder in der des Aufrufers.
func (e *Entity) ReadList(ctx context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	query := listQuery(req.Payload)
	limit, err := readLimit(query["limit"])
	if err != nil {
		return sdk.ReadEnd{}, err
	}
	var after Record
	if c := Str(query["after"]); c != "" {
		if after, err = e.cursorKey(c); err != nil {
			return sdk.ReadEnd{}, err
		}
	}
	where, args, none, ac, err := e.listWhere(ctx, query)
	if err != nil {
		return sdk.ReadEnd{}, err
	}
	cols := e.Columns()
	if err := w.Header(sdk.ReadHeader{Columns: cols, Metadata: map[string]string{
		"object": e.Object, "keys": strings.Join(e.Keys, ","),
	}}); err != nil || none {
		return sdk.ReadEnd{}, err
	}

	var end sdk.ReadEnd
	err = e.DB().InTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context) error {
		for {
			page := readPageSize
			if limit > 0 && limit-end.Rows < int64(page) {
				page = int(limit - end.Rows)
			}
			if page == 0 {
				// limit erreicht: Gibt es noch mehr, ist der letzte Schlüssel die Marke.
				more, err := e.readPage(ctx, cols, where, args, after, 1)
				if err != nil {
					return err
				}
				if len(more) > 0 {
					end.Cursor = e.RecordID(after)
				}
				return nil
			}
			recs, err := e.readPage(ctx, cols, where, args, after, page)
			if err != nil {
				return err
			}
			if len(recs) == 0 {
				return nil
			}
			rows := make([][]any, len(recs))
			for i, rec := range recs {
				hidden, _ := ac.groups(rec)
				row := make([]any, len(cols))
				for j, c := range cols {
					row[j] = rec[c]
				}
				for _, h := range hidden {
					for j, c := range cols {
						if c == h {
							row[j] = nil
						}
					}
				}
				rows[i] = row
			}
			if err := w.Rows(rows); err != nil {
				return err
			}
			end.Rows += int64(len(recs))
			after = e.KeyOf(recs[len(recs)-1])
			if len(recs) < page {
				return nil
			}
		}
	})
	return end, err
}

// readPage liest bis zu n Datensätze nach dem Schlüssel after (Keyset-Paging).
func (e *Entity) readPage(ctx context.Context, cols, where []string, args []any, after Record, n int) ([]Record, error) {
	if after != nil {
		ph := make([]string, len(e.Keys))
		for i, k := range e.Keys {
			ph[i] = "?"
			args = append(args[:len(args):len(args)], after[k])
		}
		if len(e.Keys) == 1 {
			where = append(where[:len(where):len(where)], e.Keys[0]+" > ?")
		} else {
			where = append(where[:len(where):len(where)], "("+strings.Join(e.Keys, ", ")+") > ("+strings.Join(ph, ", ")+")")
		}
	}
	q := "SELECT " + strings.Join(cols, ", ") + " FROM " + e.Table
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY " + strings.Join(e.Keys, ", ") + " LIMIT " + strconv.Itoa(n)
	res, err := e.DB().Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	recs := make([]Record, len(res.Rows))
	for i, r := range res.Rows {
		rec := Record{}
		for j, c := range cols {
			v, err := Coerce(e.Field(c), r[j])
			if err != nil {
				return nil, err
			}
			rec[c] = v
		}
		recs[i] = rec
	}
	return recs, nil
}

// cursorKey liest die Fortsetzungsmarke (vollständiger Schlüssel wie RecordID).
func (e *Entity) cursorKey(c string) (Record, error) {
	key, err := e.ParseID(c)
	if err != nil {
		return nil, err
	}
	for _, k := range e.Keys {
		v, ok := key[k]
		if !ok {
			return nil, Invalid("after %q: Schlüssel %s fehlt", c, k)
		}
		if key[k], err = Coerce(e.Field(k), v); err != nil {
			return nil, err
		}
	}
	return key, nil
}

func readLimit(v any) (int64, error) {
	if v == nil || Str(v) == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(Str(v), 10, 64)
	if err != nil || n < 0 {
		return 0, Invalid("limit %q: nichtnegative Ganzzahl erwartet", Str(v))
	}
	return n, nil
}
