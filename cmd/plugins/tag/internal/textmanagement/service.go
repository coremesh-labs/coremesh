package textmanagement

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/textservice"
)

// target ist der Ziel-Datensatz: Buchungskreis und Feldwerte (für Bedingungen).
type target struct {
	cc    string
	attrs map[string]string
}

// targetOf prüft den Ziel-Datensatz über die eigene Action des Fachmoduls
// (<Object>.get: Existenz, Leserecht) und das Recht <Object>.read bzw. .update
// im Buchungskreis des Datensatzes.
func (m *Module) targetOf(ctx context.Context, entityType, entityID string, write bool) (target, error) {
	if !objectRe.MatchString(entityType) {
		return target{}, crud.Invalid("Objekttyp %q: Name eines Objects erwartet, z. B. Contract", entityType)
	}
	if strings.TrimSpace(entityID) == "" {
		return target{}, crud.Invalid("Datensatz (entity_id) ist Pflicht")
	}
	resp, err := m.services.Call(ctx, entityType, "get", map[string]any{"id": entityID})
	switch {
	case errors.Is(err, sdk.ErrNotFound):
		return target{}, crud.Invalid("%s %s gibt es nicht", entityType, entityID)
	case err != nil:
		return target{}, err
	}
	var rec map[string]any
	_ = sdk.Decode(resp.Payload, &rec)
	t := target{attrs: map[string]string{}}
	for k, v := range rec {
		if !strings.HasPrefix(k, "_") {
			t.attrs[k] = attrText(v)
		}
	}
	t.cc = firstNonEmpty(t.attrs["company_code"], t.attrs["company_code_id"])
	action := "read"
	if write {
		action = "update"
	}
	ok, err := sdk.CheckAccess(ctx, entityType, action, t.cc)
	if err != nil {
		return target{}, err
	}
	if !ok {
		return target{}, fmt.Errorf("%w: keine Berechtigung für %s.%s (Texte)", sdk.ErrPermissionDenied, entityType, action)
	}
	return t, nil
}

func attrText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case bool:
		return strconv.FormatBool(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func (m *Module) categoryOf(ctx context.Context, code string) (textservice.Category, error) {
	res, err := m.db.Query(ctx, `SELECT code, name, cardinality FROM tag__text_category WHERE code = ? AND is_active = ?`, code, true)
	if err != nil {
		return textservice.Category{}, err
	}
	if len(res.Rows) == 0 {
		return textservice.Category{}, crud.Invalid("Textkategorie %q gibt es nicht", code)
	}
	r := res.Rows[0]
	return textservice.Category{Code: crud.Str(r[0]), Name: crud.Str(r[1]), Cardinality: crud.Str(r[2])}, nil
}

// allowed: am Datensatz erlaubte Kategorien – Zuordnungen des Objects im Buchungskreis
// des Datensatzes oder für alle (*), mit erfüllter Bedingung; der Buchungskreis geht vor.
func (m *Module) allowed(ctx context.Context, entityType string, t target) ([]textservice.Category, error) {
	res, err := m.db.Query(ctx, `SELECT a.category, c.name, c.cardinality, a.mandatory, a.condition_field, a.condition_values, a.company_code
		FROM tag__text_assignment a JOIN tag__text_category c ON c.code = a.category AND c.is_active = ?
		WHERE a.entity_type = ? AND (a.company_code = ? OR a.company_code = ?)
		ORDER BY CASE WHEN a.company_code = '*' THEN 1 ELSE 0 END, a.sort_order, c.sort_order, a.category`, true, entityType, "*", t.cc)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []textservice.Category{}
	for _, r := range res.Rows {
		code := crud.Str(r[0])
		if seen[code] {
			continue // Zuordnung des Buchungskreises vor der für alle
		}
		if f := crud.Str(r[4]); f != "" {
			ok := false
			for _, v := range splitValues(crud.Str(r[5])) {
				if strings.EqualFold(v, t.attrs[f]) {
					ok = true
				}
			}
			if !ok {
				continue
			}
		}
		seen[code] = true
		out = append(out, textservice.Category{Code: code, Name: crud.Str(r[1]), Cardinality: crud.Str(r[2]), Mandatory: crud.AsBool(r[3])})
	}
	return out, nil
}

func (m *Module) languages(ctx context.Context) ([]textservice.Language, error) {
	res, err := m.db.Query(ctx, `SELECT code, name FROM tag__text_language WHERE is_active = ? ORDER BY sort_order, code`, true)
	if err != nil {
		return nil, err
	}
	out := []textservice.Language{}
	for _, r := range res.Rows {
		out = append(out, textservice.Language{Code: crud.Str(r[0]), Name: crud.Str(r[1])})
	}
	return out, nil
}

const textCols = `x.id, x.entity_type, x.entity_id, x.category, COALESCE(c.name, x.category), x.position, x.language, x.content, x.changed_at, x.changed_by`

func toText(r []any) textservice.Text {
	pos, _ := strconv.Atoi(crud.Str(r[5]))
	return textservice.Text{ID: crud.Str(r[0]), EntityType: crud.Str(r[1]), EntityID: crud.Str(r[2]), Category: crud.Str(r[3]),
		CategoryName: crud.Str(r[4]), Position: pos, Language: crud.Str(r[6]), Content: crud.Str(r[7]), ChangedAt: crud.Str(r[8]), ChangedBy: crud.Str(r[9])}
}

func (m *Module) texts(ctx context.Context, entityType, entityID, language string) ([]textservice.Text, error) {
	q := `SELECT ` + textCols + ` FROM tag__text x LEFT JOIN tag__text_category c ON c.code = x.category
		LEFT JOIN tag__text_language l ON l.code = x.language WHERE x.entity_type = ? AND x.entity_id = ?`
	args := []any{entityType, entityID}
	if language != "" {
		q += ` AND x.language = ?`
		args = append(args, language)
	}
	res, err := m.db.Query(ctx, q+` ORDER BY c.sort_order, x.category, x.position, l.sort_order, x.language`, args...)
	if err != nil {
		return nil, err
	}
	out := []textservice.Text{}
	for _, r := range res.Rows {
		out = append(out, toText(r))
	}
	return out, nil
}

func missing(cats []textservice.Category, items []textservice.Text) []textservice.Category {
	has := map[string]bool{}
	for _, t := range items {
		if strings.TrimSpace(t.Content) != "" {
			has[t.Category] = true
		}
	}
	out := []textservice.Category{}
	for _, c := range cats {
		if c.Mandatory && !has[c.Code] {
			out = append(out, c)
		}
	}
	return out
}

func (m *Module) listAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in textservice.ListRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	t, err := m.targetOf(ctx, in.EntityType, in.EntityID, false)
	if err != nil {
		return sdk.Response{}, err
	}
	out := textservice.ListResponse{}
	if out.Items, err = m.texts(ctx, in.EntityType, in.EntityID, strings.TrimSpace(in.Language)); err != nil {
		return sdk.Response{}, err
	}
	if out.Categories, err = m.allowed(ctx, in.EntityType, t); err != nil {
		return sdk.Response{}, err
	}
	if out.Languages, err = m.languages(ctx); err != nil {
		return sdk.Response{}, err
	}
	all := out.Items
	if in.Language != "" {
		all, _ = m.texts(ctx, in.EntityType, in.EntityID, "")
	}
	out.Missing = missing(out.Categories, all)
	_, werr := m.targetOf(ctx, in.EntityType, in.EntityID, true)
	out.CanEdit = werr == nil
	return sdk.Response{Payload: out}, nil
}

func (m *Module) checkAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in textservice.CheckRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	t, err := m.targetOf(ctx, in.EntityType, in.EntityID, false)
	if err != nil {
		return sdk.Response{}, err
	}
	cats, err := m.allowed(ctx, in.EntityType, t)
	if err != nil {
		return sdk.Response{}, err
	}
	items, err := m.texts(ctx, in.EntityType, in.EntityID, "")
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: textservice.CheckResponse{Missing: missing(cats, items)}}, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func (m *Module) load(ctx context.Context, id string) (textservice.Text, error) {
	res, err := m.db.Query(ctx, `SELECT `+textCols+` FROM tag__text x LEFT JOIN tag__text_category c ON c.code = x.category WHERE x.id = ?`, id)
	if err != nil {
		return textservice.Text{}, err
	}
	if len(res.Rows) == 0 {
		return textservice.Text{}, fmt.Errorf("%w: Text %s", sdk.ErrNotFound, id)
	}
	return toText(res.Rows[0]), nil
}

// saveAction: Text anlegen (ohne id) oder ändern (Inhalt, Position, Sprache).
//
//   - Kategorie muss am Datensatz erlaubt sein (Zuordnung);
//   - einmalige Kategorie: Position 1; mehrfache: Position ≥ 1, ohne Angabe die
//     nächste freie – derselbe Text in einer anderen Sprache nimmt dieselbe Position;
//   - je Datensatz, Kategorie, Position und Sprache ein Text.
func (m *Module) saveAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in textservice.Text
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	in.ID = strings.TrimSpace(in.ID)
	if in.ID != "" {
		old, err := m.load(ctx, in.ID)
		if err != nil {
			return sdk.Response{}, err
		}
		in.EntityType, in.EntityID, in.Category = old.EntityType, old.EntityID, old.Category
		if in.Position == 0 {
			in.Position = old.Position
		}
		if strings.TrimSpace(in.Language) == "" {
			in.Language = old.Language
		}
	}
	t, err := m.targetOf(ctx, in.EntityType, in.EntityID, true)
	if err != nil {
		return sdk.Response{}, err
	}
	in.Category = strings.ToUpper(strings.TrimSpace(in.Category))
	cats, err := m.allowed(ctx, in.EntityType, t)
	if err != nil {
		return sdk.Response{}, err
	}
	var cat *textservice.Category
	for i := range cats {
		if cats[i].Code == in.Category {
			cat = &cats[i]
		}
	}
	if cat == nil {
		if _, err := m.categoryOf(ctx, in.Category); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{}, crud.Invalid("Textkategorie %s ist an %s %s nicht zugeordnet (Texte → Zuordnungen)", in.Category, in.EntityType, in.EntityID)
	}
	in.Language = strings.TrimSpace(in.Language)
	if in.Language == "" {
		return sdk.Response{}, crud.Invalid("Sprachschlüssel ist Pflicht")
	}
	langs, err := m.languages(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	known := false
	for _, l := range langs {
		known = known || l.Code == in.Language
	}
	if !known {
		return sdk.Response{}, crud.Invalid("Sprachschlüssel %q gibt es nicht (Texte → Sprachen)", in.Language)
	}
	in.Content = strings.TrimRight(in.Content, " \t\r\n")
	if strings.TrimSpace(in.Content) == "" {
		return sdk.Response{}, crud.Invalid("Text ist leer – zum Löschen „Entfernen“")
	}
	switch {
	case in.Position < 0:
		return sdk.Response{}, crud.Invalid("Position %d: 1 oder größer", in.Position)
	case cat.Cardinality == textservice.Single && in.Position > 1:
		return sdk.Response{}, crud.Invalid("Kategorie %s gibt es nur einmal je Datensatz (Position 1)", cat.Name)
	case cat.Cardinality == textservice.Single:
		in.Position = 1
	case in.Position == 0:
		res, err := m.db.Query(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM tag__text WHERE entity_type = ? AND entity_id = ? AND category = ?`,
			in.EntityType, in.EntityID, in.Category)
		if err != nil {
			return sdk.Response{}, err
		}
		n, _ := strconv.Atoi(crud.Str(res.Rows[0][0]))
		in.Position = n
	}
	res, err := m.db.Query(ctx, `SELECT id FROM tag__text WHERE entity_type = ? AND entity_id = ? AND category = ? AND position = ? AND language = ? AND id <> ?`,
		in.EntityType, in.EntityID, in.Category, in.Position, in.Language, in.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	if len(res.Rows) > 0 {
		return sdk.Response{}, crud.Invalid("%s Position %d in Sprache %s gibt es schon – den vorhandenen Text ändern", cat.Name, in.Position, in.Language)
	}
	user := nullable(sdk.CallFromContext(ctx).UserID)
	if in.ID == "" {
		in.ID = crud.NewID()
		_, err = m.db.Exec(ctx, `INSERT INTO tag__text (id, entity_type, entity_id, company_code, category, position, language, content,
			created_at, created_by, changed_at, changed_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, in.ID, in.EntityType, in.EntityID,
			nullable(t.cc), in.Category, in.Position, in.Language, in.Content, now(), user, now(), user)
	} else {
		_, err = m.db.Exec(ctx, `UPDATE tag__text SET position = ?, language = ?, content = ?, changed_at = ?, changed_by = ? WHERE id = ?`,
			in.Position, in.Language, in.Content, now(), user, in.ID)
	}
	if err != nil {
		return sdk.Response{}, err
	}
	out, err := m.load(ctx, in.ID)
	return sdk.Response{Payload: out}, err
}

func (m *Module) removeAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in textservice.RemoveRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	old, err := m.load(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return sdk.Response{}, err
	}
	if _, err := m.targetOf(ctx, old.EntityType, old.EntityID, true); err != nil {
		return sdk.Response{}, err
	}
	_, err = m.db.Exec(ctx, `DELETE FROM tag__text WHERE id = ?`, old.ID)
	return sdk.Response{Payload: map[string]any{"id": old.ID, "removed": true}}, err
}

// onRekey: <Object>.rekey (old_id → new_id) – Texte folgen dem neuen Schlüssel.
func (m *Module) onRekey(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	ev, err := events.Decode(req.Payload)
	if err != nil || ev.Action != "rekey" {
		return sdk.Response{}, err
	}
	old, id := strings.TrimSpace(crud.Str(ev.Data["old_id"])), strings.TrimSpace(crud.Str(ev.Data["new_id"]))
	if ev.Object == "" || old == "" || id == "" || old == id {
		return sdk.Response{}, nil
	}
	_, err = m.db.Exec(ctx, `UPDATE tag__text SET entity_id = ? || substr(entity_id, ?)
		WHERE entity_type = ? AND (entity_id = ? OR substr(entity_id, 1, ?) = ?)`, id, len(old)+1, ev.Object, old, len(old)+1, old+"|")
	return sdk.Response{}, err
}
