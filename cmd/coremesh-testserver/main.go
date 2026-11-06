// Command coremesh-testserver ist ein Webserver für Tests im lokalen Netz:
// Dateien hochladen und herunterladen (z. B. die Ablage für die PWA), eine
// gebaute PWA ausliefern, HTTPS mit eigener Zertifizierungsstelle, deren
// Serverzertifikat sich an das jeweilige Netz anpasst, und eine Startseite mit
// QR-Codes zum Einrichten des Handys.
//
//	coremesh-testserver [--dir=testserver] [--http=:8080] [--https=:8443]
//	                    [--name=weitere,namen] [--label=Testserver] [--open]
//
// Verzeichnis:
//
//	certs/        server.crt, server.key (die CA liegt unter --ca, Standard ~/.config/coremesh/ca)
//	files/        Dateien (Upload/Download unter /files/)
//	app/          ausgelieferte Web-App unter /app/
//	upload-token  Schlüssel für Schreibzugriffe aus dem Netz
//
// Ablauf am Handy: QR-Code (Terminal oder http://localhost:8080) scannen →
// Einrichtungsseite (HTTP) → CA-Zertifikat laden und installieren → weiter zur
// sicheren Seite (HTTPS) → Dateien bzw. App.
package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/localca"
	"github.com/coremesh-labs/coremesh/pkg/sdk/qrcode"
)

// state: was sich zur Laufzeit ändern kann (Netz, Zertifikat).
type state struct {
	mu      sync.RWMutex
	names   localca.Names
	server  *localca.Server
	renewed time.Time
}

type app struct {
	dir, filesDir, appDir, certsDir string
	caDir                           string
	httpPort, httpsPort             string
	extra                           []string
	label                           string
	token                           string
	open                            bool
	maxUpload                       int64
	ca                              *localca.CA
	st                              state
	logf                            func(string, ...any)
}

func main() {
	dir := flag.String("dir", "testserver", "Verzeichnis für Serverzertifikat, Dateien und App")
	caDir := flag.String("ca", localca.DefaultDir(), "Verzeichnis der Zertifizierungsstelle (gemeinsam für alle Server dieses Benutzers)")
	httpAddr := flag.String("http", ":8080", "Adresse für HTTP (Einrichtung, CA-Zertifikat)")
	httpsAddr := flag.String("https", ":8443", "Adresse für HTTPS")
	extra := flag.String("name", "", "weitere Namen bzw. Adressen fürs Zertifikat, durch Komma getrennt")
	label := flag.String("label", "Testserver", "Name der Zertifizierungsstelle (CoreMesh <label> CA)")
	open := flag.Bool("open", false, "Schreiben aus dem Netz ohne Upload-Schlüssel erlauben")
	maxMB := flag.Int64("max-upload", 1024, "größter Upload in MB")
	flag.Parse()

	a, err := newApp(*dir, *caDir, *extra, *label, *open, *maxMB<<20, log.Printf)
	if err != nil {
		log.Fatalf("Fehler: %v", err)
	}
	a.httpPort, a.httpsPort = port(*httpAddr), port(*httpsAddr)
	a.banner()

	stop := make(chan struct{})
	go a.watch(30*time.Second, stop)

	errc := make(chan error, 2)
	go func() {
		errc <- (&http.Server{Addr: *httpAddr, Handler: a.logged(a.httpHandler()), ReadHeaderTimeout: 10 * time.Second}).ListenAndServe()
	}()
	go func() {
		s := &http.Server{Addr: *httpsAddr, Handler: a.logged(a.httpsHandler()), ReadHeaderTimeout: 10 * time.Second,
			TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: a.certificate}}
		errc <- s.ListenAndServeTLS("", "")
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errc:
		log.Fatalf("Fehler: %v", err)
	case <-sig:
		close(stop)
		log.Printf("beendet")
	}
}

func newApp(dir, caDir, extra, label string, open bool, maxUpload int64, logf func(string, ...any)) (*app, error) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	a := &app{dir: d, filesDir: filepath.Join(d, "files"), appDir: filepath.Join(d, "app"), certsDir: filepath.Join(d, "certs"),
		label: label, open: open, maxUpload: maxUpload, logf: logf, httpPort: "8080", httpsPort: "8443"}
	for _, e := range strings.Split(extra, ",") {
		if e = strings.TrimSpace(e); e != "" {
			a.extra = append(a.extra, e)
		}
	}
	for _, p := range []string{a.filesDir, a.appDir, a.certsDir} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, err
		}
	}
	if a.token, err = loadToken(filepath.Join(d, "upload-token")); err != nil {
		return nil, err
	}
	names := localca.Detect(a.extra...)
	// Erlaubte Namen der CA: Rechnername, Suchdomänen, weitere Namen.
	domains := append([]string{}, a.extra...)
	if h, err := os.Hostname(); err == nil {
		short, _, _ := strings.Cut(strings.ToLower(h), ".")
		domains = append(domains, short)
	}
	domains = append(domains, localca.SearchDomains()...)
	// CA am festen Ort; eine CA aus certs/ (ältere Fassung) wird dorthin übernommen.
	a.caDir, err = filepath.Abs(caDir)
	if err != nil {
		return nil, err
	}
	if ok, err := localca.Adopt(a.certsDir, a.caDir); err != nil {
		return nil, err
	} else if ok {
		logf("Zertifizierungsstelle aus %s nach %s übernommen", a.certsDir, a.caDir)
	}
	if fp := localca.Same(a.certsDir); fp != "" && fp != localca.Same(a.caDir) {
		logf("Achtung: %s enthält eine andere CA – sie wird nicht benutzt", a.certsDir)
	}
	ca, created, err := localca.LoadOrCreateCA(a.caDir, label, domains)
	if err != nil {
		return nil, err
	}
	if created {
		logf("Zertifizierungsstelle angelegt: %s in %s", ca.Cert.Subject.CommonName, a.caDir)
	}
	a.ca = ca
	if err := a.refresh(names); err != nil {
		return nil, err
	}
	return a, nil
}

// refresh: Serverzertifikat zum Netz passend halten.
func (a *app) refresh(names localca.Names) error {
	s, err := localca.EnsureServer(a.certsDir, a.ca, names)
	if err != nil {
		return err
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.names, a.st.server = names, s
	if s.Renewed {
		a.st.renewed = time.Now()
		a.logf("Serverzertifikat ausgestellt für %s (gültig bis %s)", strings.Join(append(s.Cert.DNSNames, ipStrings(s.Cert.IPAddresses)...), ", "), s.Cert.NotAfter.Format("02.01.2006"))
	}
	if len(s.Skipped) > 0 && s.Renewed {
		a.logf("nicht im Zertifikat (außerhalb der CA): %s", strings.Join(s.Skipped, ", "))
	}
	return nil
}

// watch prüft regelmäßig, ob sich Adressen geändert haben (WLAN gewechselt).
func (a *app) watch(every time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			old := a.snapshot().names.Primary
			if err := a.refresh(localca.Detect(a.extra...)); err != nil {
				a.logf("Zertifikat: %v", err)
			}
			if p := a.snapshot().names.Primary; !p.Equal(old) {
				a.logf("Netz geändert: neue Adresse %s", p)
				a.printQR()
			}
		}
	}
}

type snap struct {
	names   localca.Names
	server  *localca.Server
	renewed time.Time
}

func (a *app) snapshot() snap {
	a.st.mu.RLock()
	defer a.st.mu.RUnlock()
	return snap{a.st.names, a.st.server, a.st.renewed}
}

func (a *app) certificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	s := a.snapshot().server
	if s == nil {
		return nil, errors.New("kein Zertifikat")
	}
	return &s.TLS, nil
}

// Adressen für Links und QR-Codes.
func (a *app) setupURL() string {
	return "http://" + hostPort(a.snapshot().names.Primary.String(), a.httpPort, "80") + "/"
}

func (a *app) secureURL(host string) string {
	if host == "" {
		host = a.snapshot().names.Primary.String()
	}
	return "https://" + hostPort(host, a.httpsPort, "443") + "/"
}

func hostPort(host, port, def string) string {
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port == def || port == "" {
		return host
	}
	return host + ":" + port
}

func port(addr string) string {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return p
}

func ipStrings(ips []net.IP) []string {
	var out []string
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}

func loadToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	t := hex.EncodeToString(buf)
	return t, os.WriteFile(path, []byte(t+"\n"), 0o600)
}

func (a *app) banner() {
	s := a.snapshot()
	fmt.Fprintf(os.Stderr, "\nCoreMesh Testserver – %s\n", a.dir)
	fmt.Fprintf(os.Stderr, "  Einrichtung (HTTP): %s\n", a.setupURL())
	fmt.Fprintf(os.Stderr, "  Sicher (HTTPS):     %s\n", a.secureURL(""))
	fmt.Fprintf(os.Stderr, "  am Rechner:         http://localhost:%s/\n", a.httpPort)
	fmt.Fprintf(os.Stderr, "  Namen:              %s\n", strings.Join(s.names.Hosts(), ", "))
	fmt.Fprintf(os.Stderr, "  CA:                 %s, Fingerabdruck %s\n", a.ca.Cert.Subject.CommonName, a.ca.Fingerprint())
	fmt.Fprintf(os.Stderr, "  CA-Verzeichnis:     %s (sichern!)\n", a.caDir)
	if a.open {
		fmt.Fprintf(os.Stderr, "  Schreiben:          aus dem Netz ohne Schlüssel erlaubt (--open)\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Upload-Schlüssel:   %s\n", a.token)
	}
	a.printQR()
}

// printQR: QR-Code der Einrichtungsseite im Terminal (schwarz auf weiß, je
// Zeichen zwei Module übereinander).
func (a *app) printQR() {
	u := a.setupURL()
	c, err := qrcode.Encode(u, qrcode.M)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "\nMit dem Handy scannen: %s\n", u)
	const q = 2
	dark := func(x, y int) bool {
		x, y = x-q, y-q
		return x >= 0 && y >= 0 && x < c.Size && y < c.Size && c.Dark(x, y)
	}
	n := c.Size + 2*q
	var b strings.Builder
	for y := 0; y < n; y += 2 {
		for x := range n {
			fg, bg := "37", "47"
			if dark(x, y) {
				fg = "30"
			}
			if y+1 < n && dark(x, y+1) {
				bg = "40"
			}
			b.WriteString("\x1b[" + fg + ";" + bg + "m▀")
		}
		b.WriteString("\x1b[0m\n")
	}
	fmt.Fprint(os.Stderr, b.String()+"\n")
}

// logged: jede Anfrage mit Ergebnis ins Protokoll.
func (a *app) logged(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusWriter{ResponseWriter: w, status: 200}
		h.ServeHTTP(rw, r)
		a.logf("%s %s %s → %d", remoteIP(r), r.Method, r.URL.RequestURI(), rw.status)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(s int) { w.status = s; w.ResponseWriter.WriteHeader(s) }

func remoteIP(r *http.Request) string {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func isLoopback(r *http.Request) bool {
	ip := net.ParseIP(remoteIP(r))
	return ip != nil && ip.IsLoopback()
}
