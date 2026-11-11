package iam

import (
	"context"
	"testing"
	"time"
)

// TestDashboardAllowed: Das Dashboard eines Moduls darf, wer in dem Modul etwas darf.
func TestDashboardAllowed(t *testing.T) {
	p := &Plugin{}
	p.dash.byKey = map[string][]string{"BankDashboard.status": {"BankDashboard", "BankImport", "BankAccount", "BankTransaction"}}
	p.dash.at = time.Now()
	ctx := context.Background()
	bank := []grant{{Object: "BankAccount", Action: "list"}}
	other := []grant{{Object: "Process", Action: "*"}}
	if !p.dashboardAllowed(ctx, bank, "BankDashboard", "status") {
		t.Fatal("Recht im Modul: Dashboard erlaubt")
	}
	if p.dashboardAllowed(ctx, other, "BankDashboard", "status") {
		t.Fatal("ohne Recht im Modul: kein Dashboard")
	}
	if p.dashboardAllowed(ctx, bank, "BankDashboard", "other") || p.dashboardAllowed(ctx, bank, "BankImport", "import") {
		t.Fatal("nur die gemeldete Action des Dashboards")
	}
}
