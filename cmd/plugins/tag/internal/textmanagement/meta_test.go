package textmanagement

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// texts: Übersetzungsschlüssel des Moduls textmanagement mit dem deutschen Text.
func texts(t *testing.T) map[string]string {
	e := setup(t)
	resp, err := e.p.Handle(e.ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	if err != nil {
		t.Fatal(err)
	}
	d := resp.Payload.(metamodel.DescribeResponse)
	out := map[string]string{}
	add := func(k, v string) {
		if strings.HasPrefix(k, Name+".") {
			out[k] = v
		}
	}
	for _, o := range d.Objects {
		add(o.TitleKey, o.Title)
		for _, f := range o.Fields {
			add(f.LabelKey, f.Label)
			for _, opt := range f.Options {
				add(opt.LabelKey, opt.Label)
			}
		}
		for _, s := range o.Sections {
			add(s.TitleKey, s.Title)
		}
	}
	for _, m := range d.Modules {
		add(m.TitleKey, m.Title)
		add(m.DescriptionKey, m.Description)
		for _, o := range m.Objects {
			add(o.SectionKey, o.Section)
		}
	}
	return out
}

// TestTranslations: jeder Text des Moduls hat de, en und zh-CN; TXT_DUMP_I18N=1 schreibt de.json.
func TestTranslations(t *testing.T) {
	want := texts(t)
	if os.Getenv("TXT_DUMP_I18N") != "" {
		b, _ := json.MarshalIndent(want, "", "  ")
		if err := os.WriteFile("i18n/de.json", append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, loc := range []string{"de", "en", "zh-CN"} {
		for k := range want {
			if _, ok := translations[loc][k]; !ok {
				t.Errorf("%s: %s fehlt", loc, k)
			}
		}
	}
}
