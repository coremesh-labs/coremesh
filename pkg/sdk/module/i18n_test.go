package module

import (
	"context"
	"strings"
	"testing"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

type translated struct {
	*fakeModule
	tr metamodel.Translations
}

func (t translated) Translations() metamodel.Translations { return t.tr }

func TestTranslationKeysAndDescribe(t *testing.T) {
	var ev []string
	m := simple("sales", &ev, "Order")
	m.routes = func(r *Router) {
		d := def("Order")
		d.Fields[0].Type, d.Fields[0].Options = metamodel.TypeSelect, []metamodel.Option{{Value: "a", Label: "A"}}
		d.Actions[0].Confirm = "Sicher?"
		d.Sections = []metamodel.SectionDefinition{{Key: "base", Title: "Basis", Fields: []string{"name"}}}
		r.Object("Order").Section("Belege & Verträge").Describe(d).Handle("list", echo)
	}
	p := NewPlugin(Info{Name: "erp", Version: "1"}, translated{m, metamodel.Translations{
		metamodel.LocaleZH: {"sales.Order.title": "订单", "sales.module.title": "销售"},
	}})
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	resp, _ := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	d := resp.Payload.(metamodel.DescribeResponse)
	o := d.Objects[0]
	for got, want := range map[string]string{
		o.TitleKey:                         "sales.Order.title",
		o.Fields[0].LabelKey:               "sales.Order.fields.name",
		o.Fields[0].Options[0].LabelKey:    "sales.Order.options.name.a",
		o.Actions[0].LabelKey:              "sales.Order.actions.list",
		o.Actions[0].ConfirmKey:            "sales.Order.actions.list.confirm",
		o.Sections[0].TitleKey:             "sales.Order.sections.base",
		d.Modules[0].TitleKey:              "sales.module.title",
		d.Modules[0].Objects[0].SectionKey: "sales.navigation.belege_vertraege",
	} {
		if got != want {
			t.Errorf("Schlüssel %q, erwartet %q", got, want)
		}
	}
	if d.Translations[metamodel.LocaleZH]["sales.Order.title"] != "订单" {
		t.Fatalf("Übersetzungen: %v", d.Translations)
	}

	// Fremder Namensraum und unbekannte Sprache werden abgelehnt.
	bad := NewPlugin(Info{Name: "erp", Version: "1"}, translated{simple("sales", &ev, "Order"), metamodel.Translations{
		metamodel.LocaleEN: {"businesspartner.X.title": "fremd"}, "fr": {"sales.x": "y"},
	}})
	if err := bad.Err(); err == nil || !strings.Contains(err.Error(), "Namensraum") || !strings.Contains(err.Error(), `"fr"`) {
		t.Fatalf("Prüfung der Übersetzungen: %v", err)
	}
}
