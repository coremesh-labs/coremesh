package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// installation: ein System in einem Verzeichnis.
//
//	releases/<stand>/   Programme, Plugins, configs, migrations (vom Archiv, wird ersetzt)
//	local/90-local.yaml eigene Einstellungen (Port, Pfade) – wird bei Updates nie ersetzt
//	data/               Datenbank, Socket, Log, Sicherungen
//	current             aktiver Stand
//	ZUGANG.txt          Adresse, Benutzer, Passwort
type installation struct {
	dir string
}

func (in *installation) path(p ...string) string {
	return filepath.Join(append([]string{in.dir}, p...)...)
}

func (in *installation) current() string {
	b, _ := os.ReadFile(in.path("current"))
	return strings.TrimSpace(string(b))
}

// access liest ZUGANG.txt (Adresse, Benutzer, Passwort).
func (in *installation) access() (addr, user, password string) {
	b, _ := os.ReadFile(in.path("ZUGANG.txt"))
	for _, l := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "Adresse":
			addr = strings.TrimSpace(v)
		case "Benutzer":
			user = strings.TrimSpace(v)
		case "Passwort":
			password = strings.TrimSpace(v)
		}
	}
	return
}

func (in *installation) writeAccess(addr, user, password string) error {
	s := fmt.Sprintf("CoreMesh (%s)\nAdresse:   %s\nBenutzer:  %s\nPasswort:  %s\n", in.dir, addr, user, password)
	return os.WriteFile(in.path("ZUGANG.txt"), []byte(s), 0o600)
}

func newPassword() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// unpack entpackt ein Archiv nach releases/<stand>.
func (in *installation) unpack(archive, rel string) error {
	dst := in.path("releases", rel)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if strings.HasSuffix(archive, ".zip") {
		return unzip(archive, dst)
	}
	return untar(archive, dst)
}

// configure schreibt die lokalen Einstellungen, den Zugang und den aktiven Stand.
// Ein vorhandener Zugang bleibt, ebenso eigene Zeilen in local/ (nur 90-local.yaml wird neu geschrieben).
func (in *installation) configure(rel string, port int, user, password string) error {
	for _, d := range []string{"local", "data"} {
		if err := os.MkdirAll(in.path(d), 0o755); err != nil {
			return err
		}
	}
	relDir := filepath.ToSlash(in.path("releases", rel))
	local := fmt.Sprintf(`# Eigene Einstellungen dieser Installation (coremesh-setup). Updates ersetzen nur releases/.
host:
  plugin_dir: %q
databases:
  main:
    dsn: "file:data/coremesh.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
plugins:
  dbschema:
    settings:
      migrations_dir: %q
  console:
    settings:
      listen: %q
  webserver:
    settings:
      listen: 127.0.0.1:%d          # nur dieser Rechner; 0.0.0.0:%d = im Netz (dann HTTPS!)
      tenant: coremesh
  iam:
    settings:
      tenant: coremesh
      admin_user: %s
      admin_password: ${env:COREMESH_ADMIN_PASSWORD}   # nur beim ersten Start (ZUGANG.txt)
`, relDir+"/plugins", relDir+"/migrations", "unix://"+filepath.ToSlash(in.socket()), port, port, user)
	if err := os.WriteFile(in.path("local", "90-local.yaml"), []byte(local), 0o644); err != nil {
		return err
	}
	if _, _, pw := in.access(); pw == "" {
		if err := in.writeAccess(fmt.Sprintf("http://localhost:%d", port), user, password); err != nil {
			return err
		}
	}
	return os.WriteFile(in.path("current"), []byte(rel+"\n"), 0o644)
}

// socket: data/console.sock; bei langen Verzeichnisnamen (Unix-Sockets höchstens etwa
// 100 Zeichen) eine kurze Adresse im temporären Verzeichnis.
func (in *installation) socket() string {
	if p := in.path("data", "console.sock"); len(p) < 100 {
		return p
	}
	h := sha256.Sum256([]byte(in.dir))
	return filepath.Join(os.TempDir(), "coremesh-"+hex.EncodeToString(h[:6])+".sock")
}

// start startet den Host im Hintergrund und wartet, bis die Konsole antwortet.
func (in *installation) start() error {
	if in.running() {
		return nil
	}
	rel := in.current()
	if rel == "" {
		return errors.New("keine Installation in " + in.dir)
	}
	_, _, pw := in.access()
	log, err := os.OpenFile(in.path("data", "host.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	os.Remove(in.socket())
	cmd := exec.Command(in.path("releases", rel, exe("host")), "-config",
		filepath.Join("releases", rel, "configs")+","+"local")
	cmd.Dir, cmd.Stdout, cmd.Stderr = in.dir, log, log
	cmd.Env = append(os.Environ(), "COREMESH_ADMIN_PASSWORD="+pw)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(in.path("data", "host.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		return err
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }() // kein Zombie, solange coremesh-setup läuft
	for i := 0; i < 120; i++ {
		select {
		case <-exited:
			return fmt.Errorf("Host beendet – siehe %s", in.path("data", "host.log"))
		case <-time.After(500 * time.Millisecond):
		}
		if c, err := net.Dial("unix", in.socket()); err == nil {
			c.Close()
			if _, err := in.console("list"); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("Host antwortet nicht – siehe %s", in.path("data", "host.log"))
}

func (in *installation) pid() int {
	b, err := os.ReadFile(in.path("data", "host.pid"))
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}

func (in *installation) running() bool { return in.pid() > 0 && alive(in.pid()) }

func (in *installation) stop() error {
	pid := in.pid()
	if pid == 0 || !alive(pid) {
		return nil
	}
	if err := terminate(pid); err != nil {
		return err
	}
	for i := 0; i < 60; i++ {
		if !alive(pid) {
			os.Remove(in.path("data", "host.pid"))
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("Host (PID %d) beendet sich nicht", pid)
}

// console ruft die Konsole des aktiven Stands auf, angemeldet mit dem Zugang.
func (in *installation) console(args ...string) ([]byte, error) {
	_, user, pw := in.access()
	cmd := exec.Command(in.path("releases", in.current(), exe("console")), args...)
	cmd.Dir = in.dir
	cmd.Env = append(os.Environ(), "COREMESH_CONSOLE=unix://"+filepath.ToSlash(in.socket()),
		"COREMESH_USER="+user, "COREMESH_PASSWORD="+pw)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("console %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type installOptions struct {
	release, pkg string
	port         int
	withData     bool
}

// install lädt einen Stand und richtet ihn ein. Mit Datenbestand: nur in eine leere
// Installation; das Passwort des Bestands wird sofort ersetzt.
func install(src *source, in *installation, o installOptions, progress func(string)) error {
	ix, err := src.Index()
	if err != nil {
		return err
	}
	rel, err := ix.resolve(o.release)
	if err != nil {
		return err
	}
	m, err := src.Manifest(ix, rel)
	if err != nil {
		return err
	}
	pkg, ok := m.Packages[o.pkg]
	if !ok {
		return fmt.Errorf("Stand %s hat keine Ausprägung %q", rel, o.pkg)
	}
	art, ok := m.Platforms[platform()]
	if !ok {
		return fmt.Errorf("Stand %s gibt es nicht für %s", rel, platform())
	}
	if err := os.MkdirAll(in.path("downloads"), 0o755); err != nil {
		return err
	}
	remember(in.dir)
	progress("lade " + art.File)
	archive, err := download(src, ix, rel, art, in.path("downloads"))
	if err != nil {
		return err
	}
	if in.running() {
		progress("beende laufenden Host")
		if err := in.stop(); err != nil {
			return err
		}
	}
	progress("entpacke nach releases/" + rel)
	if err := in.unpack(archive, rel); err != nil {
		return err
	}
	user, password := "admin", newPassword()
	dbFile := in.path("data", "coremesh.db")
	seeded := false
	if o.withData && pkg.Data != nil {
		if _, err := os.Stat(dbFile); err == nil {
			return errors.New("es gibt schon eine Datenbank – den Testbestand nur in ein neues Verzeichnis einrichten")
		}
		progress("lade " + pkg.Data.File)
		f, err := download(src, ix, rel, pkg.Data.Artifact, in.path("downloads"))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(in.path("data"), 0o755); err != nil {
			return err
		}
		if err := gunzipFile(f, dbFile); err != nil {
			return err
		}
		user, seeded = pkg.Data.User, true
	}
	if err := in.configure(rel, o.port, user, password); err != nil {
		return err
	}
	if seeded {
		// Zugang zuerst mit dem Passwort des Bestands, dann das eigene setzen
		addr, _, own := in.access()
		if err := in.writeAccess(addr, user, pkg.Data.Password); err != nil {
			return err
		}
		progress("starte")
		if err := in.start(); err != nil {
			return err
		}
		progress("setze eigenes Passwort")
		if out, err := in.console("handle", "Account.ChangePassword", "--current="+pkg.Data.Password, "--new="+own); err != nil {
			// ohne die Aufrufzeile: sie enthält das neue Passwort
			return fmt.Errorf("eigenes Passwort setzen: %s", strings.TrimSpace(string(out)))
		}
		return in.writeAccess(addr, user, own)
	}
	progress("starte")
	return in.start()
}

// download lädt eine Datei des Stands (falls nicht schon vorhanden) und prüft die Prüfsumme.
func download(src *source, ix *Index, rel string, a Artifact, dir string) (string, error) {
	dst := filepath.Join(dir, a.File)
	if ok, _ := checkSum(dst, a.SHA256); ok {
		return dst, nil
	}
	u, err := ix.fileURL(rel, a.File)
	if err != nil {
		return "", err
	}
	r, err := src.open(u)
	if err != nil {
		return "", err
	}
	defer r.Close()
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if ok, err := checkSum(tmp, a.SHA256); !ok {
		os.Remove(tmp)
		if err == nil {
			err = fmt.Errorf("%s: Prüfsumme stimmt nicht", a.File)
		}
		return "", err
	}
	return dst, os.Rename(tmp, dst)
}

func checkSum(path, want string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == want, nil
}

func gunzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	zr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, zr); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// safeJoin verhindert Pfade außerhalb des Ziels („../“ im Archiv).
func safeJoin(dst, name string) (string, error) {
	p := filepath.Join(dst, filepath.FromSlash(name))
	if !strings.HasPrefix(p, filepath.Clean(dst)+string(os.PathSeparator)) {
		return "", fmt.Errorf("unzulässiger Pfad im Archiv: %s", name)
	}
	return p, nil
}

func writeEntry(dst, name string, mode os.FileMode, r io.Reader) error {
	p, err := safeJoin(dst, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm()|0o200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func untar(archive, dst string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if err := writeEntry(dst, h.Name, os.FileMode(h.Mode), tr); err != nil {
			return err
		}
	}
}

func unzip(archive, dst string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		err = writeEntry(dst, f.Name, f.Mode(), r)
		r.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
