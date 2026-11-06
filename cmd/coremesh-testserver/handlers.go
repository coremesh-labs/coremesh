package main

import (
	"archive/zip"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk/qrcode"
)

//go:embed page.html
var pageFS embed.FS

var page = template.Must(template.New("page.html").Funcs(template.FuncMap{
	"size": humanSize,
}).ParseFS(pageFS, "page.html"))

func init() {
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
	mime.AddExtensionType(".wasm", "application/wasm")
	mime.AddExtensionType(".mjs", "text/javascript")
}

const tokenCookie = "cm_upload_token"

// HTTP: Einrichtungsseite und CA-Zertifikat; alles andere geht auf HTTPS.
func (a *app) httpHandler() http.Handler {
	mux := http.NewServeMux()
	// am Rechner die Übersicht mit den QR-Codes, vom Handy die Einrichtung
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if isLoopback(r) && r.URL.Query().Get("setup") == "" {
			a.render(w, r, "home", nil)
			return
		}
		a.render(w, r, "setup", nil)
	})
	mux.HandleFunc("GET /ca.crt", a.caCert)
	mux.HandleFunc("GET /ca.pem", a.caCert)
	mux.HandleFunc("GET /api/info", a.info)
	// am Rechner selbst geht alles auch über HTTP (localhost braucht kein Zertifikat)
	files, appH := a.filesHandler(), a.appHandler()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if isLoopback(r) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/files/") || r.URL.Path == "/files":
				files.ServeHTTP(w, r)
				return
			case strings.HasPrefix(r.URL.Path, "/app/") || r.URL.Path == "/app":
				appH.ServeHTTP(w, r)
				return
			}
		}
		host, _, err := splitHost(r.Host)
		if err != nil {
			host = r.Host
		}
		http.Redirect(w, r, strings.TrimSuffix(a.secureURL(host), "/")+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	})
	return mux
}

// HTTPS: Startseite, Dateien, App.
func (a *app) httpsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if t := r.URL.Query().Get("token"); t != "" {
			if !a.tokenOK(t) {
				http.Error(w, "Upload-Schlüssel falsch", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: tokenCookie, Value: t, Path: "/", Secure: true, HttpOnly: true,
				SameSite: http.SameSiteStrictMode, Expires: time.Now().AddDate(1, 0, 0)})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		a.render(w, r, "home", nil)
	})
	mux.HandleFunc("GET /ca.crt", a.caCert)
	mux.HandleFunc("GET /ca.pem", a.caCert)
	mux.HandleFunc("GET /api/info", a.info)
	mux.Handle("/files/", a.filesHandler())
	mux.Handle("/files", http.RedirectHandler("/files/", http.StatusMovedPermanently))
	mux.Handle("/app/", a.appHandler())
	mux.Handle("/app", http.RedirectHandler("/app/", http.StatusMovedPermanently))
	return mux
}

func splitHost(hp string) (string, string, error) {
	if !strings.Contains(hp, ":") || strings.HasSuffix(hp, "]") {
		return strings.Trim(hp, "[]"), "", nil
	}
	i := strings.LastIndex(hp, ":")
	return strings.Trim(hp[:i], "[]"), hp[i+1:], nil
}

// caCert: /ca.crt als DER (Android installiert es so), /ca.pem als PEM.
func (a *app) caCert(w http.ResponseWriter, r *http.Request) {
	name := "CoreMesh-" + strings.ReplaceAll(a.label, " ", "-") + "-CA"
	if strings.HasSuffix(r.URL.Path, ".pem") {
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.pem"`)
		w.Write(a.ca.PEM())
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.crt"`)
	w.Write(a.ca.DER)
}

type infoJSON struct {
	CA          string    `json:"ca"`
	Fingerprint string    `json:"fingerprint"`
	CAUntil     time.Time `json:"ca_until"`
	Names       []string  `json:"names"`
	ServerUntil time.Time `json:"server_until"`
	Skipped     []string  `json:"skipped,omitempty"`
	SetupURL    string    `json:"setup_url"`
	SecureURL   string    `json:"secure_url"`
}

func (a *app) info(w http.ResponseWriter, r *http.Request) {
	s := a.snapshot()
	writeJSON(w, http.StatusOK, infoJSON{
		CA: a.ca.Cert.Subject.CommonName, Fingerprint: a.ca.Fingerprint(), CAUntil: a.ca.Cert.NotAfter,
		Names:       append(append([]string{}, s.server.Cert.DNSNames...), ipStrings(s.server.Cert.IPAddresses)...),
		ServerUntil: s.server.Cert.NotAfter, Skipped: s.server.Skipped,
		SetupURL: a.setupURL(), SecureURL: a.secureURL(""),
	})
}

// --- Seiten ---------------------------------------------------------------------

type qrView struct {
	Title, URL, Note string
	SVG              template.HTML
}

type entry struct {
	Name, Href, Path string
	Dir              bool
	Size             int64
	Modified         time.Time
}

type crumb struct{ Name, Href string }

type pageData struct {
	View                         string
	Secure, Loopback, Open       bool
	CanWrite                     bool
	CA, Fingerprint              string
	CAUntil, ServerUntil         time.Time
	Renewed                      time.Time
	Names, Skipped, Alternatives []string
	SetupURL, SecureURL          string
	Token                        string
	QR                           []qrView
	// Dateien
	Dir     string
	Crumbs  []crumb
	Entries []entry
	Message string
	// App
	AppReady bool
}

func qr(title, u, note string) qrView {
	c, err := qrcode.Encode(u, qrcode.M)
	if err != nil {
		return qrView{Title: title, URL: u, Note: note}
	}
	// SVG aus qrcode: nur Zahlen und feste Attribute – sicher einzubetten.
	return qrView{Title: title, URL: u, Note: note, SVG: template.HTML(c.SVG(4))}
}

func (a *app) render(w http.ResponseWriter, r *http.Request, view string, fill func(*pageData)) {
	s := a.snapshot()
	d := &pageData{
		View: view, Secure: r.TLS != nil, Loopback: isLoopback(r), Open: a.open, CanWrite: a.mayWrite(r),
		CA: a.ca.Cert.Subject.CommonName, Fingerprint: a.ca.Fingerprint(), CAUntil: a.ca.Cert.NotAfter,
		ServerUntil: s.server.Cert.NotAfter, Renewed: s.renewed,
		Names:    append(append([]string{}, s.server.Cert.DNSNames...), ipStrings(s.server.Cert.IPAddresses)...),
		Skipped:  s.server.Skipped,
		SetupURL: a.setupURL(), SecureURL: a.secureURL(""),
	}
	for _, h := range s.names.DNS {
		if h != "localhost" && strings.Contains(h, ".") && a.ca.Permits(h) {
			d.Alternatives = append(d.Alternatives, a.secureURL(h))
		}
	}
	_, err := os.Stat(filepath.Join(a.appDir, "index.html"))
	d.AppReady = err == nil
	if d.Loopback {
		d.Token = a.token
	}
	switch view {
	case "setup":
		d.QR = []qrView{
			qr("1. Zertifikat laden", strings.TrimSuffix(d.SetupURL, "/")+"/ca.crt", "über HTTP, danach in den Einstellungen installieren"),
			qr("2. Sichere Seite öffnen", d.SecureURL, "erst nach dem Installieren"),
		}
	case "home":
		if d.Loopback {
			d.QR = []qrView{qr("Handy einrichten", d.SetupURL, "Einrichtungsseite mit CA-Zertifikat")}
			if !a.open {
				d.QR = append(d.QR, qr("Handy mit Schreibrecht", d.SecureURL+"?token="+a.token, "nach dem Einrichten scannen; merkt sich den Upload-Schlüssel"))
			}
			if d.AppReady {
				d.QR = append(d.QR, qr("App öffnen", d.SecureURL+"app/", "die ausgelieferte Web-App"))
			}
		}
	}
	if fill != nil {
		fill(d)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := page.Execute(w, d); err != nil {
		a.logf("Seite: %v", err)
	}
}

// --- Schreibrecht -----------------------------------------------------------------

func (a *app) tokenOK(t string) bool {
	return t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(a.token)) == 1
}

// mayWrite: am Rechner selbst immer, aus dem Netz mit Upload-Schlüssel
// (Authorization: Bearer, X-Upload-Token, ?token= oder Cookie) bzw. mit --open.
func (a *app) mayWrite(r *http.Request) bool {
	if a.open || isLoopback(r) {
		return true
	}
	if t, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && a.tokenOK(t) {
		return true
	}
	if a.tokenOK(r.Header.Get("X-Upload-Token")) || a.tokenOK(r.URL.Query().Get("token")) {
		return true
	}
	c, err := r.Cookie(tokenCookie)
	return err == nil && a.tokenOK(c.Value)
}

// --- Dateien ---------------------------------------------------------------------

// filesHandler: /files/<pfad>
//
//	GET    Datei herunterladen (Range, HEAD; ?download=1 als Anhang) bzw. Verzeichnis
//	       als Seite oder JSON (?format=json, ?recursive=1 alle Dateien darunter)
//	PUT    Datei schreiben (Inhalt = Rumpf), Verzeichnisse entstehen von selbst
//	DELETE Datei bzw. leeres Verzeichnis löschen (?recursive=1 mit Inhalt)
//	POST   Verzeichnis: Formular-Upload (Feld file, mehrfach; unzip=1 entpackt zip),
//	       op=mkdir (Feld name), op=delete (Feld name)
//
// CORS ist offen, damit eine PWA von anderer Adresse lesen und schreiben kann.
func (a *app) filesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Vary", "Origin")
		h.Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, POST, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Upload-Token, Range")
		h.Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, ETag, Last-Modified, X-Sha256")
		h.Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		rel, ok := cleanRel(strings.TrimPrefix(r.URL.Path, "/files"))
		if !ok {
			httpError(w, r, http.StatusBadRequest, "ungültiger Pfad")
			return
		}
		root, err := os.OpenRoot(a.filesDir)
		if err != nil {
			httpError(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		defer root.Close()
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			a.get(w, r, root, rel)
		case http.MethodPut, http.MethodPost, http.MethodDelete:
			if !a.mayWrite(r) {
				httpError(w, r, http.StatusForbidden, "Schreiben nur mit Upload-Schlüssel")
				return
			}
			switch r.Method {
			case http.MethodPut:
				a.put(w, r, root, rel)
			case http.MethodDelete:
				a.del(w, r, root, rel, r.URL.Query().Get("recursive") == "1")
			default:
				a.post(w, r, root, rel)
			}
		default:
			httpError(w, r, http.StatusMethodNotAllowed, "Methode nicht erlaubt")
		}
	})
}

// cleanRel: Pfad relativ zum Ablageverzeichnis ("" = Wurzel); ".." ist nicht erlaubt.
func cleanRel(p string) (string, bool) {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", false
		}
	}
	c := strings.TrimPrefix(path.Clean("/"+p), "/")
	if strings.ContainsRune(c, 0) || strings.Contains(c, "\\") {
		return "", false
	}
	return c, true
}

func dot(rel string) string {
	if rel == "" {
		return "."
	}
	return rel
}

func wantsJSON(r *http.Request) bool {
	return r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json")
}

func (a *app) get(w http.ResponseWriter, r *http.Request, root *os.Root, rel string) {
	fi, err := root.Stat(dot(rel))
	if err != nil {
		httpError(w, r, http.StatusNotFound, "nicht gefunden")
		return
	}
	if fi.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/"+query(r), http.StatusMovedPermanently)
			return
		}
		a.listing(w, r, root, rel, "")
		return
	}
	f, err := root.Open(rel)
	if err != nil {
		httpError(w, r, http.StatusNotFound, "nicht gefunden")
		return
	}
	defer f.Close()
	ct := mime.TypeByExtension(path.Ext(rel))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	// Ablage: o/ und m/ ändern sich nie, alles andere frisch holen.
	if top, _, _ := strings.Cut(rel, "/"); (top == "o" || top == "m") && strings.Contains(rel, "/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(path.Base(rel)))
	}
	http.ServeContent(w, r, path.Base(rel), fi.ModTime(), f)
}

func query(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return "?" + r.URL.RawQuery
}

type listJSON struct {
	Path    string      `json:"path"`
	Entries []entryJSON `json:"entries"`
}

type entryJSON struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Dir      bool      `json:"dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

func (a *app) listing(w http.ResponseWriter, r *http.Request, root *os.Root, rel, msg string) {
	var entries []entry
	if r.URL.Query().Get("recursive") == "1" {
		fs.WalkDir(root.FS(), dot(rel), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if fi, err := d.Info(); err == nil {
				entries = append(entries, entry{Name: d.Name(), Path: p, Size: fi.Size(), Modified: fi.ModTime()})
			}
			return nil
		})
	} else {
		f, err := root.Open(dot(rel))
		if err != nil {
			httpError(w, r, http.StatusNotFound, "nicht gefunden")
			return
		}
		des, _ := f.ReadDir(-1)
		f.Close()
		for _, d := range des {
			if strings.HasSuffix(d.Name(), ".upload") {
				continue // halb geschriebene Uploads
			}
			fi, err := d.Info()
			if err != nil {
				continue
			}
			entries = append(entries, entry{Name: d.Name(), Path: path.Join(rel, d.Name()), Dir: d.IsDir(), Size: fi.Size(), Modified: fi.ModTime()})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return entries[i].Path < entries[j].Path
	})
	if wantsJSON(r) {
		out := listJSON{Path: rel, Entries: []entryJSON{}}
		for _, e := range entries {
			out.Entries = append(out.Entries, entryJSON{e.Name, e.Path, e.Dir, e.Size, e.Modified})
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	for i := range entries {
		entries[i].Href = "/files/" + escapePath(entries[i].Path)
		if entries[i].Dir {
			entries[i].Href += "/"
		}
	}
	crumbs := []crumb{{"Dateien", "/files/"}}
	acc := ""
	if rel != "" {
		for _, seg := range strings.Split(rel, "/") {
			acc = path.Join(acc, seg)
			crumbs = append(crumbs, crumb{seg, "/files/" + escapePath(acc) + "/"})
		}
	}
	a.render(w, r, "files", func(d *pageData) {
		d.Dir, d.Crumbs, d.Entries, d.Message = rel, crumbs, entries, msg
	})
}

func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

type putJSON struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Sha256 string `json:"sha256"`
}

// writeFile schreibt atomar: Hilfsdatei „.upload“, dann umbenennen.
func (a *app) writeFile(root *os.Root, rel string, src io.Reader) (putJSON, error) {
	if rel == "" {
		return putJSON{}, errors.New("Dateiname fehlt")
	}
	if dir := path.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return putJSON{}, err
		}
	}
	tmp := rel + ".upload"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return putJSON{}, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(src, a.maxUpload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > a.maxUpload {
		err = fmt.Errorf("größer als %s", humanSize(a.maxUpload))
	}
	if err != nil {
		root.Remove(tmp)
		return putJSON{}, err
	}
	if fi, err := root.Stat(rel); err == nil && fi.IsDir() {
		root.Remove(tmp)
		return putJSON{}, errors.New("an dieser Stelle liegt ein Verzeichnis")
	}
	if err := root.Rename(tmp, rel); err != nil {
		root.Remove(tmp)
		return putJSON{}, err
	}
	return putJSON{Path: rel, Size: n, Sha256: hex.EncodeToString(h.Sum(nil))}, nil
}

func (a *app) put(w http.ResponseWriter, r *http.Request, root *os.Root, rel string) {
	if strings.HasSuffix(r.URL.Path, "/") {
		if err := root.MkdirAll(dot(rel), 0o755); err != nil {
			httpError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, putJSON{Path: rel})
		return
	}
	res, err := a.writeFile(root, rel, r.Body)
	if err != nil {
		httpError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("X-Sha256", res.Sha256)
	writeJSON(w, http.StatusCreated, res)
}

func (a *app) del(w http.ResponseWriter, r *http.Request, root *os.Root, rel string, recursive bool) {
	if rel == "" {
		httpError(w, r, http.StatusBadRequest, "die Wurzel wird nicht gelöscht")
		return
	}
	var err error
	if recursive {
		err = root.RemoveAll(rel)
	} else {
		err = root.Remove(rel)
	}
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, fs.ErrNotExist) {
			code = http.StatusNotFound
		}
		httpError(w, r, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// post: Formular im Verzeichnis (Upload, neuer Ordner, Löschen).
func (a *app) post(w http.ResponseWriter, r *http.Request, root *os.Root, rel string) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		httpError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var msgs []string
	var results []putJSON
	switch r.FormValue("op") {
	case "mkdir":
		name, ok := cleanRel(r.FormValue("name"))
		if !ok || name == "" {
			httpError(w, r, http.StatusBadRequest, "ungültiger Name")
			return
		}
		if err := root.MkdirAll(path.Join(rel, name), 0o755); err != nil {
			httpError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		msgs = append(msgs, "Ordner "+name+" angelegt")
	case "delete":
		name, ok := cleanRel(r.FormValue("name"))
		if !ok || name == "" {
			httpError(w, r, http.StatusBadRequest, "ungültiger Name")
			return
		}
		if err := root.RemoveAll(path.Join(rel, name)); err != nil {
			httpError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		msgs = append(msgs, name+" gelöscht")
	default:
		if r.MultipartForm == nil || len(r.MultipartForm.File["file"]) == 0 {
			httpError(w, r, http.StatusBadRequest, "keine Datei gewählt")
			return
		}
		unzip := r.FormValue("unzip") == "1"
		for _, fh := range r.MultipartForm.File["file"] {
			f, err := fh.Open()
			if err != nil {
				msgs = append(msgs, fh.Filename+": "+err.Error())
				continue
			}
			name := path.Base(strings.ReplaceAll(fh.Filename, "\\", "/"))
			if unzip && strings.EqualFold(path.Ext(name), ".zip") {
				n, skipped, err := a.unzip(root, rel, f, fh.Size)
				switch {
				case err != nil:
					msgs = append(msgs, name+": "+err.Error())
				default:
					m := fmt.Sprintf("%s: %d Dateien entpackt", name, n)
					if skipped > 0 {
						m += fmt.Sprintf(", %d übersprungen", skipped)
					}
					msgs = append(msgs, m)
				}
			} else if res, err := a.writeFile(root, path.Join(rel, name), f); err != nil {
				msgs = append(msgs, name+": "+err.Error())
			} else {
				results = append(results, res)
				msgs = append(msgs, fmt.Sprintf("%s hochgeladen (%s)", name, humanSize(res.Size)))
			}
			f.Close()
		}
	}
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{"files": results, "messages": msgs})
		return
	}
	a.listing(w, r, root, rel, strings.Join(msgs, " · "))
}

// unzip entpackt ein zip in das Verzeichnis rel; Einträge, die nach außen
// zeigen würden, lehnt os.Root ab (übersprungen).
func (a *app) unzip(root *os.Root, rel string, f io.ReaderAt, size int64) (n, skipped int, err error) {
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return 0, 0, err
	}
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		name, ok := cleanRel(zf.Name)
		if !ok || name == "" {
			skipped++
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			skipped++
			continue
		}
		_, err = a.writeFile(root, path.Join(rel, name), rc)
		rc.Close()
		if err != nil {
			skipped++
			continue
		}
		n++
	}
	return n, skipped, nil
}

// --- App ---------------------------------------------------------------------------

// appHandler: /app/ liefert app/ aus (Pfade ohne Datei → index.html, für Apps mit
// eigener Navigation); POST /app/ mit einem zip (Feld file) ersetzt die App.
func (a *app) appHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !a.mayWrite(r) {
				httpError(w, r, http.StatusForbidden, "Schreiben nur mit Upload-Schlüssel")
				return
			}
			a.replaceApp(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpError(w, r, http.StatusMethodNotAllowed, "Methode nicht erlaubt")
			return
		}
		rel, ok := cleanRel(strings.TrimPrefix(r.URL.Path, "/app"))
		if !ok {
			httpError(w, r, http.StatusBadRequest, "ungültiger Pfad")
			return
		}
		root, err := os.OpenRoot(a.appDir)
		if err != nil {
			httpError(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		defer root.Close()
		fi, err := root.Stat(dot(rel))
		if err == nil && fi.IsDir() {
			rel = path.Join(rel, "index.html")
			fi, err = root.Stat(rel)
		}
		if err != nil && !strings.Contains(path.Base(rel), ".") {
			rel = "index.html"
			fi, err = root.Stat(rel)
		}
		if err != nil {
			if rel == "index.html" {
				a.render(w, r, "noapp", nil)
				return
			}
			http.NotFound(w, r)
			return
		}
		f, err := root.Open(rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		if ct := mime.TypeByExtension(path.Ext(rel)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, path.Base(rel), fi.ModTime(), f)
	})
}

// replaceApp entpackt das zip in ein neues Verzeichnis und tauscht es dann aus.
// Liegt alles in einem Ordner (z. B. dist/), wird dieser zur Wurzel.
func (a *app) replaceApp(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxUpload)
	f, fh, err := r.FormFile("file")
	if err != nil {
		httpError(w, r, http.StatusBadRequest, "zip-Datei fehlt")
		return
	}
	defer f.Close()
	zr, err := zip.NewReader(f, fh.Size)
	if err != nil {
		httpError(w, r, http.StatusBadRequest, "kein zip: "+err.Error())
		return
	}
	prefix := commonDir(zr.File)
	tmp, err := os.MkdirTemp(a.dir, "app-neu-")
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(tmp)
	root, err := os.OpenRoot(tmp)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	n := 0
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		name, ok := cleanRel(strings.TrimPrefix(zf.Name, prefix))
		if !ok || name == "" {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		if _, err := a.writeFile(root, name, rc); err == nil {
			n++
		}
		rc.Close()
	}
	_, err = root.Stat("index.html")
	root.Close()
	if err != nil {
		httpError(w, r, http.StatusBadRequest, "im zip fehlt index.html")
		return
	}
	old := a.appDir + ".alt"
	os.RemoveAll(old)
	if err := os.Rename(a.appDir, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		httpError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.Rename(tmp, a.appDir); err != nil {
		os.Rename(old, a.appDir)
		httpError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	os.RemoveAll(old)
	a.logf("App ersetzt: %d Dateien", n)
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{"files": n})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// commonDir: gemeinsamer erster Ordner aller Einträge ("dist/"), sonst "".
func commonDir(files []*zip.File) string {
	prefix := ""
	for i, f := range files {
		first, _, ok := strings.Cut(f.Name, "/")
		if !ok {
			return ""
		}
		if i == 0 {
			prefix = first + "/"
		} else if first+"/" != prefix {
			return ""
		}
	}
	return prefix
}

// --- Hilfen ------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	if wantsJSON(r) || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Header.Get("Origin") != "" {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	http.Error(w, msg, code)
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
