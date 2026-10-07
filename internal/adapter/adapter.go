package adapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/coremesh-labs/coremesh/internal/api/plugin/v1"
	"github.com/coremesh-labs/coremesh/internal/txctx"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// GRPCPlugin verbindet sdk.Plugin mit HashiCorp go-plugin.
// Impl ist nur auf Plugin-Seite gesetzt.
type GRPCPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	Impl sdk.Plugin
}

var _ goplugin.GRPCPlugin = (*GRPCPlugin)(nil)

func (p *GRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterPluginServiceServer(s, &pluginServer{impl: p.Impl, broker: broker})
	return nil
}

func (p *GRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &pluginClient{client: pluginv1.NewPluginServiceClient(c), broker: broker}, nil
}

// ---------------------------------------------------------------------------
// Plugin-Seite: PluginService-Server, ruft die Plugin-Implementierung auf.

type pluginServer struct {
	pluginv1.UnimplementedPluginServiceServer
	impl   sdk.Plugin
	broker *goplugin.GRPCBroker

	mu       sync.Mutex
	hostConn *grpc.ClientConn
	host     sdk.Host // nil bis zum ersten Configure
}

// withHost legt den aktuellen Host-Client in ctx, damit das Plugin ihn per
// sdk.HostFrom(ctx) erreicht, ohne ihn selbst speichern zu müssen.
func (s *pluginServer) withHost(ctx context.Context) context.Context {
	s.mu.Lock()
	h := s.host
	s.mu.Unlock()
	if h == nil {
		return ctx
	}
	return sdk.WithHost(ctx, h)
}

func (s *pluginServer) GetManifest(ctx context.Context, req *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	m, err := s.impl.Manifest(ctxFromProto(ctx, req.GetContext()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pluginv1.GetManifestResponse{Manifest: manifestToProto(m)}, nil
}

func (s *pluginServer) Configure(ctx context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	if req.GetHostServiceBrokerId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "host_service_broker_id fehlt")
	}
	conn, err := s.broker.Dial(req.GetHostServiceBrokerId())
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "HostService nicht erreichbar: %v", err)
	}
	host := &hostClient{client: pluginv1.NewHostServiceClient(conn)}

	// Bei erneutem Configure die alte Rückverbindung ersetzen und schließen.
	s.mu.Lock()
	old := s.hostConn
	s.hostConn, s.host = conn, host
	s.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}

	ctx = sdk.WithHost(ctxFromProto(ctx, req.GetContext()), host)
	cfg := sdk.Config{Settings: req.GetSettings().AsMap(), Host: host}
	if err := s.impl.Configure(ctx, cfg); err != nil {
		return nil, toStatus(err)
	}
	return &pluginv1.ConfigureResponse{}, nil
}

func (s *pluginServer) Handle(ctx context.Context, req *pluginv1.HandleRequest) (*pluginv1.HandleResponse, error) {
	return serveHandle(s.withHost(ctx), s.impl, req)
}

// ---------------------------------------------------------------------------
// Host-Seite: PluginService-Client, implementiert sdk.Plugin über gRPC.

type pluginClient struct {
	client pluginv1.PluginServiceClient
	broker *goplugin.GRPCBroker
}

var _ sdk.Plugin = (*pluginClient)(nil)

func (c *pluginClient) Manifest(ctx context.Context) (sdk.Manifest, error) {
	resp, err := c.client.GetManifest(ctx, &pluginv1.GetManifestRequest{
		Context: ctxToProto(ctx),
	})
	if err != nil {
		return sdk.Manifest{}, fromStatus(err)
	}
	return manifestFromProto(resp.GetManifest()), nil
}

// Configure startet den HostService auf einer neuen Broker-Verbindung und
// übergibt deren ID an das Plugin, das sich damit zurückverbindet.
func (c *pluginClient) Configure(ctx context.Context, cfg sdk.Config) error {
	if cfg.Host == nil {
		return errors.New("adapter: Config.Host darf auf Host-Seite nicht nil sein")
	}
	settings, err := toStruct(cfg.Settings)
	if err != nil {
		return fmt.Errorf("adapter: settings: %w", err)
	}

	id := c.broker.NextId()
	go c.broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		pluginv1.RegisterHostServiceServer(s, &hostServer{impl: cfg.Host})
		return s
	})

	_, err = c.client.Configure(ctx, &pluginv1.ConfigureRequest{
		Context:             ctxToProto(ctx),
		Settings:            settings,
		HostServiceBrokerId: id,
	})
	return fromStatus(err)
}

func (c *pluginClient) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	return callHandle(ctx, req, c.client.Handle)
}

// ---------------------------------------------------------------------------
// Host-Seite: HostService-Server, ruft die Host-Implementierung aus internal/ auf.

type hostServer struct {
	pluginv1.UnimplementedHostServiceServer
	impl sdk.Host
}

// Dispatch übergibt die Anfrage eines Plugins an Host.Handle (Dispatcher).
func (s *hostServer) Dispatch(ctx context.Context, req *pluginv1.HandleRequest) (*pluginv1.HandleResponse, error) {
	return serveHandle(ctx, s.impl, req)
}

func (s *hostServer) Log(ctx context.Context, req *pluginv1.LogRequest) (*pluginv1.LogResponse, error) {
	ctx = ctxFromProto(ctx, req.GetContext())
	if err := s.impl.Log(ctx, sdk.LogLevel(req.GetLevel()), req.GetMessage(), req.GetFields()); err != nil {
		return nil, toStatus(err)
	}
	return &pluginv1.LogResponse{}, nil
}

func (s *hostServer) Query(ctx context.Context, req *pluginv1.QueryRequest) (*pluginv1.QueryResponse, error) {
	ctx = ctxFromProto(ctx, req.GetContext())
	res, err := s.impl.Query(ctx, req.GetDatabase(), req.GetSql(), fromValues(req.GetArgs())...)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &pluginv1.QueryResponse{Columns: res.Columns, Rows: make([]*pluginv1.Row, len(res.Rows))}
	for i, row := range res.Rows {
		values, err := toValues(row)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Zeile %d: %v", i, err)
		}
		resp.Rows[i] = &pluginv1.Row{Values: values}
	}
	return resp, nil
}

func (s *hostServer) Exec(ctx context.Context, req *pluginv1.ExecRequest) (*pluginv1.ExecResponse, error) {
	ctx = ctxFromProto(ctx, req.GetContext())
	res, err := s.impl.Exec(ctx, req.GetDatabase(), req.GetSql(), fromValues(req.GetArgs())...)
	if err != nil {
		return nil, toStatus(err)
	}
	return &pluginv1.ExecResponse{RowsAffected: res.RowsAffected, LastInsertId: res.LastInsertID}, nil
}

// txManager liefert die Transaktionssteuerung der Host-Implementierung.
func (s *hostServer) txManager() (txctx.Manager, error) {
	if m, ok := s.impl.(txctx.Manager); ok {
		return m, nil
	}
	return nil, status.Error(codes.Unimplemented, "Host unterstützt keine Transaktionen")
}

func (s *hostServer) BeginTx(ctx context.Context, req *pluginv1.BeginTxRequest) (*pluginv1.BeginTxResponse, error) {
	m, err := s.txManager()
	if err != nil {
		return nil, err
	}
	opts := sql.TxOptions{Isolation: sql.IsolationLevel(req.GetIsolation()), ReadOnly: req.GetReadOnly()}
	id, err := m.BeginTx(ctxFromProto(ctx, req.GetContext()), req.GetDatabase(), opts)
	if err != nil {
		return nil, toStatus(err)
	}
	return &pluginv1.BeginTxResponse{TxId: id}, nil
}

func (s *hostServer) CommitTx(ctx context.Context, req *pluginv1.CommitTxRequest) (*pluginv1.CommitTxResponse, error) {
	m, err := s.txManager()
	if err != nil {
		return nil, err
	}
	if err := m.CommitTx(ctxFromProto(ctx, req.GetContext()), req.GetTxId()); err != nil {
		return nil, toStatus(err)
	}
	return &pluginv1.CommitTxResponse{}, nil
}

func (s *hostServer) RollbackTx(ctx context.Context, req *pluginv1.RollbackTxRequest) (*pluginv1.RollbackTxResponse, error) {
	m, err := s.txManager()
	if err != nil {
		return nil, err
	}
	if err := m.RollbackTx(ctxFromProto(ctx, req.GetContext()), req.GetTxId()); err != nil {
		return nil, toStatus(err)
	}
	return &pluginv1.RollbackTxResponse{}, nil
}

// ---------------------------------------------------------------------------
// Plugin-Seite: HostService-Client, implementiert sdk.Host über gRPC.

type hostClient struct {
	client pluginv1.HostServiceClient
}

var _ sdk.Host = (*hostClient)(nil)

// Handle ruft über den Host ein anderes Plugin auf.
func (c *hostClient) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	return callHandle(ctx, req, c.client.Dispatch)
}

func (c *hostClient) Log(ctx context.Context, level sdk.LogLevel, msg string, fields map[string]string) error {
	_, err := c.client.Log(ctx, &pluginv1.LogRequest{
		Context: ctxToProto(ctx),
		Level:   pluginv1.LogLevel(level),
		Message: msg,
		Fields:  fields,
	})
	return fromStatus(err)
}

func (c *hostClient) Query(ctx context.Context, database, sql string, args ...any) (*sdk.QueryResult, error) {
	pargs, err := toValues(args)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Query(ctx, &pluginv1.QueryRequest{
		Context:  ctxToProto(ctx),
		Database: database,
		Sql:      sql,
		Args:     pargs,
	})
	if err != nil {
		return nil, fromStatus(err)
	}
	res := &sdk.QueryResult{Columns: resp.GetColumns(), Rows: make([][]any, len(resp.GetRows()))}
	for i, row := range resp.GetRows() {
		res.Rows[i] = fromValues(row.GetValues())
	}
	return res, nil
}

func (c *hostClient) Exec(ctx context.Context, database, sql string, args ...any) (sdk.ExecResult, error) {
	pargs, err := toValues(args)
	if err != nil {
		return sdk.ExecResult{}, err
	}
	resp, err := c.client.Exec(ctx, &pluginv1.ExecRequest{
		Context:  ctxToProto(ctx),
		Database: database,
		Sql:      sql,
		Args:     pargs,
	})
	if err != nil {
		return sdk.ExecResult{}, fromStatus(err)
	}
	return sdk.ExecResult{RowsAffected: resp.GetRowsAffected(), LastInsertID: resp.GetLastInsertId()}, nil
}

var _ txctx.Manager = (*hostClient)(nil)

func (c *hostClient) BeginTx(ctx context.Context, database string, opts sql.TxOptions) (string, error) {
	resp, err := c.client.BeginTx(ctx, &pluginv1.BeginTxRequest{
		Context:   ctxToProto(ctx),
		Database:  database,
		Isolation: pluginv1.IsolationLevel(opts.Isolation),
		ReadOnly:  opts.ReadOnly,
	})
	if err != nil {
		return "", fromStatus(err)
	}
	return resp.GetTxId(), nil
}

func (c *hostClient) CommitTx(ctx context.Context, txID string) error {
	_, err := c.client.CommitTx(ctx, &pluginv1.CommitTxRequest{Context: ctxToProto(ctx), TxId: txID})
	return fromStatus(err)
}

func (c *hostClient) RollbackTx(ctx context.Context, txID string) error {
	_, err := c.client.RollbackTx(ctx, &pluginv1.RollbackTxRequest{Context: ctxToProto(ctx), TxId: txID})
	return fromStatus(err)
}

// ---------------------------------------------------------------------------
// Handle-Aufrufe in beide Richtungen: Host -> Plugin (PluginService.Handle)
// und Plugin -> Host -> Plugin (HostService.Dispatch) teilen dieselbe Form.

// serveHandle stellt einen sdk.Handler hinter einem gRPC-Server bereit.
func serveHandle(ctx context.Context, h sdk.Handler, req *pluginv1.HandleRequest) (*pluginv1.HandleResponse, error) {
	ctx = ctxFromProto(ctx, req.GetContext())
	resp, err := h.Handle(ctx, sdk.Request{
		Object:  req.GetObject(),
		Action:  req.GetAction(),
		Payload: fromValue(req.GetPayload()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	payload, err := toValue(resp.Payload)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Antwort-Payload nicht serialisierbar: %v", err)
	}
	return &pluginv1.HandleResponse{
		Context: &pluginv1.Context{RequestId: req.GetContext().GetRequestId(), Metadata: resp.Metadata},
		Payload: payload,
	}, nil
}

type handleRPC func(context.Context, *pluginv1.HandleRequest, ...grpc.CallOption) (*pluginv1.HandleResponse, error)

// callHandle ruft einen entfernten Handler auf; der CallContext kommt aus ctx.
func callHandle(ctx context.Context, req sdk.Request, rpc handleRPC) (sdk.Response, error) {
	payload, err := toValue(req.Payload)
	if err != nil {
		return sdk.Response{}, err
	}
	resp, err := rpc(ctx, &pluginv1.HandleRequest{
		Context: ctxToProto(ctx),
		Object:  req.Object,
		Action:  req.Action,
		Payload: payload,
	})
	if err != nil {
		return sdk.Response{}, fromStatus(err)
	}
	return sdk.Response{
		Payload:  fromValue(resp.GetPayload()),
		Metadata: resp.GetContext().GetMetadata(),
	}, nil
}

// ---------------------------------------------------------------------------
// Übersetzung zwischen öffentlichen SDK-Typen und internen Protobuf-Typen.

// ctxToProto baut die Protobuf-Message Context aus CallContext und den
// laufenden Transaktionen in ctx.
func ctxToProto(ctx context.Context) *pluginv1.Context {
	c := sdk.CallFromContext(ctx)
	return &pluginv1.Context{
		RequestId: c.RequestID,
		TenantId:  c.TenantID,
		UserId:    c.UserID,
		Metadata:  c.Metadata,
		TxIds:     txctx.All(ctx),
	}
}

// ctxFromProto legt CallContext und Transaktionen einer empfangenen
// Context-Message in ctx ab.
func ctxFromProto(ctx context.Context, c *pluginv1.Context) context.Context {
	ctx = sdk.WithCall(ctx, sdk.CallContext{
		RequestID: c.GetRequestId(),
		TenantID:  c.GetTenantId(),
		UserID:    c.GetUserId(),
		Metadata:  c.GetMetadata(),
	})
	return txctx.WithAll(ctx, c.GetTxIds())
}

func manifestToProto(m sdk.Manifest) *pluginv1.Manifest {
	caps := make([]*pluginv1.Capability, len(m.Capabilities))
	for i, c := range m.Capabilities {
		caps[i] = &pluginv1.Capability{Object: c.Object, Actions: c.Actions, Description: c.Description}
	}
	return &pluginv1.Manifest{Name: m.Name, Version: m.Version, Description: m.Description, Capabilities: caps}
}

func manifestFromProto(m *pluginv1.Manifest) sdk.Manifest {
	caps := make([]sdk.Capability, len(m.GetCapabilities()))
	for i, c := range m.GetCapabilities() {
		caps[i] = sdk.Capability{Object: c.GetObject(), Actions: c.GetActions(), Description: c.GetDescription()}
	}
	return sdk.Manifest{Name: m.GetName(), Version: m.GetVersion(), Description: m.GetDescription(), Capabilities: caps}
}
