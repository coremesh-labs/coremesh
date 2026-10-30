package main

import (
	"archive/tar"
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
//	releases/<stand>/   Programme, configs, migrations, Inhalte, manifest.json eines Stands
//	plugins/<xx>/       Plugins aller Stände (Name und Version im Dateinamen, je Version einmal)
//	local/90-local.yaml eigene Einstellungen (Port, Pfade) – wird bei Updates neu geschrieben,
//	                    weitere Dateien in local/ bleiben
//	data/               Datenbank, Socket, Log; data/setup-backup/ Sicherungen vor Updates
//	current             aktiver Stand
//	setup.json          Linie, Ausprägung, Port, vorheriger Stand (Rückfall)
//	ZUGANG.txt          Adresse, Benutzer, Passwort
//	start.sh, stop.sh, status.sh (Windows: .cmd) und coremesh-setup
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
`, filepath.ToSlash(in.path("plugins")), relDir+"/migrations", "unix://"+filepath.ToSlash(in.socket()), port, port, user)
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

// hostCmd: Aufruf des Hosts im aktiven Stand (Arbeitsverzeichnis, Log, Zugang).
// Der Aufrufer schließt die Log-Datei.
func (in *installation) hostCmd() (*exec.Cmd, *os.File, error) {
	rel := in.current()
	if rel == "" {
		return nil, nil, errors.New("keine Installation in " + in.dir)
	}
	_, _, pw := in.access()
	log, err := os.OpenFile(in.path("data", "host.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	os.Remove(in.socket())
	cmd := exec.Command(in.path("releases", rel, exe("host")), "-config",
		filepath.Join("releases", rel, "configs")+","+"local")
	cmd.Dir, cmd.Stdout, cmd.Stderr = in.dir, log, log
	cmd.Env = append(os.Environ(), "COREMESH_ADMIN_PASSWORD="+pw)
	return cmd, log, nil
}

// start startet den Host im Hintergrund (bzw. über den Benutzerdienst) und
// wartet, bis die Konsole antwortet.
func (in *installation) start() error {
	if in.running() {
		return nil
	}
	exited := make(chan struct{})
	if unit := in.state().Service; unit != "" {
		if err := systemctl("start", unit); err != nil {
			return err
		}
	} else {
		cmd, log, err := in.hostCmd()
		if err != nil {
			return err
		}
		defer log.Close()
		detach(cmd)
		if err := cmd.Start(); err != nil {
			return err
		}
		if err := os.WriteFile(in.path("data", "host.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
			return err
		}
		go func() { cmd.Wait(); close(exited) }() // kein Zombie, solange coremesh-setup läuft
	}
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

// running: Host läuft (PID-Datei) oder die Konsole antwortet (z. B. eigenständig gestartet).
func (in *installation) running() bool {
	return (in.pid() > 0 && alive(in.pid())) || listening(in.socket())
}

func (in *installation) stop() error {
	if unit := in.state().Service; unit != "" {
		if err := systemctl("stop", unit); err != nil {
			return err
		}
		os.Remove(in.path("data", "host.pid"))
		return nil
	}
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
