package tagmanagement

import (
	"context"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/tagservice"
)

// Umschlüsselung: Ändert ein Modul den Schlüssel eines Datensatzes (z. B.
// partner 0.10.0: BP-Nummer statt GUID), meldet es das SystemEvent
// <Object>.rekey mit old_id und new_id. Zuordnungen auf den Datensatz
// (target_entity_id = id oder id|Zeitscheibe) und Verweise in Tags mit
// diesem Object (value_ref) folgen.

const actionRekey = "rekey"

func (m *Module) subscribeRekey(ctx context.Context) {
	if err := events.Register(ctx, m.services, events.Subscription{Object: events.All, Action: actionRekey, CompanyCode: events.All,
		Callback: tagservice.Object}); err != nil {
		m.log.WarnContext(ctx, "Event nicht abonniert", "action", actionRekey, "err", err.Error())
	}
}

func (m *Module) onRekey(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	ev, err := events.Decode(req.Payload)
	if err != nil || ev.Action != actionRekey {
		return sdk.Response{}, err
	}
	old, id := strings.TrimSpace(crud.Str(ev.Data["old_id"])), strings.TrimSpace(crud.Str(ev.Data["new_id"]))
	if ev.Object == "" || old == "" || id == "" || old == id {
		return sdk.Response{}, nil
	}
	return sdk.Response{}, m.db.InTx(ctx, nil, func(ctx context.Context) error {
		// id bzw. id|Zeitscheibe
		if _, err := m.db.Exec(ctx, `UPDATE tag__tag_assignments SET target_entity_id = ? || substr(target_entity_id, ?)
			WHERE target_entity_type = ? AND (target_entity_id = ? OR substr(target_entity_id, 1, ?) = ?)`,
			id, len(old)+1, ev.Object, old, len(old)+1, old+"|"); err != nil {
			return err
		}
		_, err := m.db.Exec(ctx, `UPDATE tag__tag_assignments SET value_ref = ? WHERE value_ref = ?
			AND tag_type_code IN (SELECT code FROM tag__tag_types WHERE ref_object = ?)`, id, old, ev.Object)
		return err
	})
}
