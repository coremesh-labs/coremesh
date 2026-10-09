package businesspartner

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/hook"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Kontenplanwechsel eines Buchungskreises (Hook ledger.chart_change des
// Hauptbuchs): Die Abstimmkonten der Partner im Buchungskreis müssen in der
// Zuordnung alt → neu stehen (check) und werden umgestellt (commit).

const (
	chartChangeHook     = "ledger.chart_change"
	chartChangeCallback = "PartnerChartChange"
)

type chartChange struct {
	CompanyCode string            `json:"company_code"`
	ToChart     string            `json:"to_chart"`
	Mapping     map[string]string `json:"mapping"`
}

func (m *Module) registerChartChange(r *module.Router) {
	hook.Handle(r, chartChangeCallback, m.onChartChange)
}

func (m *Module) subscribeChartChange(ctx context.Context) {
	for _, phase := range []string{hook.PhaseCheck, hook.PhaseCommit} {
		if err := hook.Subscribe(ctx, m.services, hook.Subscription{Hook: chartChangeHook, Phase: phase, Callback: chartChangeCallback,
			Priority: 50, Description: "Geschäftspartner: Abstimmkonten beim Kontenplanwechsel prüfen und umstellen"}); err != nil {
			m.log.WarnContext(ctx, "Hook nicht abonniert", "hook", chartChangeHook, "err", err.Error())
		}
	}
}

func (m *Module) onChartChange(ctx context.Context, req hook.Request) (hook.Response, error) {
	var in chartChange
	if err := req.DecodeData(&in); err != nil {
		return hook.Response{}, err
	}
	res, err := m.db.Query(ctx, `SELECT DISTINCT reconciliation_account FROM partner__company_codes
		WHERE company_code = ? AND reconciliation_account IS NOT NULL AND reconciliation_account <> ''`, in.CompanyCode)
	if err != nil {
		return hook.Response{}, err
	}
	var missing []string
	for _, r := range res.Rows {
		if a := fmt.Sprint(r[0]); in.Mapping[a] == "" {
			missing = append(missing, a)
		}
	}
	slices.Sort(missing)
	switch req.Action {
	case hook.PhaseCheck:
		if len(missing) > 0 {
			return hook.Reply(nil, hook.Error("BP-CHART-1", fmt.Sprintf("Geschäftspartner: Abstimmkonten ohne Zuordnung in Buchungskreis %s: %s",
				in.CompanyCode, strings.Join(missing, ", ")))), nil
		}
	case hook.PhaseCommit:
		n := 0
		err := m.db.InTx(ctx, nil, func(ctx context.Context) error {
			for _, r := range res.Rows {
				from := fmt.Sprint(r[0])
				to := in.Mapping[from]
				if to == "" || to == from {
					continue
				}
				// über einen Platzhalter, damit Ketten (a → b, b → c) nicht doppelt umstellen
				x, err := m.db.Exec(ctx, `UPDATE partner__company_codes SET reconciliation_account = ? WHERE company_code = ? AND reconciliation_account = ?`,
					"#"+to, in.CompanyCode, from)
				if err != nil {
					return err
				}
				n += int(x.RowsAffected)
			}
			_, err := m.db.Exec(ctx, `UPDATE partner__company_codes SET reconciliation_account = SUBSTR(reconciliation_account, 2)
				WHERE company_code = ? AND reconciliation_account LIKE '#%'`, in.CompanyCode)
			return err
		})
		if err != nil {
			return hook.Response{}, err
		}
		return hook.Reply(nil, hook.Info("BP-CHART-2", fmt.Sprintf("Geschäftspartner: %d Abstimmkonten auf %s umgestellt", n, in.ToChart))), nil
	}
	return hook.Reply(nil), nil
}
