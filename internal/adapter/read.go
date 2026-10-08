package adapter

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/coremesh-labs/coremesh/internal/api/plugin/v1"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Read-Ströme in beide Richtungen: Host -> Plugin (PluginService.Read) und
// Plugin -> Host -> Plugin (HostService.DispatchRead) teilen dieselbe Form –
// wie Handle und Dispatch.

// Grenzen eines batch: deutlich unter der gRPC-Grenze von 4 MiB je Nachricht.
const (
	maxBatchBytes = 1 << 20
	maxBatchRows  = 5000
)

// Plugin-Seite: PluginService.Read ruft sdk.Reader der Implementierung auf.
func (s *pluginServer) Read(req *pluginv1.HandleRequest, stream pluginv1.PluginService_ReadServer) error {
	r, ok := s.impl.(sdk.Reader)
	if !ok {
		return status.Errorf(codes.Unimplemented, "Plugin unterstützt Read nicht (%s.%s)", req.GetObject(), req.GetAction())
	}
	return serveRead(s.withHost(stream.Context()), r, req, stream.Send)
}

// Host-Seite: sdk.Reader über PluginService.Read.
func (c *pluginClient) Read(ctx context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	return callRead(ctx, req, w, func(ctx context.Context, in *pluginv1.HandleRequest) (recvStream, error) {
		return c.client.Read(ctx, in)
	})
}

// Host-Seite: HostService.DispatchRead übergibt die Anfrage an Host.Read (Dispatcher).
func (s *hostServer) DispatchRead(req *pluginv1.HandleRequest, stream pluginv1.HostService_DispatchReadServer) error {
	return serveRead(stream.Context(), s.impl, req, stream.Send)
}

// Plugin-Seite: sdk.Host.Read über HostService.DispatchRead.
func (c *hostClient) Read(ctx context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	return callRead(ctx, req, w, func(ctx context.Context, in *pluginv1.HandleRequest) (recvStream, error) {
		return c.client.DispatchRead(ctx, in)
	})
}

// serveRead stellt einen sdk.Reader hinter einem gRPC-Server-Stream bereit.
func serveRead(ctx context.Context, r sdk.Reader, req *pluginv1.HandleRequest, send func(*pluginv1.ReadResponse) error) error {
	ctx = ctxFromProto(ctx, req.GetContext())
	gw := &grpcWriter{send: send, rctx: &pluginv1.Context{RequestId: req.GetContext().GetRequestId()}}
	sw := &sdk.StrictWriter{W: gw}
	end, err := r.Read(ctx, sdk.Request{Object: req.GetObject(), Action: req.GetAction(), Payload: fromValue(req.GetPayload())}, sw)
	if err != nil {
		return toStatus(err)
	}
	if end, err = sw.Finish(end); err != nil {
		return toStatus(err)
	}
	if err := gw.flush(); err != nil {
		return err
	}
	return send(&pluginv1.ReadResponse{Context: gw.take(), Part: &pluginv1.ReadResponse_End{End: &pluginv1.ReadEnd{
		Rows: end.Rows, Cursor: end.Cursor, Metadata: end.Metadata}}})
}

// grpcWriter übersetzt RowWriter-Aufrufe in Nachrichten. Zeilen werden zu
// batches bis maxBatchBytes/maxBatchRows gesammelt; Send blockiert, solange
// der Empfänger nicht nachkommt (Gegendruck über gRPC-Flusskontrolle).
type grpcWriter struct {
	send  func(*pluginv1.ReadResponse) error
	rctx  *pluginv1.Context // nur in der ersten Nachricht
	batch []*pluginv1.ReadRow
	size  int
}

func (g *grpcWriter) take() *pluginv1.Context {
	c := g.rctx
	g.rctx = nil
	return c
}

func (g *grpcWriter) Header(h sdk.ReadHeader) error {
	return g.send(&pluginv1.ReadResponse{Context: g.take(), Part: &pluginv1.ReadResponse_Header{Header: &pluginv1.ReadHeader{
		Columns: h.Columns, Metadata: h.Metadata}}})
}

func (g *grpcWriter) Rows(rows [][]any) error {
	for i, r := range rows {
		values, err := toValues(r)
		if err != nil {
			return fmt.Errorf("Read: Zeile %d: %w", i+1, err)
		}
		row := &pluginv1.ReadRow{Values: values}
		n := proto.Size(row)
		if n > maxBatchBytes {
			return fmt.Errorf("Read: Zeile %d ist mit %d Bytes zu groß (max. %d)", i+1, n, maxBatchBytes)
		}
		if g.size+n > maxBatchBytes {
			if err := g.flush(); err != nil {
				return err
			}
		}
		g.batch, g.size = append(g.batch, row), g.size+n
		if len(g.batch) >= maxBatchRows {
			if err := g.flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *grpcWriter) flush() error {
	if len(g.batch) == 0 {
		return nil
	}
	msg := &pluginv1.ReadResponse{Context: g.take(), Part: &pluginv1.ReadResponse_Batch{Batch: &pluginv1.ReadBatch{Rows: g.batch}}}
	g.batch, g.size = nil, 0
	return g.send(msg)
}

type recvStream interface {
	Recv() (*pluginv1.ReadResponse, error)
}

type readRPC func(context.Context, *pluginv1.HandleRequest) (recvStream, error)

// callRead ruft einen entfernten Reader auf und reicht den Strom an w weiter.
// Bricht w ab, wird der Strom beendet (ctx) und der Fehler von w geliefert.
func callRead(ctx context.Context, req sdk.Request, w sdk.RowWriter, rpc readRPC) (sdk.ReadEnd, error) {
	payload, err := toValue(req.Payload)
	if err != nil {
		return sdk.ReadEnd{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := rpc(ctx, &pluginv1.HandleRequest{
		Context: ctxToProto(ctx),
		Object:  req.Object,
		Action:  req.Action,
		Payload: payload,
	})
	if err != nil {
		return sdk.ReadEnd{}, fromStatus(err)
	}
	sw := &sdk.StrictWriter{W: w}
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return sdk.ReadEnd{}, fmt.Errorf("%w: Read %s.%s: Strom ohne Abschluss", sdk.ErrUnavailable, req.Object, req.Action)
		}
		if err != nil {
			return sdk.ReadEnd{}, fromStatus(err)
		}
		switch p := msg.GetPart().(type) {
		case *pluginv1.ReadResponse_Header:
			err = sw.Header(sdk.ReadHeader{Columns: p.Header.GetColumns(), Metadata: p.Header.GetMetadata()})
		case *pluginv1.ReadResponse_Batch:
			rows := make([][]any, len(p.Batch.GetRows()))
			for i, r := range p.Batch.GetRows() {
				rows[i] = fromValues(r.GetValues())
			}
			err = sw.Rows(rows)
		case *pluginv1.ReadResponse_End:
			end := sdk.ReadEnd{Rows: p.End.GetRows(), Cursor: p.End.GetCursor(), Metadata: p.End.GetMetadata()}
			return sw.Finish(end)
		default:
			err = fmt.Errorf("Read %s.%s: unbekannte Nachricht", req.Object, req.Action)
		}
		if err != nil {
			return sdk.ReadEnd{}, err
		}
	}
}
