package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

func TestGoneAfterAction(t *testing.T) {
	s := &server{}
	nf := fmt.Errorf("%w: Posteingang 2000|x", sdk.ErrNotFound)
	req := func(target, query string) *http.Request {
		r := httptest.NewRequest("GET", "/m/mail/MailMessage/x"+query, nil)
		r.Header.Set("HX-Request", "true")
		if target != "" {
			r.Header.Set("HX-Target", target)
		}
		r.SetPathValue("object", "MailMessage")
		return r
	}
	w := httptest.NewRecorder()
	if !s.goneAfterAction(w, req("detail", ""), nf) || !strings.Contains(w.Header().Get("HX-Location"), "MailMessage") {
		t.Fatalf("Detail: %v", w.Header())
	}
	w = httptest.NewRecorder()
	if !s.goneAfterAction(w, req("", "?view=row"), nf) || w.Body.Len() != 0 {
		t.Fatal("Zeile")
	}
	if s.goneAfterAction(httptest.NewRecorder(), req("main-content", ""), nf) {
		t.Fatal("normaler Aufruf darf nicht umleiten")
	}
	if s.goneAfterAction(httptest.NewRecorder(), req("detail", ""), sdk.ErrPermissionDenied) {
		t.Fatal("nur bei nicht gefunden")
	}
}
