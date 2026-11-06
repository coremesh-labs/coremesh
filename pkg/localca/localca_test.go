package localca

import (
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func names(dns []string, ips ...string) Names {
	n := Names{DNS: dns}
	for _, s := range ips {
		n.IPs = append(n.IPs, net.ParseIP(s))
	}
	return n
}

func TestCAAndServer(t *testing.T) {
	dir := t.TempDir()
	ca, created, err := LoadOrCreateCA(dir, "Test", []string{"faust-pc", "fritz.box"})
	if err != nil || !created {
		t.Fatalf("CA: %v %v", created, err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "ca.key")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("ca.key Rechte %v", fi.Mode().Perm())
	}
	n := names([]string{"localhost", "faust-pc", "faust-pc.local", "faust-pc.fritz.box", "example.com"}, "127.0.0.1", "192.168.178.52", "8.8.8.8")
	s, err := EnsureServer(dir, ca, n)
	if err != nil || !s.Renewed {
		t.Fatalf("Server: %v", err)
	}
	if len(s.Skipped) != 2 {
		t.Fatalf("übersprungen %v, erwartet example.com und 8.8.8.8", s.Skipped)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)
	for _, h := range []string{"localhost", "faust-pc.local", "faust-pc.fritz.box", "192.168.178.52"} {
		if _, err := s.Cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: h}); err != nil {
			t.Errorf("%s: %v", h, err)
		}
	}

	// gleiches Netz: kein neues Zertifikat; CA wird geladen, nicht neu angelegt
	ca2, created, err := LoadOrCreateCA(dir, "Test", nil)
	if err != nil || created || ca2.Fingerprint() != ca.Fingerprint() {
		t.Fatalf("CA neu geladen: %v %v", created, err)
	}
	s2, err := EnsureServer(dir, ca2, n)
	if err != nil || s2.Renewed {
		t.Fatalf("ohne Änderung neu ausgestellt: %v", err)
	}

	// neues Netz: andere IP → neues Serverzertifikat, dieselbe CA
	n.IPs = append(n.IPs, net.ParseIP("10.0.0.7"))
	s3, err := EnsureServer(dir, ca2, n)
	if err != nil || !s3.Renewed || s3.Cert.VerifyHostname("10.0.0.7") != nil {
		t.Fatalf("neue IP: %v", err)
	}
}

func TestNameConstraints(t *testing.T) {
	ca, err := createCA("Test", []string{"faust-pc"})
	if err != nil {
		t.Fatal(err)
	}
	// Selbst ein Zertifikat für eine fremde Seite, mit dem CA-Schlüssel signiert,
	// besteht die Prüfung nicht.
	bad, err := issue(ca, []string{"bank.example"})
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)
	if _, err := bad.Cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "bank.example"}); err == nil {
		t.Fatal("fremder Name wurde angenommen")
	}
	for h, want := range map[string]bool{"faust-pc": true, "x.local": true, "localhost": true, "10.1.2.3": true, "8.8.8.8": false, "evil.com": false, "notlocal": false} {
		if ca.Permits(h) != want {
			t.Errorf("Permits(%s) = %v", h, !want)
		}
	}
}

func TestDetect(t *testing.T) {
	n := Detect("extra-name", "192.168.1.200")
	if n.Primary == nil || len(n.DNS) == 0 {
		t.Fatalf("Detect: %+v", n)
	}
	hosts := n.Hosts()
	for _, h := range []string{"localhost", "127.0.0.1", "extra-name", "192.168.1.200"} {
		found := false
		for _, x := range hosts {
			found = found || x == h
		}
		if !found {
			t.Errorf("%s fehlt in %v", h, hosts)
		}
	}
}
