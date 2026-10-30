package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// Benutzerdienst (Linux, systemd --user): startet den Host mit der Anmeldung und
// beendet ihn sauber beim Abmelden oder Herunterfahren. Ist er eingerichtet
// (setup.json service), starten und beenden start, stop, update und rollback
// über den Dienst; der Dienst ruft „coremesh-setup run“ der Installation auf und
// folgt so jedem Stand ohne Änderung der Unit.

var unitNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// defaultUnit: coremesh-<Verzeichnisname>, z. B. coremesh-vuv.
func defaultUnit(dir string) string {
	n := strings.Map(func(r rune) rune {
		if r < 128 && (r == '-' || r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return r
		}
		return '-'
	}, filepath.Base(dir))
	return "coremesh-" + strings.Trim(n, "-.")
}

// unitFile: Inhalt der Unit; Pfade in Anführungszeichen, % verdoppelt (systemd-Kennzeichner).
// Ohne WorkingDirectory (nimmt keine Anführungszeichen) – run setzt das Verzeichnis selbst.
func (in *installation) unitFile() string {
	q := func(s string) string {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%").Replace(s) + `"`
	}
	return fmt.Sprintf(`# CoreMesh %s – Benutzerdienst von coremesh-setup (service --remove entfernt ihn)
[Unit]
Description=CoreMesh (%s)

[Service]
Type=simple
ExecStart=%s run --dir=%s
KillMode=mixed
TimeoutStopSec=60
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
`, strings.ReplaceAll(in.dir, "%", "%%"), strings.ReplaceAll(in.dir, "%", "%%"), q(in.path(exe("coremesh-setup"))), q(in.dir))
}

// installService richtet den Benutzerdienst ein und startet den Host darüber
// (ein ohne Dienst laufender Host wird vorher beendet).
func installService(in *installation, name string, progress func(string)) error {
	if in.current() == "" {
		return fmt.Errorf("keine Installation in %s", in.dir)
	}
	if name == "" {
		name = defaultUnit(in.dir)
	}
	if !unitNameRe.MatchString(name) {
		return fmt.Errorf("Name des Dienstes %q: nur Buchstaben, Ziffern, - _ .", name)
	}
	dir, err := unitDir()
	if err != nil {
		return err
	}
	if err := in.writeScripts(); err != nil { // coremesh-setup liegt in der Installation
		return err
	}
	st := in.state()
	if st.Service != "" && st.Service != name {
		if err := removeService(in, progress); err != nil {
			return err
		}
		st = in.state()
	}
	if st.Service == "" && in.running() {
		progress("beende den ohne Dienst gestarteten Host")
		if err := in.stop(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	unit := filepath.Join(dir, name+".service")
	progress("schreibe " + unit)
	if err := os.WriteFile(unit, []byte(in.unitFile()), 0o644); err != nil {
		return err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := systemctl("enable", name); err != nil {
		return err
	}
	st.Service = name
	if err := in.saveState(st); err != nil {
		return err
	}
	progress("starte über den Dienst " + name)
	return in.start()
}

// removeService beendet den Host, entfernt den Benutzerdienst; danach startet
// coremesh-setup den Host wieder selbst.
func removeService(in *installation, progress func(string)) error {
	st := in.state()
	if st.Service == "" {
		return nil
	}
	progress("entferne den Dienst " + st.Service)
	if err := in.stop(); err != nil {
		return err
	}
	if err := systemctl("disable", st.Service); err != nil {
		return err
	}
	if dir, err := unitDir(); err == nil {
		os.Remove(filepath.Join(dir, st.Service+".service"))
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	st.Service = ""
	return in.saveState(st)
}

// runForeground: Host im Vordergrund (Aufruf des Dienstes); SIGTERM und SIGINT
// gehen an den Host, das Ende des Hosts beendet coremesh-setup mit seinem Code.
func runForeground(in *installation) error {
	cmd, log, err := in.hostCmd()
	if err != nil {
		return err
	}
	defer log.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	pidFile := in.path("data", "host.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		cmd.Process.Kill()
		return err
	}
	defer os.Remove(pidFile)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for s := range sig {
			cmd.Process.Signal(s)
		}
	}()
	err = cmd.Wait()
	signal.Stop(sig)
	if err != nil {
		return fmt.Errorf("Host: %w – siehe %s", err, in.path("data", "host.log"))
	}
	return nil
}
