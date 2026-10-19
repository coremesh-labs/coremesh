package main

import "testing"

func TestDownloadsOf(t *testing.T) {
	pdf := map[string]any{"content": "JVBERi0=", "content_type": "application/pdf", "file_name": "Bericht.pdf"}
	csv := map[string]any{"content": "YTtiDQo=", "content_type": "text/csv; charset=utf-8", "file_name": "../x/bericht.csv"}
	got := downloadsOf(map[string]any{"download": pdf, "downloads": []any{csv, map[string]any{"content": "kein base64!"}}})
	if len(got) != 2 || got[0].FileName != "Bericht.pdf" || got[1].FileName != "bericht.csv" {
		t.Fatalf("Dateien: %+v", got)
	}
	if downloadsOf("text") != nil || len(downloadsOf(map[string]any{})) != 0 {
		t.Fatal("ohne Dateien")
	}
}
