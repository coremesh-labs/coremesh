package businesspartner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// „Partnerkonto …“ am Geschäftspartner: alle Debitoren- und Kreditorenposten
// mit laufendem Saldo, je Vertrag der Stand (ausgeglichen bzw. Soll-/Haben-Saldo).
// Gerechnet im GuV-Modul (profit-loss, LedgerReport.partnerAccount); fehlt es,
// meldet die Aktion das.

const ledgerReportObject = "LedgerReport"

var partnerAccountFields = []field{
	{Key: "statement_company_code", Label: "Buchungskreis (leer = alle)", Type: tText, ActionOnly: true,
		Lookup: &metamodel.Lookup{Object: "CompanyCode", ValueField: "code", LabelFields: []string{"description"}}},
	{Key: "statement_from", Label: "Von (leer = erster Posten)", Type: tDate, ActionOnly: true},
	{Key: "statement_to", Label: "Bis (leer = heute)", Type: tDate, ActionOnly: true},
}

func (m *Module) partnerAccountAction() crud.Action {
	return crud.Action{ActionConfig: metamodel.ActionConfig{Name: "partnerAccount", Label: "Partnerkonto …", Record: true,
		Fields: []string{"statement_company_code", "statement_from", "statement_to"}}, Handle: m.partnerAccount}
}

func (m *Module) partnerAccount(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			CompanyCode string `json:"statement_company_code"`
			From        string `json:"statement_from"`
			To          string `json:"statement_to"`
		} `json:"data"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	bp, _, _ := strings.Cut(in.ID, "|") // ID der Zeitscheibe: <BP-Nummer>|<gültig ab>
	if strings.TrimSpace(bp) == "" {
		return sdk.Response{}, invalid("Geschäftspartner fehlt")
	}
	call := map[string]any{"partner_id": bp}
	if cc := strings.TrimSpace(in.Data.CompanyCode); cc != "" {
		call["company_code"] = cc
	}
	for k, v := range map[string]string{"period_from": in.Data.From, "period_to": in.Data.To} {
		if strings.TrimSpace(v) == "" {
			continue
		}
		d, err := parseDate(v)
		if err != nil {
			return sdk.Response{}, err
		}
		call[k] = d
	}
	resp, err := m.services.Call(ctx, ledgerReportObject, "partnerAccount", call)
	if errors.Is(err, sdk.ErrUnimplemented) {
		return resp, fmt.Errorf("%w: Partnerkonto nicht verfügbar – Plugin profit-loss (GuV) ist nicht gestartet", sdk.ErrUnavailable)
	}
	return resp, err
}
