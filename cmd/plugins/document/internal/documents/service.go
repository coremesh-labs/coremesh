package documents

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/docservice"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
)

var objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{1,63}$`)

// target prüft den Ziel-Datensatz: Es gibt ihn (<Object>.get), und der
// Benutzer darf ihn lesen bzw. ändern (<Object>.read bzw. .update im
// Buchungskreis des Datensatzes).
func (m *Module) target(ctx context.Context, entityType, entityID string, write bool) error {
	if !objectRe.MatchString(entityType) {
		return crud.Invalid("Objekttyp %q: Name eines Objects erwartet, z. B. Contract", entityType)
	}
	if strings.TrimSpace(entityID) == "" {
		return crud.Invalid("Datensatz (entity_id) ist Pflicht")
	}
	resp, err := m.services.Call(ctx, entityType, "get", map[string]any{"id": entityID})
	switch {
	case errors.Is(err, sdk.ErrNotFound):
		return crud.Invalid("%s %s gibt es nicht", entityType, entityID)
	case err != nil:
		return err
	}
	var rec map[string]any
	_ = sdk.Decode(resp.Payload, &rec)
	cc := crud.Str(rec["company_code"])
	if cc == "" {
		cc = crud.Str(rec["company_code_id"])
	}
	action := "read"
	if write {
		action = "update"
	}
	ok, err := sdk.CheckAccess(ctx, entityType, action, cc)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: keine Berechtigung für %s.%s (Dokumente)", sdk.ErrPermissionDenied, entityType, action)
	}
	return nil
}

func (m *Module) canEdit(ctx context.Context, entityType, entityID string) bool {
	return m.target(ctx, entityType, entityID, true) == nil
}

const refCols = `r.id, r.entity_type, r.entity_id, r.doc_type, COALESCE(t.name, r.doc_type), r.title, r.doc_date, r.location, r.valid_from,
	r.valid_to, r.note, r.created_at, r.created_by`

func toDoc(r []any) docservice.Document {
	d := func(v any) string {
		if crud.Str(v) == "" {
			return ""
		}
		s, _ := crud.ParseDate(v)
		return s
	}
	return docservice.Document{ID: crud.Str(r[0]), EntityType: crud.Str(r[1]), EntityID: crud.Str(r[2]), DocType: crud.Str(r[3]),
		DocTypeName: crud.Str(r[4]), Title: crud.Str(r[5]), DocDate: d(r[6]), Location: crud.Str(r[7]), ValidFrom: d(r[8]),
		ValidTo: d(r[9]), Note: crud.Str(r[10]), CreatedAt: crud.Str(r[11]), CreatedBy: crud.Str(r[12])}
}

func (m *Module) types(ctx context.Context) ([]docservice.DocType, error) {
	res, err := m.db.Query(ctx, `SELECT code, name FROM document__type WHERE is_active = ? ORDER BY sort_order, code`, true)
	if err != nil {
		return nil, err
	}
	out := []docservice.DocType{}
	for _, r := range res.Rows {
		out = append(out, docservice.DocType{Code: crud.Str(r[0]), Name: crud.Str(r[1])})
	}
	return out, nil
}

func (m *Module) listAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in docservice.ListRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if err := m.target(ctx, in.EntityType, in.EntityID, false); err != nil {
		return sdk.Response{}, err
	}
	q := `SELECT ` + refCols + ` FROM document__reference r LEFT JOIN document__type t ON t.code = r.doc_type
		WHERE r.entity_type = ? AND r.entity_id = ? AND r.removed_at IS NULL`
	args := []any{in.EntityType, in.EntityID}
	if in.EffectiveDate != "" {
		d, err := crud.ParseDate(in.EffectiveDate)
		if err != nil {
			return sdk.Response{}, err
		}
		q += ` AND (r.valid_from IS NULL OR r.valid_from <= ?) AND (r.valid_to IS NULL OR r.valid_to >= ?)`
		args = append(args, d, d)
	}
	res, err := m.db.Query(ctx, q+` ORDER BY r.doc_date DESC, r.created_at DESC`, args...)
	if err != nil {
		return sdk.Response{}, err
	}
	out := docservice.ListResponse{Items: []docservice.Document{}, CanEdit: m.canEdit(ctx, in.EntityType, in.EntityID)}
	for _, r := range res.Rows {
		out.Items = append(out.Items, toDoc(r))
	}
	if out.Types, err = m.types(ctx); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: out}, nil
}

func (m *Module) typesAction(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
	t, err := m.types(ctx)
	return sdk.Response{Payload: map[string]any{"types": t}}, err
}

// checkDoc: Pflichtfelder, Dokumentart, Daten.
func (m *Module) checkDoc(ctx context.Context, d *docservice.Document) error {
	d.DocType = strings.ToUpper(strings.TrimSpace(d.DocType))
	d.Title, d.Location = strings.TrimSpace(d.Title), strings.TrimSpace(d.Location)
	if d.Title == "" || d.Location == "" {
		return crud.Invalid("Titel und Ablage (Dateiname, Ablageort oder Link) sind Pflicht")
	}
	res, err := m.db.Query(ctx, `SELECT 1 FROM document__type WHERE code = ? AND is_active = ?`, d.DocType, true)
	if err != nil {
		return err
	}
	if len(res.Rows) == 0 {
		return crud.Invalid("Dokumentart %q gibt es nicht", d.DocType)
	}
	for _, p := range []*string{&d.DocDate, &d.ValidFrom, &d.ValidTo} {
		if *p == "" {
			continue
		}
		if *p, err = crud.ParseDate(*p); err != nil {
			return err
		}
	}
	if d.ValidFrom != "" && d.ValidTo != "" && d.ValidTo < d.ValidFrom {
		return crud.Invalid("Gültig bis liegt vor gültig ab")
	}
	return nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func (m *Module) attachAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in docservice.AttachRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if err := m.target(ctx, in.EntityType, in.EntityID, true); err != nil {
		return sdk.Response{}, err
	}
	d := in.Document
	if err := m.checkDoc(ctx, &d); err != nil {
		return sdk.Response{}, err
	}
	d.ID, d.EntityType, d.EntityID, d.CreatedAt, d.CreatedBy = crud.NewID(), in.EntityType, in.EntityID, now(), sdk.CallFromContext(ctx).UserID
	if _, err := m.db.Exec(ctx, `INSERT INTO document__reference (id, entity_type, entity_id, doc_type, title, doc_date, location, valid_from, valid_to,
		note, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, d.ID, d.EntityType, d.EntityID, d.DocType, d.Title,
		nullable(d.DocDate), d.Location, nullable(d.ValidFrom), nullable(d.ValidTo), nullable(d.Note), d.CreatedAt, nullable(d.CreatedBy)); err != nil {
		return sdk.Response{}, err
	}
	return m.respond(ctx, d.ID)
}

func (m *Module) load(ctx context.Context, id string) (docservice.Document, error) {
	res, err := m.db.Query(ctx, `SELECT `+refCols+` FROM document__reference r LEFT JOIN document__type t ON t.code = r.doc_type
		WHERE r.id = ? AND r.removed_at IS NULL`, id)
	if err != nil {
		return docservice.Document{}, err
	}
	if len(res.Rows) == 0 {
		return docservice.Document{}, fmt.Errorf("%w: Dokument %s", sdk.ErrNotFound, id)
	}
	return toDoc(res.Rows[0]), nil
}

func (m *Module) respond(ctx context.Context, id string) (sdk.Response, error) {
	d, err := m.load(ctx, id)
	return sdk.Response{Payload: d}, err
}

func (m *Module) updateAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in docservice.UpdateRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	old, err := m.load(ctx, in.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := m.target(ctx, old.EntityType, old.EntityID, true); err != nil {
		return sdk.Response{}, err
	}
	d := in.Document
	if err := m.checkDoc(ctx, &d); err != nil {
		return sdk.Response{}, err
	}
	if _, err := m.db.Exec(ctx, `UPDATE document__reference SET doc_type = ?, title = ?, doc_date = ?, location = ?, valid_from = ?, valid_to = ?,
		note = ? WHERE id = ?`, d.DocType, d.Title, nullable(d.DocDate), d.Location, nullable(d.ValidFrom), nullable(d.ValidTo), nullable(d.Note),
		in.ID); err != nil {
		return sdk.Response{}, err
	}
	return m.respond(ctx, in.ID)
}

// detachAction: Verweis entfernen – bleibt mit removed_at gespeichert.
func (m *Module) detachAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in docservice.DetachRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	old, err := m.load(ctx, in.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := m.target(ctx, old.EntityType, old.EntityID, true); err != nil {
		return sdk.Response{}, err
	}
	_, err = m.db.Exec(ctx, `UPDATE document__reference SET removed_at = ?, removed_by = ? WHERE id = ?`, now(),
		nullable(sdk.CallFromContext(ctx).UserID), in.ID)
	return sdk.Response{Payload: map[string]any{"id": in.ID, "removed": true}}, err
}

// onRekey: <Object>.rekey (old_id → new_id) – Verweise folgen dem neuen
// Schlüssel (auch in der Form id|Zeitscheibe).
func (m *Module) onRekey(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	ev, err := events.Decode(req.Payload)
	if err != nil || ev.Action != "rekey" {
		return sdk.Response{}, err
	}
	old, id := strings.TrimSpace(crud.Str(ev.Data["old_id"])), strings.TrimSpace(crud.Str(ev.Data["new_id"]))
	if ev.Object == "" || old == "" || id == "" || old == id {
		return sdk.Response{}, nil
	}
	_, err = m.db.Exec(ctx, `UPDATE document__reference SET entity_id = ? || substr(entity_id, ?)
		WHERE entity_type = ? AND (entity_id = ? OR substr(entity_id, 1, ?) = ?)`, id, len(old)+1, ev.Object, old, len(old)+1, old+"|")
	return sdk.Response{}, err
}
