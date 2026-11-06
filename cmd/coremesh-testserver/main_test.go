package main

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testApp(t *testing.T) *app {
	t.Helper()
	a, err := newApp(t.TempDir(), "faust-pc.fritz.box", "Test", false, 1<<20, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// do: Anfrage an den HTTPS-Teil, von außen (LAN) bzw. vom Rechner selbst.
func do(a *app, method, target string, body io.Reader, lan bool, hdr ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, body)
	r.TLS = &tls.ConnectionState{}
	r.RemoteAddr = "127.0.0.1:5000"
	if lan {
		r.RemoteAddr = "192.168.178.60:5000"
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	a.httpsHandler().ServeHTTP(w, r)
	return w
}

func TestUploadDownload(t *testing.T) {
	a := testApp(t)
	// aus dem Netz ohne Schlüssel: verboten
	if w := do(a, "PUT", "/files/i/start1", strings.NewReader("x"), true); w.Code != 403 {
		t.Fatalf("ohne Schlüssel: %d", w.Code)
	}
	w := do(a, "PUT", "/files/i/start1", strings.NewReader("hallo"), true, "Authorization", "Bearer "+a.token)
	if w.Code != 201 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	var res putJSON
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Size != 5 || res.Sha256 != "d3751d33f9cd5049c4af2b462735457e4d3baf130bcbb87f389e349fbaeb20b9" {
		t.Fatalf("Ergebnis %+v", res)
	}
	// lesen darf jeder; i/ frisch, o/ dauerhaft
	w = do(a, "GET", "/files/i/start1", nil, true, "Origin", "https://pwa.example")
	if w.Body.String() != "hallo" || w.Header().Get("Cache-Control") != "no-cache" || w.Header().Get("Access-Control-Allow-Origin") != "https://pwa.example" {
		t.Fatalf("GET: %q %v", w.Body, w.Header())
	}
	do(a, "PUT", "/files/o/abc", strings.NewReader("z"), false)
	if w = do(a, "GET", "/files/o/abc", nil, true); !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("o/: %v", w.Header())
	}
	// Liste als JSON, auch rekursiv
	w = do(a, "GET", "/files/?format=json&recursive=1", nil, true)
	var l listJSON
	json.Unmarshal(w.Body.Bytes(), &l)
	if len(l.Entries) != 2 || l.Entries[0].Path != "i/start1" {
		t.Fatalf("Liste %s", w.Body)
	}
	// Verzeichnis als Seite
	if w = do(a, "GET", "/files/i/", nil, true); !strings.Contains(w.Body.String(), "start1") {
		t.Fatalf("Seite: %s", w.Body)
	}
	// Bereich
	if w = do(a, "GET", "/files/i/start1", nil, true, "Range", "bytes=1-2"); w.Code != 206 || w.Body.String() != "al" {
		t.Fatalf("Range: %d %q", w.Code, w.Body)
	}
	// löschen
	if w = do(a, "DELETE", "/files/i/start1", nil, true, "X-Upload-Token", a.token); w.Code != 204 {
		t.Fatalf("DELETE: %d", w.Code)
	}
	if w = do(a, "GET", "/files/i/start1", nil, true); w.Code != 404 {
		t.Fatalf("nach DELETE: %d", w.Code)
	}
	// Preflight
	if w = do(a, "OPTIONS", "/files/i/x", nil, true, "Origin", "https://pwa.example"); w.Code != 204 {
		t.Fatalf("OPTIONS: %d", w.Code)
	}
}

func TestPathEscape(t *testing.T) {
	a := testApp(t)
	os.WriteFile(filepath.Join(a.dir, "geheim"), []byte("x"), 0o600)
	for _, p := range []string{"/files/../geheim", "/files/a/../../geheim", "/files/..%2fgeheim"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.URL.Path = strings.ReplaceAll(p, "%2f", "/")
		r.TLS, r.RemoteAddr = &tls.ConnectionState{}, "127.0.0.1:1"
		w := httptest.NewRecorder()
		a.filesHandler().ServeHTTP(w, r)
		if w.Code == 200 {
			t.Fatalf("%s erreichbar", p)
		}
	}
	// Symlink nach außen hält os.Root auf
	os.Symlink(filepath.Join(a.dir, "geheim"), filepath.Join(a.filesDir, "link"))
	if w := do(a, "GET", "/files/link", nil, false); w.Code == 200 {
		t.Fatal("Symlink nach außen gelesen")
	}
}

func zipOf(t *testing.T, files map[string]string) []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, c := range files {
		w, _ := z.Create(n)
		w.Write([]byte(c))
	}
	z.Close()
	return b.Bytes()
}

func form(t *testing.T, fields map[string]string, name string, data []byte) (io.Reader, string) {
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	for k, v := range fields {
		m.WriteField(k, v)
	}
	w, _ := m.CreateFormFile("file", name)
	w.Write(data)
	m.Close()
	return &b, m.FormDataContentType()
}

func TestFormUploadUnzipAndApp(t *testing.T) {
	a := testApp(t)
	body, ct := form(t, map[string]string{"unzip": "1"}, "ablage.zip", zipOf(t, map[string]string{"i/a": "1", "o/b": "2", "../boese": "x"}))
	w := do(a, "POST", "/files/pruef/", body, false, "Content-Type", ct)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "2 Dateien entpackt, 1 übersprungen") {
		t.Fatalf("unzip: %d %s", w.Code, w.Body)
	}
	if b, _ := os.ReadFile(filepath.Join(a.filesDir, "pruef", "o", "b")); string(b) != "2" {
		t.Fatal("o/b fehlt")
	}
	// App als zip mit Ordner dist/
	body, ct = form(t, nil, "app.zip", zipOf(t, map[string]string{"dist/index.html": "<h1>PWA</h1>", "dist/sw.js": "//"}))
	if w = do(a, "POST", "/app/", body, false, "Content-Type", ct); w.Code != 303 {
		t.Fatalf("App: %d %s", w.Code, w.Body)
	}
	if w = do(a, "GET", "/app/", nil, true); w.Body.String() != "<h1>PWA</h1>" {
		t.Fatalf("App /: %s", w.Body)
	}
	if w = do(a, "GET", "/app/zaehler/16", nil, true); w.Body.String() != "<h1>PWA</h1>" {
		t.Fatalf("App Navigation: %s", w.Body)
	}
	if w = do(a, "GET", "/app/sw.js", nil, true); !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("sw.js: %v", w.Header())
	}
}

func TestPagesAndToken(t *testing.T) {
	a := testApp(t)
	if w := do(a, "GET", "/", nil, false); !strings.Contains(w.Body.String(), "<svg") || !strings.Contains(w.Body.String(), a.token) {
		t.Fatal("Startseite am Rechner ohne QR bzw. Schlüssel")
	}
	if w := do(a, "GET", "/", nil, true); strings.Contains(w.Body.String(), a.token) {
		t.Fatal("Schlüssel im Netz angezeigt")
	}
	w := do(a, "GET", "/?token="+a.token, nil, true)
	if w.Code != 303 || !strings.Contains(w.Header().Get("Set-Cookie"), tokenCookie) {
		t.Fatalf("Token-Link: %d %v", w.Code, w.Header())
	}
	if w = do(a, "PUT", "/files/x", strings.NewReader("1"), true, "Cookie", tokenCookie+"="+a.token); w.Code != 201 {
		t.Fatalf("mit Cookie: %d", w.Code)
	}
	if w = do(a, "GET", "/ca.crt", nil, true); w.Header().Get("Content-Type") != "application/x-x509-ca-cert" || !bytes.Equal(w.Body.Bytes(), a.ca.DER) {
		t.Fatal("ca.crt")
	}
	// HTTP: vom Handy Einrichtung, sonst Umleitung auf HTTPS
	r := httptest.NewRequest("GET", "http://192.168.178.52:8080/files/x", nil)
	r.RemoteAddr = "192.168.178.60:1"
	rw := httptest.NewRecorder()
	a.httpHandler().ServeHTTP(rw, r)
	if rw.Code != http.StatusTemporaryRedirect || rw.Header().Get("Location") != "https://192.168.178.52:8443/files/x" {
		t.Fatalf("Umleitung: %d %s", rw.Code, rw.Header().Get("Location"))
	}
	r = httptest.NewRequest("GET", "http://192.168.178.52:8080/", nil)
	r.RemoteAddr = "192.168.178.60:1"
	rw = httptest.NewRecorder()
	a.httpHandler().ServeHTTP(rw, r)
	if !strings.Contains(rw.Body.String(), "Handy einrichten") {
		t.Fatal("Einrichtungsseite")
	}
}
