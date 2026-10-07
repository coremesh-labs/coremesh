// Command console ist die Kommandozeile für CoreMesh. Sie spricht per gRPC mit
// dem Plugin console (Unix-Socket oder TCP) und meldet sich über iam an.
//
// Beispiele:
//
//	console --object Greeting --action list
//	console --object Greeting --action say --param name=Christof
//	console --object BusinessPartner --action SampleFile --out ./partner_template.yaml
//	console --object BusinessPartner --sample --format csv
//	console --object AssetsModule --action ExportBundle --param theme=dark --target-dir /var/www/coremesh/static
//	console --logout
//
// Konsolenbefehle der Module (ModuleDefinition.Commands), aufgelöst vom Console-Plugin:
//
//	console ledger:help
//	console ledger:load-coa --chart=SKR04
//	console ledger:load-coa --chart=SKR25 --file=./skr25_komplett.csv
//
// Adresse: --addr oder COREMESH_CONSOLE (Standard unix://data/console.sock),
// Benutzer: --user oder COREMESH_USER, Passwort: COREMESH_PASSWORD oder Abfrage.
// Das Token wird im Benutzer-Konfigurationsverzeichnis zwischengespeichert.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/coremesh-lab/coremesh/pkg/consoleapi/console/v1"
)

const maxMessage = 64 << 20

// params sammelt wiederholte --param key=value.
type params map[string]any

func (p params) String() string { return fmt.Sprint(map[string]any(p)) }

// Set: Werte, die gültiges JSON sind (Zahl, true, Objekt …), werden als
// solche übernommen, alles andere als Text.
func (p params) Set(s string) error {
	k, v, ok := strings.Cut(s, "=")
	if !ok || k == "" {
		return fmt.Errorf("erwartet key=value: %q", s)
	}
	var j any
	if err := json.Unmarshal([]byte(v), &j); err == nil {
		p[k] = j
	} else {
		p[k] = v
	}
	return nil
}

type options struct {
	command                   string // <modul>:<befehl>
	addr, tlsCA, user         string
	object, action, targetDir string
	format, out               string
	sample, logout            bool
	params                    params
	timeout                   time.Duration
}

func main() {
	o := options{params: params{}}
	flag.StringVar(&o.addr, "addr", env("COREMESH_CONSOLE", "unix://data/console.sock"), "unix://<pfad> oder tcp://<host>:<port>")
	flag.StringVar(&o.tlsCA, "tls-ca", "", "TLS für tcp:// mit dieser CA-Datei (PEM); ohne Angabe bei tcp:// unverschlüsselt")
	flag.StringVar(&o.user, "user", env("COREMESH_USER", ""), "Benutzername (iam)")
	flag.StringVar(&o.object, "object", "", "Ziel-Object, z. B. BusinessPartner")
	flag.StringVar(&o.action, "action", "", "Ziel-Action, z. B. list – oder SampleFile")
	flag.Var(o.params, "param", "Parameter key=value (mehrfach möglich)")
	flag.StringVar(&o.targetDir, "target-dir", "", "absolutes Verzeichnis auf dem Server: zip_content der Antwort dort entpacken")
	flag.BoolVar(&o.sample, "sample", false, "Beispieldatei aus dem Metamodell erzeugen (wie --action SampleFile)")
	flag.StringVar(&o.format, "format", "yaml", "Format der Beispieldatei: yaml, json oder csv")
	flag.StringVar(&o.out, "out", "", "Ergebnis in diese Datei schreiben statt auf die Standardausgabe")
	flag.BoolVar(&o.logout, "logout", false, "abmelden und gespeichertes Token löschen")
	flag.DurationVar(&o.timeout, "timeout", 2*time.Minute, "Zeitlimit des Aufrufs")
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && strings.Contains(args[0], ":") {
		o.command, args = args[0], commandArgs(args[1:], o.params)
	}
	if err := flag.CommandLine.Parse(args); err != nil {
		os.Exit(2)
	}

	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "console:", describe(err))
		os.Exit(1)
	}
}

func run(o options) error {
	conn, err := dial(o.addr, o.tlsCA)
	if err != nil {
		return err
	}
	defer conn.Close()
	c := consolev1.NewConsoleServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	tokens := tokenFile(o.addr)

	if o.logout {
		if tok, _ := os.ReadFile(tokens); len(tok) > 0 {
			_, _ = c.Logout(withToken(ctx, string(tok)), &consolev1.LogoutRequest{})
		}
		_ = os.Remove(tokens)
		fmt.Fprintln(os.Stderr, "abgemeldet")
		return nil
	}
	if o.command == "" && (o.object == "" || (o.action == "" && !o.sample)) {
		flag.Usage()
		return errors.New("--object und --action (oder --sample) sind Pflicht")
	}

	// Aufruf; bei abgelaufenem/fehlendem Token einmal neu anmelden.
	for attempt := 0; ; attempt++ {
		tok, _ := os.ReadFile(tokens)
		err := call(withToken(ctx, string(tok)), c, o)
		if status.Code(err) != codes.Unauthenticated || attempt > 0 {
			return err
		}
		if err := login(ctx, c, o.user, tokens); err != nil {
			return err
		}
	}
}

func call(ctx context.Context, c consolev1.ConsoleServiceClient, o options) error {
	if o.command != "" {
		return runCommand(ctx, c, o)
	}
	if o.sample || strings.EqualFold(o.action, "SampleFile") {
		resp, err := c.SampleFile(ctx, &consolev1.SampleFileRequest{TargetObject: o.object, Format: o.format})
		if err != nil {
			return err
		}
		return output(o.out, resp.Content, fmt.Sprintf("Beispieldatei (%s) für %s", resp.Format, o.object))
	}

	p, err := structpb.NewStruct(o.params)
	if err != nil {
		return err
	}
	resp, err := c.Execute(ctx, &consolev1.ExecuteRequest{
		TargetObject: o.object, TargetAction: o.action, Parameters: p, TargetDirectory: o.targetDir})
	if err != nil {
		return err
	}
	if resp.Message != "" {
		fmt.Fprintln(os.Stderr, resp.Message)
	}
	body, err := json.MarshalIndent(resp.Payload.AsInterface(), "", "  ")
	if err != nil {
		return err
	}
	return output(o.out, append(body, '\n'), "Antwort")
}

func output(path string, content []byte, what string) error {
	if path == "" {
		_, err := os.Stdout.Write(content)
		return err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s nach %s geschrieben (%d Bytes)\n", what, path, len(content))
	return nil
}

// --- Anmeldung -------------------------------------------------------------------

func login(ctx context.Context, c consolev1.ConsoleServiceClient, user, tokens string) error {
	if user == "" {
		fmt.Fprint(os.Stderr, "Benutzername: ")
		if _, err := fmt.Fscanln(os.Stdin, &user); err != nil {
			return errors.New("kein Benutzername (--user oder COREMESH_USER)")
		}
	}
	pw := os.Getenv("COREMESH_PASSWORD")
	if pw == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("kein Passwort: COREMESH_PASSWORD setzen oder interaktiv aufrufen")
		}
		fmt.Fprintf(os.Stderr, "Passwort für %s: ", user)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		pw = string(b)
	}
	resp, err := c.Login(ctx, &consolev1.LoginRequest{Username: user, Password: pw})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "angemeldet als %s (bis %s)\n", resp.DisplayName, time.Unix(resp.ExpiresUnix, 0).Format("15:04"))
	if err := os.MkdirAll(filepath.Dir(tokens), 0o700); err == nil {
		_ = os.WriteFile(tokens, []byte(resp.Token), 0o600)
	}
	return nil
}

func withToken(ctx context.Context, tok string) context.Context {
	if tok == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+strings.TrimSpace(tok))
}

// tokenFile: ein Token je Adresse im Benutzer-Konfigurationsverzeichnis.
func tokenFile(addr string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	sum := sha256.Sum256([]byte(addr))
	return filepath.Join(dir, "coremesh", "console-"+hex.EncodeToString(sum[:6])+".token")
}

// --- Verbindung --------------------------------------------------------------------

func dial(addr, tlsCA string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessage), grpc.MaxCallSendMsgSize(maxMessage))}
	switch {
	case strings.HasPrefix(addr, "unix://"):
		path := strings.TrimPrefix(addr, "unix://")
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", path)
			}))
		return grpc.NewClient("passthrough:///console", opts...)
	case strings.HasPrefix(addr, "tcp://"):
		creds := insecure.NewCredentials()
		if tlsCA != "" {
			pem, err := os.ReadFile(tlsCA)
			if err != nil {
				return nil, err
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("%s enthält kein Zertifikat", tlsCA)
			}
			creds = credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
		return grpc.NewClient("passthrough:///"+strings.TrimPrefix(addr, "tcp://"), opts...)
	}
	return nil, fmt.Errorf("--addr muss mit unix:// oder tcp:// beginnen: %q", addr)
}

// describe macht gRPC-Fehler lesbar.
func describe(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return err.Error()
	}
	prefix := map[codes.Code]string{
		codes.Unauthenticated:   "nicht angemeldet",
		codes.PermissionDenied:  "keine Berechtigung",
		codes.NotFound:          "nicht gefunden",
		codes.InvalidArgument:   "ungültige Eingabe",
		codes.Unimplemented:     "nicht vorhanden",
		codes.Unavailable:       "Console nicht erreichbar",
		codes.ResourceExhausted: "gesperrt",
	}[st.Code()]
	if prefix == "" {
		prefix = st.Code().String()
	}
	return prefix + ": " + st.Message()
}

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
