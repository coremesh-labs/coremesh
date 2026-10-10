package main

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Dateien von Datensätzen (SectionDefinition.Viewer, fester Link):
//
//	GET /file/{object}/{id}             Inhalt eingebettet (inline)
//	GET /file/{object}/{id}?download=1  als Download
//
// Der Inhalt kommt aus der Action metamodel.ContentAction des Objects – mit
// den Rechten des Benutzers; das Modul prüft wie bei get.

// inlineTypes: MIME-Typen, die der Browser eingebettet zeigen darf. Alles andere
// (HTML, SVG, Skripte …) geht nur als Download – kein fremdes Skript im
// Ursprung der Anwendung.
var inlineTypes = map[string]bool{
	"application/pdf": true, "image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"image/tiff": true, "image/bmp": true, "text/plain": true,
}

func (s *server) fileContent(w http.ResponseWriter, r *http.Request) {
	object, id := r.PathValue("object"), r.PathValue("id")
	resp, err := s.call(r, object, metamodel.ContentAction, map[string]any{"id": id})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var c metamodel.Content
	if err := sdk.Decode(resp.Payload, &c); err != nil {
		s.fail(w, r, err)
		return
	}
	b, err := base64.StdEncoding.DecodeString(c.Content)
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: Inhalt ist nicht Base64-kodiert: %v", sdk.ErrFailedPrecondition, err))
		return
	}
	typ, _, _ := mime.ParseMediaType(c.ContentType)
	if typ == "" {
		typ = http.DetectContentType(b)
		typ, _, _ = mime.ParseMediaType(typ)
	}
	disposition := "inline"
	if r.URL.Query().Get("download") != "" || !inlineTypes[typ] {
		disposition = "attachment"
		if !inlineTypes[typ] {
			typ = "application/octet-stream"
		}
	}
	if typ == "text/plain" {
		typ += "; charset=utf-8"
	}
	h := w.Header()
	h.Set("Content-Type", typ)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, no-store")
	if name := strings.TrimSpace(c.FileName); name != "" {
		h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	} else {
		h.Set("Content-Disposition", disposition)
	}
	_, _ = w.Write(b)
}

// FileURL: fester Link auf den Inhalt eines Datensatzes.
func fileURL(object, id string) string { return "/file/" + object + "/" + pathEscape(id) }
