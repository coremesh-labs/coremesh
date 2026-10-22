package backup

import (
	"embed"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// schemaHCL: Einstellungen (ein Datensatz) und Protokoll der Snapshots (Präfix backup__).
const schemaHCL = `
schema "main" {}

table "backup__setting" {
  schema = schema.main
  column "id" {
    type = text
  }
  # Verzeichnis der Snapshots (relativ zum Arbeitsverzeichnis des Hosts oder absolut)
  column "directory" {
    type = text
  }
  column "interval_hours" {
    type    = integer
    default = 24
  }
  # höchstens so viele erfolgreiche Snapshots (0 = beliebig viele)
  column "keep_count" {
    type    = integer
    default = 14
  }
  # Verweildauer in Tagen (0 = unbegrenzt); der jüngste Snapshot bleibt immer
  column "keep_days" {
    type    = integer
    default = 30
  }
  # 1 = nur sichern, wenn sich seit dem letzten Snapshot etwas geändert hat
  column "only_if_changed" {
    type    = integer
    default = 1
  }
  # Tabellen ohne Bedeutung für „geändert“ (Muster mit *, Komma-Liste)
  column "ignore_tables" {
    type    = text
    default = ""
  }
  column "active" {
    type    = integer
    default = 1
  }
  # Stand der Datenbank nach dem letzten Snapshot (Änderungen erkennen)
  column "change_mark" {
    type = text
    null = true
  }
  column "last_run_at" {
    type = text
    null = true
  }
  column "updated_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
}

table "backup__snapshot" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "setting_id" {
    type = text
  }
  column "file_name" {
    type = text
  }
  column "path" {
    type = text
  }
  column "created_at" {
    type = text
  }
  column "size_bytes" {
    type    = bigint
    default = 0
  }
  column "duration_ms" {
    type    = bigint
    default = 0
  }
  # AUTO | MANUAL
  column "trigger_kind" {
    type = text
  }
  column "user_id" {
    type = text
    null = true
  }
  # OK | FAILED | REMOVED
  column "status" {
    type = text
  }
  column "message" {
    type = text
    null = true
  }
  primary_key {
    columns = [column.id]
  }
}
`

var (
	statusOptions = []metamodel.Option{{Value: statusOK, Label: "gesichert"}, {Value: statusError, Label: "fehlgeschlagen"},
		{Value: statusGone, Label: "gelöscht (Verweildauer, Höchstanzahl)"}}
	triggerOptions = []metamodel.Option{{Value: trigAuto, Label: "automatisch"}, {Value: trigManual, Label: "von Hand"}}
	lookupUser     = &metamodel.Lookup{Object: "User", ValueField: "id", LabelFields: []string{"username", "display_name"}}

	settingDef = metamodel.ObjectDefinition{
		Name: "BackupSetting", Title: "Datensicherung", Icon: "icon-database", TitleField: "directory",
		Fields: []metamodel.FieldDefinition{
			{Key: "id", Label: "ID", Type: metamodel.TypeText},
			{Key: "database", Label: "Datenbank", Type: metamodel.TypeText, Listable: true},
			{Key: "directory", Label: "Verzeichnis der Snapshots (relativ zum Arbeitsverzeichnis des Servers oder absolut)", Short: "Verzeichnis",
				Type: metamodel.TypeText, Required: true, Listable: true, Editable: true, Wide: true},
			{Key: "directory_abs", Label: "Verzeichnis (vollständiger Pfad)", Type: metamodel.TypeText},
			{Key: "interval_hours", Label: "Abstand (Stunden)", Type: metamodel.TypeNumber, Required: true, Listable: true, Editable: true},
			{Key: "only_if_changed", Label: "Nur sichern, wenn seit dem letzten Snapshot etwas geändert wurde (ein Snapshot je Arbeitstag)",
				Short: "Nur bei Änderungen", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
			{Key: "ignore_tables", Label: "Tabellen ohne Bedeutung für „geändert“ (Muster mit *, durch Komma getrennt)", Short: "Ohne Tabellen",
				Type: metamodel.TypeText, Editable: true, Wide: true},
			{Key: "keep_days", Label: "Verweildauer (Tage; 0 = unbegrenzt)", Short: "Verweildauer", Type: metamodel.TypeNumber, Required: true, Listable: true, Editable: true},
			{Key: "keep_count", Label: "Höchstens so viele Snapshots (0 = beliebig viele)", Short: "Höchstens", Type: metamodel.TypeNumber, Required: true, Listable: true, Editable: true},
			{Key: "active", Label: "Automatisch sichern", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
			{Key: "last_run_at", Label: "Letzter Snapshot", Type: metamodel.TypeText, Listable: true},
			{Key: "last_file", Label: "Letzte Datei", Type: metamodel.TypeText},
			{Key: "last_status", Label: "Letztes Ergebnis", Type: metamodel.TypeSelect, Options: statusOptions, Listable: true},
			{Key: "last_message", Label: "Letzte Meldung", Type: metamodel.TypeText},
			{Key: "next_run_at", Label: "Nächster Snapshot", Type: metamodel.TypeText, Listable: true},
		},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
			{Name: "update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
			{Name: "run", Kind: metamodel.KindCustom, Label: "Jetzt sichern", Record: true, Confirm: "Jetzt einen Snapshot der Datenbank sichern?"},
			{Name: "compact", Kind: metamodel.KindCustom, Label: "Datenbank verdichten", Record: true,
				Confirm: "Freien Platz der Datenbank zurückgeben? Die Datenbank ist dabei kurz gesperrt."},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "snapshots", Title: "Snapshots", Relation: &metamodel.Relation{Object: "BackupSnapshot", ForeignKey: "setting_id",
				Columns: []string{"created_at", "file_name", "size", "duration_ms", "trigger_kind", "user_id", "status", "message"}}},
		},
	}
	snapshotDef = metamodel.ObjectDefinition{
		Name: "BackupSnapshot", Title: "Snapshots", Icon: "icon-list", TitleField: "file_name",
		Fields: []metamodel.FieldDefinition{
			{Key: "created_at", Label: "Zeitpunkt", Type: metamodel.TypeText, Listable: true},
			{Key: "file_name", Label: "Datei", Type: metamodel.TypeText, Listable: true},
			{Key: "path", Label: "Pfad", Type: metamodel.TypeText},
			{Key: "size", Label: "Größe", Type: metamodel.TypeText, Listable: true},
			{Key: "size_bytes", Label: "Größe (Bytes)", Type: metamodel.TypeNumber},
			{Key: "duration_ms", Label: "Dauer (ms)", Type: metamodel.TypeNumber, Listable: true},
			{Key: "trigger_kind", Label: "Auslöser", Type: metamodel.TypeSelect, Options: triggerOptions, Listable: true},
			{Key: "user_id", Label: "Benutzer", Type: metamodel.TypeText, Listable: true, Lookup: lookupUser},
			{Key: "status", Label: "Ergebnis", Type: metamodel.TypeSelect, Options: statusOptions, Listable: true},
			{Key: "message", Label: "Meldung", Type: metamodel.TypeText, Listable: true},
			{Key: "setting_id", Label: "Einstellung", Type: metamodel.TypeText},
		},
		Filters: []string{"status"},
		Actions: []metamodel.ActionConfig{
			{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
			{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		},
	}
	backupModule = metamodel.ModuleDefinition{
		Name: "backup", Title: "Datensicherung", Icon: "icon-database",
		Description: "Snapshots der Datenbank in einstellbarem Abstand (SQLite: VACUUM INTO, PostgreSQL: pg_dump)",
		Objects: []metamodel.ModuleObject{
			{Object: "BackupSetting", Section: "Datensicherung"},
			{Object: "BackupSnapshot", Section: "Protokoll"},
		},
		Customizing: []string{"BackupSetting"},
	}
)

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")
