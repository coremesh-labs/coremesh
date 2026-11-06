// Package localca ist eine kleine Zertifizierungsstelle (CA) für das lokale
// Netz: Sie legt eine CA an, stellt Serverzertifikate für die Namen und
// Adressen des Rechners aus und erneuert sie, wenn sich das Netz ändert
// (andere IP, anderer Rechnername) oder die Laufzeit zu Ende geht.
//
// Die CA ist per Name Constraints auf lokale Namen (localhost, *.local,
// Rechnername, Suchdomäne des Netzes wie fritz.box) und private Adressbereiche
// beschränkt. Ein Handy, das ihr vertraut, vertraut damit keiner fremden
// Seite – auch dann nicht, wenn ca.key in falsche Hände gerät. Weil die
// Bereiche ganze private Netze umfassen, bleibt die CA gültig, wenn der Rechner
// eine neue Adresse bekommt; nur das Serverzertifikat wird neu ausgestellt.
//
// Ablage: CA (ca.crt, ca.key 0600) am festen Ort je Benutzer (DefaultDir),
// Serverzertifikat (server.crt, server.key 0600) je Server bzw. Installation.
package localca

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Laufzeiten: CA 10 Jahre; Server 397 Tage (Höchstdauer, die Browser annehmen).
// Erneuert wird das Serverzertifikat 30 Tage vor Ablauf.
const (
	CAValidity     = 10 * 365 * 24 * time.Hour
	ServerValidity = 397 * 24 * time.Hour
	RenewBefore    = 30 * 24 * time.Hour
)

// PrivateRanges: die Adressbereiche, auf die die CA beschränkt ist.
var PrivateRanges = []string{
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
	"::1/128", "fc00::/7", "fe80::/10",
}

// Names: Namen und Adressen, unter denen der Rechner erreichbar ist.
type Names struct {
	DNS []string
	IPs []net.IP
	// Primary: die Adresse der Standardroute (für den QR-Code), sonst die erste private.
	Primary net.IP
}

// Detect ermittelt die Namen und Adressen dieses Rechners im aktuellen Netz:
// localhost, Rechnername, rechnername.local, rechnername.<Suchdomäne>,
// 127.0.0.1 und alle privaten IPv4-Adressen der aktiven Schnittstellen.
// extra: zusätzliche Namen bzw. Adressen.
func Detect(extra ...string) Names {
	n := Names{DNS: []string{"localhost"}, IPs: []net.IP{net.IPv4(127, 0, 0, 1).To4()}}
	if h, err := os.Hostname(); err == nil && h != "" {
		h = strings.ToLower(strings.TrimSuffix(h, "."))
		short, _, _ := strings.Cut(h, ".")
		n.addDNS(h)
		n.addDNS(short)
		n.addDNS(short + ".local")
		for _, d := range SearchDomains() {
			n.addDNS(short + "." + d)
		}
	}
	if ifs, err := net.Interfaces(); err == nil {
		for _, i := range ifs {
			if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := i.Addrs()
			for _, a := range addrs {
				ipn, ok := a.(*net.IPNet)
				// nur IPv4: IPv6-Datenschutzadressen wechseln laufend
				if !ok || ipn.IP.To4() == nil || !private(ipn.IP) || ipn.IP.IsLinkLocalUnicast() {
					continue
				}
				n.addIP(ipn.IP)
			}
		}
	}
	for _, e := range extra {
		if e = strings.TrimSpace(e); e == "" {
			continue
		}
		if ip := net.ParseIP(e); ip != nil {
			n.addIP(ip)
		} else {
			n.addDNS(strings.ToLower(e))
		}
	}
	n.Primary = primaryIP()
	if n.Primary == nil || !slices.ContainsFunc(n.IPs, n.Primary.Equal) {
		n.Primary = nil
		for _, ip := range n.IPs {
			if !ip.IsLoopback() && ip.To4() != nil {
				n.Primary = ip
				break
			}
		}
	}
	if n.Primary == nil {
		n.Primary = n.IPs[0]
	}
	return n
}

func (n *Names) addDNS(s string) {
	if s != "" && !slices.Contains(n.DNS, s) {
		n.DNS = append(n.DNS, s)
	}
}

func (n *Names) addIP(ip net.IP) {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if !slices.ContainsFunc(n.IPs, ip.Equal) {
		n.IPs = append(n.IPs, ip)
	}
}

// Hosts: alle Namen und Adressen als Text (Anzeige).
func (n Names) Hosts() []string {
	out := slices.Clone(n.DNS)
	for _, ip := range n.IPs {
		out = append(out, ip.String())
	}
	return out
}

// primaryIP: Quelladresse der Standardroute (UDP „verbinden“ sendet nichts).
func primaryIP() net.IP {
	c, err := net.Dial("udp", "192.0.2.1:9")
	if err != nil {
		return nil
	}
	defer c.Close()
	if a, ok := c.LocalAddr().(*net.UDPAddr); ok && private(a.IP) {
		return a.IP.To4()
	}
	return nil
}

func private(ip net.IP) bool {
	for _, r := range PrivateRanges {
		if _, n, _ := net.ParseCIDR(r); n.Contains(ip) {
			return true
		}
	}
	return false
}

// SearchDomains: Suchdomänen des Netzes aus /etc/resolv.conf (z. B. fritz.box);
// leer, wenn es die Datei nicht gibt (Windows).
func SearchDomains() []string {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 2 || (fs[0] != "search" && fs[0] != "domain") {
			continue
		}
		for _, d := range fs[1:] {
			d = strings.ToLower(strings.Trim(d, "."))
			if d != "" && !slices.Contains(out, d) {
				out = append(out, d)
			}
		}
	}
	return out
}

// CA: Zertifikat und Schlüssel der Zertifizierungsstelle.
type CA struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey
	DER  []byte
}

// Fingerprint: SHA-256 des CA-Zertifikats, hex in Paaren (zum Vergleichen am Handy).
func (c *CA) Fingerprint() string {
	s := sha256.Sum256(c.DER)
	h := strings.ToUpper(hex.EncodeToString(s[:]))
	var b strings.Builder
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(h[i : i+2])
	}
	return b.String()
}

// PEM: das CA-Zertifikat als PEM.
func (c *CA) PEM() []byte { return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.DER}) }

// Permits: Darf die CA für diesen Namen bzw. diese Adresse ausstellen?
func (c *CA) Permits(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		for _, r := range c.Cert.PermittedIPRanges {
			if r.Contains(ip) {
				return true
			}
		}
		return false
	}
	host = strings.ToLower(host)
	for _, d := range c.Cert.PermittedDNSDomains {
		d = strings.ToLower(strings.TrimPrefix(d, "."))
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// DefaultDir: der feste Ort der CA je Benutzer, gemeinsam für alle Testserver
// und Installationen dieses Benutzers (COREMESH_CA_DIR bzw.
// <Konfigurationsverzeichnis>/coremesh/ca, unter Linux ~/.config/coremesh/ca).
// Ein Handy, das ihr einmal vertraut, bleibt eingerichtet – auch wenn ein
// Testserver oder eine Installation neu angelegt wird.
func DefaultDir() string {
	if d := os.Getenv("COREMESH_CA_DIR"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "coremesh", "ca")
}

// Adopt übernimmt eine vorhandene CA aus from nach to, wenn es in to noch
// keine gibt (z. B. die CA eines älteren Testservers, der das Handy schon
// vertraut). Die Dateien in from werden danach entfernt, damit es nur eine
// gibt. Ergebnis: übernommen ja/nein.
func Adopt(from, to string) (bool, error) {
	if _, err := os.Stat(filepath.Join(to, "ca.crt")); err == nil {
		return false, nil
	}
	ca, err := loadCA(from)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	keyDER, err := x509.MarshalECPrivateKey(ca.Key)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(to, 0o700); err != nil {
		return false, err
	}
	if err := writeFile(filepath.Join(to, "ca.key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return false, err
	}
	if err := writeFile(filepath.Join(to, "ca.crt"), ca.PEM(), 0o644); err != nil {
		return false, err
	}
	if back, err := loadCA(to); err != nil || back.Fingerprint() != ca.Fingerprint() {
		return false, fmt.Errorf("localca: Übernahme nach %s nicht lesbar", to)
	}
	os.Remove(filepath.Join(from, "ca.key"))
	os.Remove(filepath.Join(from, "ca.crt"))
	return true, nil
}

// Same: Fingerabdruck der CA in dir ("" wenn keine).
func Same(dir string) string {
	ca, err := loadCA(dir)
	if err != nil {
		return ""
	}
	return ca.Fingerprint()
}

// LoadOrCreateCA lädt die CA aus dir oder legt sie an. label erscheint im
// Namen („CoreMesh <label> CA“); domains: zusätzlich erlaubte Namen bzw.
// Domänen (Rechnername, Suchdomäne), localhost und local sind immer dabei.
func LoadOrCreateCA(dir, label string, domains []string) (*CA, bool, error) {
	if ca, err := loadCA(dir); err == nil {
		return ca, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	ca, err := createCA(label, domains)
	if err != nil {
		return nil, false, err
	}
	keyDER, err := x509.MarshalECPrivateKey(ca.Key)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false, err
	}
	if err := writeFile(filepath.Join(dir, "ca.key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return nil, false, err
	}
	if err := writeFile(filepath.Join(dir, "ca.crt"), ca.PEM(), 0o644); err != nil {
		return nil, false, err
	}
	return ca, true, nil
}

func createCA(label string, domains []string) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	permitted := []string{"localhost", "local"}
	for _, d := range domains {
		d = strings.ToLower(strings.Trim(strings.TrimSpace(d), "."))
		if d != "" && net.ParseIP(d) == nil && !slices.Contains(permitted, d) {
			permitted = append(permitted, d)
		}
	}
	var ranges []*net.IPNet
	for _, r := range PrivateRanges {
		_, n, _ := net.ParseCIDR(r)
		ranges = append(ranges, n)
	}
	now := time.Now()
	name := strings.TrimSpace("CoreMesh " + label + " CA")
	tpl := &x509.Certificate{
		SerialNumber:                serial(),
		Subject:                     pkix.Name{CommonName: name, Organization: []string{"CoreMesh"}},
		NotBefore:                   now.Add(-time.Hour),
		NotAfter:                    now.Add(CAValidity),
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid:       true,
		IsCA:                        true,
		MaxPathLenZero:              true,
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         permitted,
		PermittedIPRanges:           ranges,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: key, DER: der}, nil
}

func loadCA(dir string) (*CA, error) {
	crtPEM, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, "ca.key"))
	if err != nil {
		return nil, err
	}
	b, _ := pem.Decode(crtPEM)
	k, _ := pem.Decode(keyPEM)
	if b == nil || k == nil {
		return nil, fmt.Errorf("localca: ca.crt bzw. ca.key in %s unlesbar", dir)
	}
	cert, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParseECPrivateKey(k.Bytes)
	if err != nil {
		return nil, err
	}
	if !key.PublicKey.Equal(cert.PublicKey) {
		return nil, fmt.Errorf("localca: ca.key passt nicht zu ca.crt")
	}
	return &CA{Cert: cert, Key: key, DER: b.Bytes}, nil
}

// Server: das aktuelle Serverzertifikat.
type Server struct {
	TLS     tls.Certificate
	Cert    *x509.Certificate
	Renewed bool     // eben neu ausgestellt
	Skipped []string // Namen, die die CA nicht abdeckt (nicht im Zertifikat)
}

// EnsureServer lädt server.crt aus dir und stellt ein neues aus, wenn es fehlt,
// bald abläuft, nicht von dieser CA stammt oder einen der Namen bzw. eine der
// Adressen nicht enthält. Namen außerhalb der Beschränkung der CA entfallen.
func EnsureServer(dir string, ca *CA, names Names) (*Server, error) {
	var want []string
	var skipped []string
	for _, h := range names.Hosts() {
		if ca.Permits(h) {
			want = append(want, h)
		} else {
			skipped = append(skipped, h)
		}
	}
	if s, err := loadServer(dir); err == nil && s.covers(ca, want) {
		s.Skipped = skipped
		return s, nil
	}
	s, err := issue(ca, want)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(s.TLS.PrivateKey.(*ecdsa.PrivateKey))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(dir, "server.key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return nil, err
	}
	crt := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Cert.Raw}), ca.PEM()...)
	if err := writeFile(filepath.Join(dir, "server.crt"), crt, 0o644); err != nil {
		return nil, err
	}
	s.Renewed, s.Skipped = true, skipped
	return s, nil
}

func (s *Server) covers(ca *CA, hosts []string) bool {
	if time.Until(s.Cert.NotAfter) < RenewBefore || s.Cert.CheckSignatureFrom(ca.Cert) != nil {
		return false
	}
	for _, h := range hosts {
		if s.Cert.VerifyHostname(h) != nil {
			return false
		}
	}
	return true
}

func issue(ca *CA, hosts []string) (*Server, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: hosts[0], Organization: []string{"CoreMesh"}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(ServerValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Server{
		TLS:  tls.Certificate{Certificate: [][]byte{der, ca.DER}, PrivateKey: key, Leaf: cert},
		Cert: cert,
	}, nil
}

func loadServer(dir string) (*Server, error) {
	t, err := tls.LoadX509KeyPair(filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"))
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(t.Certificate[0])
	if err != nil {
		return nil, err
	}
	t.Leaf = cert
	return &Server{TLS: t, Cert: cert}, nil
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	return n
}

// writeFile schreibt atomar (Hilfsdatei, dann umbenennen).
func writeFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
