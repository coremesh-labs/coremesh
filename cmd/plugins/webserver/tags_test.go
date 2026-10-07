package main

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/tagservice"
)

// tagsHost spielt den TagService: Tag Set RISK mit Auswahlwert, Betrag, Datum;
// Regel SHOW_IF RISK=HIGH → LIMIT.
type tagsHost struct{ *fakeHost }

var tagSchema = tagservice.Schema{EntityType: "Partner", EffectiveDate: "2026-10-06", Sets: []tagservice.TagSet{{
	Code: "RISK_SET", Name: "Risiko", CompanyCode: "*",
	Items: []tagservice.SetItem{
		{Scope: "*", Mandatory: true, Tag: tagservice.TagType{Code: "RISK", Name: "Risikoklasse", DataType: "STRING", ValueMode: "OPTIONS", Status: "ACTIVE",
			Options: []tagservice.ValueOption{{Code: "LOW", Label: "Niedrig"}, {Code: "HIGH", Label: "Hoch"}}}},
		{Scope: "1000", Tag: tagservice.TagType{Code: "LIMIT", Name: "Kreditlimit", DataType: "CURRENCY", ValueMode: "FREE", Status: "ACTIVE"}},
		{Scope: "*", Tag: tagservice.TagType{Code: "AUDIT", Name: "Prüfdatum", DataType: "DATE", ValueMode: "FREE", Status: "DEPRECATED"}},
		{Scope: "*", Tag: tagservice.TagType{Code: "OBJ", Name: "Mietobjekt", DataType: "REFERENCE", ValueMode: "FREE", Status: "ACTIVE", RefObject: "Address"}},
	},
	Rules: []tagservice.Rule{{Type: tagservice.RuleShowIf, Source: "RISK", Condition: "HIGH", Target: "LIMIT"}},
}}}

func (h tagsHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	if req.Object == tagservice.Object || req.Object+"."+req.Action == "CompanyCode.list" {
		h.fakeHost.record(ctx, req)
	}
	switch req.Object + "." + req.Action {
	case "CompanyCode.list":
		return sdk.Response{Payload: map[string]any{"items": []any{map[string]any{"code": "1000", "description": "Zürich"}}}}, nil
	case "Tags.get":
		return sdk.Response{Payload: tagservice.EntityTags{Schema: tagSchema, EntityID: "p1",
			Values: []tagservice.Assignment{{Tag: "RISK", CompanyCode: "*", Value: tagservice.Value{Option: ptr("HIGH")}, ValidFrom: "2026-01-01"},
				{Tag: "LIMIT", CompanyCode: "1000", Value: tagservice.Value{Amount: ptr("5000.00"), Currency: ptr("CHF")}, ValidFrom: "2026-01-01"},
				{Tag: "OBJ", CompanyCode: "*", Value: tagservice.Value{Ref: ptr("a1")}, RefLabel: "Zürich", ValidFrom: "2026-01-01"}},
			State: tagservice.State{Visible: map[string]bool{"RISK": true, "LIMIT": true, "AUDIT": true, "OBJ": true}, Required: map[string]bool{"RISK": true}}}}, nil
	case "Tags.schema":
		return sdk.Response{Payload: tagSchema}, nil
	case "Tags.validate":
		in := req.Payload.(tagservice.SetRequest)
		visible := in.Values["RISK"] != nil && *in.Values["RISK"].Option == "HIGH"
		return sdk.Response{Payload: map[string]any{"violations": []any{},
			"state": tagservice.State{Visible: map[string]bool{"RISK": true, "LIMIT": visible, "AUDIT": true, "OBJ": true}, Required: map[string]bool{"RISK": true}}}}, nil
	case "Tags.set":
		return sdk.Response{Payload: tagservice.EntityTags{Schema: tagSchema, EntityID: "p1",
			State: tagservice.State{Visible: map[string]bool{"RISK": true, "LIMIT": false, "AUDIT": true}}}}, nil
	}
	return h.fakeHost.Handle(ctx, req)
}

func ptr(s string) *string { return &s }

func newTagServer(t *testing.T, perms ...string) *server {
	t.Helper()
	views, err := newRenderer("")
	if err != nil {
		t.Fatal(err)
	}
	ids := newMemIdentity()
	ids.add("tester", testPassword, "", perms...)
	auth := newAuthService(ids, newMemStore(), time.Hour)
	_, token, _, err := auth.Login(context.Background(), "tester", testPassword, "test")
	if err != nil {
		t.Fatal(err)
	}
	s := newServer(tagsHost{&fakeHost{fail: map[string]error{}}}, views, settings{Title: "CoreMesh"}, auth)
	testTokens.Store(s, token)
	return s
}

func TestTagEditorRender(t *testing.T) {
	s := newTagServer(t, "*.*")
	b := do(s, "GET", "/tags/Partner/p%2F1?effectiveDate=2026-10-06", nil, true).Body.String()
	mustContain(t, b,
		`<legend>Risiko</legend>`,
		`<option value="HIGH" selected>Hoch</option>`,                                  // Auswahlwert
		`name="v.LIMIT.amount" value="5000.00"`, `name="v.LIMIT.currency" value="CHF"`, // Betrag + Währung
		`<small class="badge">BK 1000</small>`,           // Geltungsbereich Buchungskreis
		`<small class="badge inactive">veraltet</small>`, // veralteter Tag: nur lesbar
		`name="v.AUDIT" value="" disabled`,
		`hx-post="/tags/Partner/p%2F1/preview"`,
		`<option value="1000" >1000 Zürich</option>`,
		`name="validFrom" value="2026-10-06"`, ">Tags speichern<",
	)

	// Ohne Recht zum Schreiben: alles schreibgeschützt, kein Speichern.
	ro := newTagServer(t, "Partner.Item", "Tags.get")
	b = do(ro, "GET", "/tags/Partner/p1", nil, true).Body.String()
	mustContain(t, b, `name="v.RISK" disabled`)
	mustNotContain(t, b, "Tags speichern")
}

func TestTagEditorPreviewAndSave(t *testing.T) {
	s := newTagServer(t, "*.*")
	form := url.Values{"effectiveDate": {"2026-10-06"}, "validFrom": {"2026-11-01"}, "companyCode": {"1000"},
		"v.RISK": {"LOW"}, "v.LIMIT.amount": {"1000,50"}, "v.LIMIT.currency": {"eur"}}

	// Vorschau: RISK=LOW → LIMIT ausgeblendet (SHOW_IF).
	b := do(s, "POST", "/tags/Partner/p1/preview", form, true).Body.String()
	mustNotContain(t, b, `name="v.LIMIT.amount"`)
	mustContain(t, b, `<option value="LOW" selected>Niedrig</option>`)

	// Speichern: ausgeblendetes LIMIT wird beendet (nil), Komma wird Punkt, Währung groß.
	h := s.host.(tagsHost)
	w := do(s, "POST", "/tags/Partner/p1", form, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Tags gespeichert.") {
		t.Fatalf("Speichern: %d %s", w.Code, w.Body.String())
	}
	req := h.find("set").Payload.(tagservice.SetRequest)
	if req.ValidFrom != "2026-11-01" || req.CompanyCode != "1000" || *req.Values["RISK"].Option != "LOW" || req.Values["LIMIT"] != nil {
		t.Fatalf("SetRequest: %+v", req)
	}
	if _, sent := req.Values["AUDIT"]; sent {
		t.Fatal("veralteter Tag gesendet")
	}
	form.Set("v.RISK", "HIGH")
	do(s, "POST", "/tags/Partner/p1", form, true)
	if v := h.find("set").Payload.(tagservice.SetRequest).Values["LIMIT"]; v == nil || *v.Amount != "1000.50" || *v.Currency != "EUR" {
		t.Fatalf("Betrag: %+v", v)
	}
}

// TestTagSection: Abschnitt mit Tags lädt den Editor über den fachlichen Schlüssel.
func TestTagSection(t *testing.T) {
	v := view{objectCtx: newObjectCtx("crm", "Customer", mdDefs["Customer"]), Record: record{"_id": "c1|2026-01-01", "id": "c1"}}
	v.Def.Sections = append(v.Def.Sections, mdDefs["Customer"].Sections[0])
	v.Def.Sections[len(v.Def.Sections)-1].Key, v.Def.Sections[len(v.Def.Sections)-1].Fields, v.Def.Sections[len(v.Def.Sections)-1].Tags = "tags", nil, true
	for _, s := range v.Sections() {
		if s.Tags && s.URL != "/tags/Customer/c1" {
			t.Fatalf("URL: %s", s.URL)
		}
	}
}
