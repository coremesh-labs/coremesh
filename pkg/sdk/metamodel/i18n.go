package metamodel

import (
	"slices"
	"strings"
)

// Mehrsprachigkeit (i18n).
//
// Texte in Metamodellen haben zwei Formen: den Text selbst (Title, Label, …,
// Rückfall) und einen Übersetzungsschlüssel (TitleKey, LabelKey, …). Ein
// Modul liefert seine Übersetzungen mit Catalog.Describe
// (DescribeResponse.Translations); Schlüssel beginnen mit dem Modulnamen,
// z. B. "businesspartner.BusinessPartner.fields.name1". Framework-Texte
// (Speichern, Abbrechen, …) gehören dem Frontend.

// Unterstützte Sprachen (BCP 47).
const (
	LocaleDE = "de"    // Deutsch
	LocaleEN = "en"    // Englisch
	LocaleZH = "zh-CN" // Chinesisch (Festland, vereinfachte Schrift)
)

// Locales sind alle unterstützten Sprachen; die erste ist der Standard.
var Locales = []string{LocaleDE, LocaleEN, LocaleZH}

// Translations sind Übersetzungen je Sprache: Locale → Schlüssel → Text.
type Translations map[string]map[string]string

// NormalizeLocale bildet ein Sprach-Tag auf eine unterstützte Sprache ab
// ("" = nicht unterstützt): de-CH → de, en-US → en, zh / zh-CN / zh-Hans /
// zh-SG → zh-CN. Traditionelles Chinesisch (zh-TW, zh-HK, zh-Hant) wird
// nicht unterstützt.
func NormalizeLocale(tag string) string {
	t := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(tag, "_", "-")))
	if t == "" {
		return ""
	}
	primary, rest, _ := strings.Cut(t, "-")
	switch primary {
	case "de":
		return LocaleDE
	case "en":
		return LocaleEN
	case "zh":
		for _, part := range strings.Split(rest, "-") {
			if part == "tw" || part == "hk" || part == "mo" || part == "hant" {
				return ""
			}
		}
		return LocaleZH
	}
	return ""
}

// Lookup schlägt einen Text nach: zuerst in der gewünschten Sprache, dann
// im Standard (de); fehlt der Schlüssel, liefert es fallback.
func (t Translations) Lookup(locale, key, fallback string) string {
	if key == "" {
		return fallback
	}
	for _, l := range []string{locale, LocaleDE} {
		if s, ok := t[l][key]; ok && s != "" {
			return s
		}
	}
	return fallback
}

// Localize liefert eine Kopie der Definition mit den Texten der Sprache.
// Fehlende Übersetzungen behalten den Originaltext.
func (d ObjectDefinition) Localize(t Translations, locale string) ObjectDefinition {
	out := d
	out.Title = t.Lookup(locale, d.TitleKey, d.Title)
	out.Fields = make([]FieldDefinition, len(d.Fields))
	for i, f := range d.Fields {
		f.Label = t.Lookup(locale, f.LabelKey, f.Label)
		if len(f.Options) > 0 {
			opts := make([]Option, len(f.Options))
			for j, o := range f.Options {
				o.Label = t.Lookup(locale, o.LabelKey, o.Label)
				opts[j] = o
			}
			f.Options = opts
		}
		out.Fields[i] = f
	}
	out.Actions = make([]ActionConfig, len(d.Actions))
	for i, a := range d.Actions {
		a.Label = t.Lookup(locale, a.LabelKey, a.Label)
		a.Confirm = t.Lookup(locale, a.ConfirmKey, a.Confirm)
		out.Actions[i] = a
	}
	out.Sections = make([]SectionDefinition, len(d.Sections))
	for i, s := range d.Sections {
		s.Title = t.Lookup(locale, s.TitleKey, s.Title)
		out.Sections[i] = s
	}
	return out
}

// reservedModules sind Pfade der API, die kein Modul belegen darf
// (/api/v1/i18n/{lang}, /api/v1/user/profile).
var reservedModules = map[string]bool{"i18n": true, "user": true}

// Schlüssel-Konvention für Übersetzungen (WithKeys setzt fehlende Schlüssel):
//
//	<modul>.module.title | .description          Modul
//	<modul>.navigation.<gruppe>                  Gruppe in der Navigation (ModuleObject.Section)
//	<modul>.<Object>.title                       Object
//	<modul>.<Object>.fields.<feld>               Feld
//	<modul>.<Object>.options.<feld>.<wert>       Auswahlwert
//	<modul>.<Object>.actions.<action>[.confirm]  Action
//	<modul>.<Object>.sections.<abschnitt>        Abschnitt der Detailansicht
//
// Für Actions der Kinds list, item, create, update, expire und deactivate
// bringt das Frontend Standardtexte mit; ein Modul übersetzt sie nur, wenn es
// abweichende Texte will.

// WithKeys liefert eine Kopie der Definition, in der fehlende
// Übersetzungsschlüssel nach der Konvention gesetzt sind.
func WithKeys(module string, d ObjectDefinition) ObjectDefinition {
	p := module + "." + d.Name
	d.TitleKey = orKey(d.TitleKey, p+".title")
	d.Fields = slices.Clone(d.Fields)
	for i, f := range d.Fields {
		f.LabelKey = orKey(f.LabelKey, p+".fields."+f.Key)
		if len(f.Options) > 0 {
			f.Options = slices.Clone(f.Options)
			for j, o := range f.Options {
				o.LabelKey = orKey(o.LabelKey, p+".options."+f.Key+"."+o.Value)
				f.Options[j] = o
			}
		}
		d.Fields[i] = f
	}
	d.Actions = slices.Clone(d.Actions)
	for i, a := range d.Actions {
		a.LabelKey = orKey(a.LabelKey, p+".actions."+a.Name)
		if a.Confirm != "" {
			a.ConfirmKey = orKey(a.ConfirmKey, p+".actions."+a.Name+".confirm")
		}
		d.Actions[i] = a
	}
	d.Sections = slices.Clone(d.Sections)
	for i, s := range d.Sections {
		s.TitleKey = orKey(s.TitleKey, p+".sections."+s.Key)
		d.Sections[i] = s
	}
	return d
}

// ModuleKeys setzt fehlende Schlüssel einer Modul-Definition.
func ModuleKeys(m ModuleDefinition) ModuleDefinition {
	m.TitleKey = orKey(m.TitleKey, m.Name+".module.title")
	m.DescriptionKey = orKey(m.DescriptionKey, m.Name+".module.description")
	m.Objects = slices.Clone(m.Objects)
	for i, o := range m.Objects {
		if o.Section != "" {
			o.SectionKey = orKey(o.SectionKey, NavigationKey(m.Name, o.Section))
		}
		m.Objects[i] = o
	}
	return m
}

// NavigationKey ist der Schlüssel einer Navigationsgruppe:
// NavigationKey("businesspartner", "Partnerdaten") == "businesspartner.navigation.partnerdaten".
func NavigationKey(module, section string) string { return module + ".navigation." + slug(section) }

func orKey(key, def string) string {
	if key != "" {
		return key
	}
	return def
}

// slug: Kleinbuchstaben, Ziffern und "_" – "Belege & Verträge" → "belege_vertraege".
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == 'ä':
			b.WriteString("ae")
		case r == 'ö':
			b.WriteString("oe")
		case r == 'ü':
			b.WriteString("ue")
		case r == 'ß':
			b.WriteString("ss")
		default:
			b.WriteRune('_')
		}
	}
	return strings.Join(strings.FieldsFunc(b.String(), func(r rune) bool { return r == '_' }), "_")
}
