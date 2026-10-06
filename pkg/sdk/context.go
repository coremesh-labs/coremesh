package sdk

import (
	"context"
	"database/sql"
	"fmt"
)

// CallContext begleitet jeden Aufruf über die Prozessgrenze hinweg
// (entspricht der Protobuf-Message Context). Die Werte setzt ausschließlich
// der Host; Plugins leiten Mandant oder Benutzer nie aus dem Payload ab.
type CallContext struct {
	RequestID string
	TenantID  string
	UserID    string
	Metadata  map[string]string
}

type callKey struct{}

// WithCall hängt einen CallContext an ctx. Der Host ruft das vor jedem
// Plugin-Aufruf auf; im Plugin setzt das SDK ihn automatisch.
func WithCall(ctx context.Context, c CallContext) context.Context {
	return context.WithValue(ctx, callKey{}, c)
}

// CallFromContext liefert den CallContext des laufenden Aufrufs oder den
// Nullwert, falls keiner gesetzt ist.
//
// Plugins reichen ctx an alle Host-Aufrufe weiter – nur so kann der Host
// Log/Query/Exec dem auslösenden Handle-Aufruf zuordnen.
func CallFromContext(ctx context.Context) CallContext {
	c, _ := ctx.Value(callKey{}).(CallContext)
	return c
}

type hostKey struct{}

// WithHost hängt einen Host an ctx. Im Plugin setzt das SDK ihn automatisch
// für Configure und jeden Handle-Aufruf; in Unit-Tests eines Plugins lässt
// sich so ein Fake-Host einsetzen.
func WithHost(ctx context.Context, h Host) context.Context {
	return context.WithValue(ctx, hostKey{}, h)
}

// HasHost meldet, ob ctx einen Host trägt (z. B. false in Hintergrund-Goroutinen
// mit eigenem Kontext).
func HasHost(ctx context.Context) bool {
	h, ok := ctx.Value(hostKey{}).(Host)
	return ok && h != nil
}

// HostFrom liefert den Host des laufenden Aufrufs, z. B. um ein anderes
// Plugin aufzurufen:
//
//	resp, err := sdk.HostFrom(ctx).Handle(ctx, sdk.Request{Object: "BusinessPartner", Action: "get"})
//
// Das Ergebnis ist nie nil. Ist kein Host verfügbar (etwa vor Configure),
// liefern alle Methoden ErrUnavailable.
func HostFrom(ctx context.Context) Host {
	if h, ok := ctx.Value(hostKey{}).(Host); ok && h != nil {
		return h
	}
	return unavailableHost{}
}

type unavailableHost struct{}

var errNoHost = fmt.Errorf("%w: kein Host im Kontext (Plugin noch nicht konfiguriert?)", ErrUnavailable)

func (unavailableHost) Handle(context.Context, Request) (Response, error) {
	return Response{}, errNoHost
}
func (unavailableHost) Log(context.Context, LogLevel, string, map[string]string) error {
	return errNoHost
}
func (unavailableHost) Query(context.Context, string, string, ...any) (*QueryResult, error) {
	return nil, errNoHost
}
func (unavailableHost) Exec(context.Context, string, string, ...any) (ExecResult, error) {
	return ExecResult{}, errNoHost
}
func (unavailableHost) BeginTx(context.Context, string, sql.TxOptions) (string, error) {
	return "", errNoHost
}
func (unavailableHost) CommitTx(context.Context, string) error   { return errNoHost }
func (unavailableHost) RollbackTx(context.Context, string) error { return errNoHost }
