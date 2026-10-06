package metamodel

import "testing"

func TestNormalizeLocale(t *testing.T) {
	for in, want := range map[string]string{
		"de": "de", "de-CH": "de", "DE_at": "de", "en-US": "en", "en": "en",
		"zh": "zh-CN", "zh-CN": "zh-CN", "zh-Hans": "zh-CN", "zh-Hans-CN": "zh-CN", "zh-SG": "zh-CN",
		"zh-TW": "", "zh-Hant": "", "zh-HK": "", "fr": "", "": "",
	} {
		if got := NormalizeLocale(in); got != want {
			t.Errorf("%q → %q, erwartet %q", in, got, want)
		}
	}
}

func TestLocalize(t *testing.T) {
	tr := Translations{
		LocaleDE: {"m.X.title": "Partner", "m.X.fields.name": "Name", "m.X.options.kind.a": "Art A"},
		LocaleZH: {"m.X.title": "业务伙伴", "m.X.actions.end": "结束", "m.X.sections.base": "主数据"},
	}
	d := ObjectDefinition{Name: "X", Title: "Original", TitleKey: "m.X.title",
		Fields: []FieldDefinition{{Key: "name", Label: "Name?", LabelKey: "m.X.fields.name", Type: TypeSelect,
			Options: []Option{{Value: "a", Label: "a?", LabelKey: "m.X.options.kind.a"}}}},
		Actions:  []ActionConfig{{Name: "end", Label: "Beenden", LabelKey: "m.X.actions.end", Confirm: "Sicher?", ConfirmKey: "m.X.actions.end.confirm"}},
		Sections: []SectionDefinition{{Key: "base", Title: "Stammdaten", TitleKey: "m.X.sections.base", Fields: []string{"name"}}},
	}
	zh := d.Localize(tr, LocaleZH)
	if zh.Title != "业务伙伴" || zh.Actions[0].Label != "结束" || zh.Sections[0].Title != "主数据" {
		t.Fatalf("zh: %+v", zh)
	}
	// Fehlt der Schlüssel in zh: Rückfall auf de, sonst auf den Originaltext.
	if zh.Fields[0].Label != "Name" || zh.Fields[0].Options[0].Label != "Art A" || zh.Actions[0].Confirm != "Sicher?" {
		t.Fatalf("Rückfall: %+v", zh)
	}
	// Das Original bleibt unverändert.
	if d.Title != "Original" || d.Fields[0].Label != "Name?" || d.Fields[0].Options[0].Label != "a?" {
		t.Fatalf("Original verändert: %+v", d)
	}
	if ValidModuleName("i18n") || ValidModuleName("user") {
		t.Fatal("reservierte Modulnamen")
	}
}
