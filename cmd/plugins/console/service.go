package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/coremesh-lab/coremesh/pkg/consoleapi/console/v1"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// service implementiert consolev1.ConsoleService.
//
// Ablauf je Aufruf (außer Login):
//  1. Token aus "authorization: Bearer …" → Session (im Speicher, mit Ablauf).
//  2. Account.Me beim Core-Plugin iam: Benutzer noch aktiv? Mandant?
//  3. Aufruf über den Host mit diesem Benutzer im CallContext. Der
//     Dispatcher prüft die Berechtigung, Module können Buchungskreise prüfen.
type service struct {
	consolev1.UnimplementedConsoleServiceServer

	host    sdk.Host
	cfg     settings
	blocked []string
	limits  extractLimits

	mu       sync.Mutex
	sessions map[string]session // SHA-256(Token) → Session
	limiter  *limiter
	now      func() time.Time
}

type session struct {
	userID   string
	username string
	expires  time.Time
}

type principal struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	TenantID    string `json:"tenant_id"`
}

func newService(host sdk.Host, cfg settings) *service {
	return &service{
		host: host, cfg: cfg,
		blocked:  append(systemDirectories(), cfg.BlockedDirectories...),
		limits:   extractLimits{MaxBytes: cfg.MaxExtractBytes, MaxFiles: cfg.MaxExtractFiles},
		sessions: map[string]session{},
		limiter:  newLimiter(),
		now:      time.Now,
	}
}

// --- Anmeldung -------------------------------------------------------------------

func (s *service) Login(ctx context.Context, req *consolev1.LoginRequest) (*consolev1.LoginResponse, error) {
	key := strings.ToLower(strings.TrimSpace(req.GetUsername())) + "|" + peerAddr(ctx)
	if s.limiter.blocked(key, s.now()) {
		return nil, status.Error(codes.ResourceExhausted, "Zu viele Fehlversuche – bitte später erneut versuchen")
	}
	resp, err := s.host.Handle(s.systemCtx(ctx), sdk.Request{Object: "Account", Action: "Authenticate",
		Payload: map[string]any{"username": req.GetUsername(), "password": req.GetPassword()}})
	if errors.Is(err, sdk.ErrPermissionDenied) {
		s.limiter.fail(key, s.now())
		return nil, status.Error(codes.Unauthenticated, "Benutzername oder Passwort falsch")
	}
	if err != nil {
		return nil, toStatus(err)
	}
	s.limiter.reset(key)
	var p principal
	if err := sdk.Decode(resp.Payload, &p); err != nil {
		return nil, toStatus(err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, toStatus(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := s.now().Add(s.cfg.tokenTTL)
	s.mu.Lock()
	s.sessions[hashToken(token)] = session{userID: p.ID, username: p.Username, expires: expires}
	s.cleanupLocked()
	s.mu.Unlock()

	name := p.DisplayName
	if name == "" {
		name = p.Username
	}
	return &consolev1.LoginResponse{Token: token, ExpiresUnix: expires.Unix(), DisplayName: name}, nil
}

func (s *service) Logout(ctx context.Context, _ *consolev1.LogoutRequest) (*consolev1.LogoutResponse, error) {
	if tok := bearer(ctx); tok != "" {
		s.mu.Lock()
		delete(s.sessions, hashToken(tok))
		s.mu.Unlock()
	}
	return &consolev1.LogoutResponse{}, nil
}

type principalKey struct{}

// authInterceptor verlangt für alle Methoden außer Login ein gültiges Token
// und legt den Benutzer (frisch aus iam) in den Kontext.
func (s *service) authInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
	if info.FullMethod == consolev1.ConsoleService_Login_FullMethodName {
		return h(ctx, req)
	}
	tok := bearer(ctx)
	s.mu.Lock()
	sess, ok := s.sessions[hashToken(tok)]
	if ok && !s.now().Before(sess.expires) {
		delete(s.sessions, hashToken(tok))
		ok = false
	}
	s.mu.Unlock()
	if tok == "" || !ok {
		return nil, status.Error(codes.Unauthenticated, "nicht angemeldet oder Anmeldung abgelaufen")
	}
	if info.FullMethod == consolev1.ConsoleService_Logout_FullMethodName {
		return h(ctx, req)
	}
	// Bei jedem Aufruf neu: deaktivierte oder gelöschte Benutzer sind sofort draußen.
	resp, err := s.host.Handle(s.userCtx(ctx, principal{ID: sess.userID, Username: sess.username}),
		sdk.Request{Object: "Account", Action: "Me"})
	if err != nil {
		s.mu.Lock()
		delete(s.sessions, hashToken(tok))
		s.mu.Unlock()
		return nil, status.Error(codes.Unauthenticated, "Benutzer nicht mehr aktiv")
	}
	var p principal
	if err := sdk.Decode(resp.Payload, &p); err != nil {
		return nil, toStatus(err)
	}
	return h(context.WithValue(ctx, principalKey{}, p), req)
}

// --- Execute ---------------------------------------------------------------------

func (s *service) Execute(ctx context.Context, req *consolev1.ExecuteRequest) (*consolev1.ExecuteResponse, error) {
	object, action := req.GetTargetObject(), req.GetTargetAction()
	params := req.GetParameters().AsMap()
	cctx := s.userCtx(ctx, current(ctx))
	// Konsolenbefehl eines Moduls: target_object = "<modul>:<befehl>" (z. B. ledger:load-coa).
	if mod, cmd, ok := strings.Cut(object, ":"); ok {
		c, list, err := s.command(cctx, mod, cmd, params)
		if err != nil {
			return nil, toStatus(err)
		}
		if c == nil { // <modul>: oder <modul>:help – Befehle auflisten
			v, err := toValue(map[string]any{"module": mod, "commands": list})
			if err != nil {
				return nil, toStatus(err)
			}
			return &consolev1.ExecuteResponse{Payload: v}, nil
		}
		object, action = c.Object, c.Action
	}
	if object == "" || action == "" {
		return nil, status.Error(codes.InvalidArgument, "target_object und target_action sind Pflicht (oder target_object <modul>:<befehl>)")
	}
	dir := req.GetTargetDirectory()
	if v, ok := params["target_directory"].(string); ok {
		if dir == "" {
			dir = v
		}
		delete(params, "target_directory") // nicht an das Modul weitergeben
	}

	resp, err := s.host.Handle(cctx, sdk.Request{Object: object, Action: action, Payload: params})
	if err != nil {
		return nil, toStatus(err)
	}

	payload, zipData, err := takeZip(resp.Payload)
	if err != nil {
		return nil, toStatus(err)
	}
	out := &consolev1.ExecuteResponse{}
	switch {
	case zipData != nil && dir != "":
		if err := s.mayExtract(cctx); err != nil {
			return nil, toStatus(err)
		}
		res, err := extractZip(zipData, dir, s.blocked, s.limits)
		if err != nil {
			return nil, toStatus(err)
		}
		out.Extract = &consolev1.ExtractResult{Directory: res.Directory, Files: int32(res.Files),
			Directories: int32(res.Directories), Bytes: res.Bytes}
		out.Message = fmt.Sprintf("%d Dateien (%d Bytes) nach %s entpackt", res.Files, res.Bytes, res.Directory)
		_ = s.host.Log(cctx, sdk.LogInfo, "ZIP entpackt", map[string]string{
			"object": object, "action": action, "directory": res.Directory,
			"files": fmt.Sprint(res.Files), "user": current(ctx).Username,
		})
	case zipData != nil:
		out.Message = fmt.Sprintf("Antwort enthält ein ZIP (%d Bytes); ohne target_directory wird es nicht entpackt", len(zipData))
	case dir != "":
		out.Message = "target_directory angegeben, aber die Antwort enthält kein zip_content"
	}
	if out.Payload, err = toValue(payload); err != nil {
		return nil, toStatus(err)
	}
	return out, nil
}

// mayExtract: Dateien auf den Server schreiben braucht die Berechtigung
// Console.ExtractZip (in irgendeinem Buchungskreis) – zusätzlich zur
// Berechtigung für die aufgerufene Action.
func (s *service) mayExtract(ctx context.Context) error {
	g, err := sdk.GrantedCompanyCodes(sdk.WithHost(ctx, s.host), "Console", "ExtractZip")
	if err != nil {
		return err
	}
	if g.None() {
		return fmt.Errorf("%w: Entpacken braucht die Berechtigung Console.ExtractZip", sdk.ErrPermissionDenied)
	}
	return nil
}

// takeZip entfernt zip_content aus der Antwort. Module liefern []byte; über
// das Plugin-Protokoll (google.protobuf.Value) kommt es Base64-kodiert an.
func takeZip(payload any) (rest any, zipData []byte, err error) {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload, nil, nil
	}
	raw, ok := m["zip_content"]
	if !ok {
		return payload, nil, nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		if k != "zip_content" {
			cp[k] = v
		}
	}
	switch v := raw.(type) {
	case []byte:
		return cp, v, nil
	case string:
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			if b, err := enc.DecodeString(v); err == nil {
				return cp, b, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("%w: zip_content muss Bytes (Base64) enthalten", sdk.ErrInvalidArgument)
}

// --- SampleFile -----------------------------------------------------------------

func (s *service) SampleFile(ctx context.Context, req *consolev1.SampleFileRequest) (*consolev1.SampleFileResponse, error) {
	if req.GetTargetObject() == "" {
		return nil, status.Error(codes.InvalidArgument, "target_object ist Pflicht")
	}
	resp, err := s.host.Handle(s.userCtx(ctx, current(ctx)), sdk.Request{
		Object: sdk.ObjectCatalog, Action: "GetDefinition", Payload: map[string]any{"object": req.GetTargetObject()}})
	if err != nil {
		return nil, toStatus(err)
	}
	var def struct {
		Definition metamodel.ObjectDefinition `json:"definition"`
	}
	if err := sdk.Decode(resp.Payload, &def); err != nil {
		return nil, toStatus(err)
	}
	content, format, ctype, err := renderSample(def.Definition, req.GetFormat())
	if err != nil {
		return nil, toStatus(err)
	}
	return &consolev1.SampleFileResponse{Content: content, Format: format, ContentType: ctype,
		Filename: sampleFilename(def.Definition.Name, format)}, nil
}

// --- Hilfsfunktionen ---------------------------------------------------------------

func current(ctx context.Context) principal {
	p, _ := ctx.Value(principalKey{}).(principal)
	return p
}

// userCtx: Aufrufkontext eines angemeldeten Benutzers (Ingress-Wurzelanfrage).
func (s *service) userCtx(ctx context.Context, p principal) context.Context {
	return sdk.WithCall(ctx, sdk.CallContext{RequestID: newID(), UserID: p.ID, TenantID: p.TenantID,
		Metadata: map[string]string{"ingress": name, "username": p.Username}})
}

// systemCtx: Aufruf ohne Benutzer (Login).
func (s *service) systemCtx(ctx context.Context) context.Context {
	return sdk.WithCall(ctx, sdk.CallContext{RequestID: newID(), Metadata: map[string]string{"ingress": name}})
}

func (s *service) cleanupLocked() {
	now := s.now()
	for k, v := range s.sessions {
		if !now.Before(v.expires) {
			delete(s.sessions, k)
		}
	}
}

func bearer(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	for _, v := range md.Get("authorization") {
		if t, ok := strings.CutPrefix(v, "Bearer "); ok {
			return strings.TrimSpace(t)
		}
	}
	return ""
}

func peerAddr(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return ""
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func toValue(v any) (*structpb.Value, error) {
	if pv, err := structpb.NewValue(v); err == nil {
		return pv, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return nil, err
	}
	return structpb.NewValue(generic)
}

// toStatus übersetzt SDK-Fehler in gRPC-Status für die CLI.
func toStatus(err error) error {
	for _, m := range []struct {
		err  error
		code codes.Code
	}{
		{sdk.ErrInvalidArgument, codes.InvalidArgument},
		{sdk.ErrNotFound, codes.NotFound},
		{sdk.ErrPermissionDenied, codes.PermissionDenied},
		{sdk.ErrUnimplemented, codes.Unimplemented},
		{sdk.ErrAlreadyExists, codes.AlreadyExists},
		{sdk.ErrFailedPrecondition, codes.FailedPrecondition},
		{sdk.ErrUnavailable, codes.Unavailable},
		{context.DeadlineExceeded, codes.DeadlineExceeded},
		{context.Canceled, codes.Canceled},
	} {
		if errors.Is(err, m.err) {
			return status.Error(m.code, err.Error())
		}
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Error(codes.Internal, err.Error())
}

// --- Schutz vor Brute Force (wie im WebServer) ---------------------------------------

type limiter struct {
	mu      sync.Mutex
	entries map[string]*attempts
}

type attempts struct {
	fails int
	until time.Time
}

func newLimiter() *limiter { return &limiter{entries: map[string]*attempts{}} }

func (l *limiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	return e != nil && now.Before(e.until)
}

func (l *limiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil {
		e = &attempts{}
		l.entries[key] = e
	}
	if e.fails++; e.fails >= 5 {
		e.until = now.Add(min(time.Minute<<min(e.fails-5, 4), 15*time.Minute))
	}
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

// command löst einen Konsolenbefehl <modul>:<befehl> über den Catalog auf
// (ModuleDefinition.Commands) und prüft die Parameter: Pflichtparameter
// vorhanden, keine unbekannten. Ohne Befehl (oder "help") liefert es nur die
// Liste der Befehle des Moduls.
func (s *service) command(ctx context.Context, module, name string, params map[string]any) (*metamodel.CommandDefinition, []metamodel.CommandDefinition, error) {
	resp, err := s.host.Handle(ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: "GetModule", Payload: map[string]any{"module": module}})
	if err != nil {
		return nil, nil, err
	}
	var mi struct {
		Commands []metamodel.CommandDefinition `json:"commands"`
	}
	if err := sdk.Decode(resp.Payload, &mi); err != nil {
		return nil, nil, err
	}
	if name == "" || name == "help" {
		return nil, mi.Commands, nil
	}
	i := slices.IndexFunc(mi.Commands, func(c metamodel.CommandDefinition) bool { return c.Name == name })
	if i < 0 {
		names := make([]string, len(mi.Commands))
		for j, c := range mi.Commands {
			names[j] = module + ":" + c.Name
		}
		return nil, nil, fmt.Errorf("%w: Befehl %s:%s – vorhanden: %s", sdk.ErrNotFound, module, name, strings.Join(names, ", "))
	}
	c := mi.Commands[i]
	for _, p := range c.Params {
		if _, ok := params[p.Name]; p.Required && !ok {
			return nil, nil, fmt.Errorf("%w: %s:%s braucht --%s", sdk.ErrInvalidArgument, module, name, p.Name)
		}
	}
	for k := range params {
		if !slices.ContainsFunc(c.Params, func(p metamodel.CommandParam) bool { return p.Name == k }) {
			return nil, nil, fmt.Errorf("%w: %s:%s kennt den Parameter --%s nicht", sdk.ErrInvalidArgument, module, name, k)
		}
	}
	return &c, nil, nil
}
