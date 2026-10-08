package main

import (
	"context"
	"errors"
	"io"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/coremesh-labs/coremesh/pkg/consoleapi/console/v1"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Read des fakeHost: Item.list liefert drei Zeilen mit dem Benutzer.
func (h *fakeHost) Read(ctx context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	call := sdk.CallFromContext(ctx)
	h.mu.Lock()
	h.lastReq, h.lastCtx = req, call
	h.mu.Unlock()
	if req.Object+"."+req.Action != "Item.list" {
		return sdk.ReadEnd{}, sdk.ErrUnimplemented
	}
	if err := w.Header(sdk.ReadHeader{Columns: []string{"i", "user"}}); err != nil {
		return sdk.ReadEnd{}, err
	}
	if err := w.Rows([][]any{{1, call.UserID}, {2, call.UserID}}); err != nil {
		return sdk.ReadEnd{}, err
	}
	return sdk.ReadEnd{Rows: 3, Cursor: "c-2"}, w.Rows([][]any{{3, call.UserID}})
}

func readAll(ctx context.Context, c consolev1.ConsoleServiceClient, object, action string) ([]*consolev1.ReadChunk, error) {
	p, _ := structpb.NewStruct(map[string]any{"limit": 3})
	stream, err := c.Read(ctx, &consolev1.ExecuteRequest{TargetObject: object, TargetAction: action, Parameters: p})
	if err != nil {
		return nil, err
	}
	var out []*consolev1.ReadChunk
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, msg)
	}
}

func TestReadRequiresLogin(t *testing.T) {
	h := &fakeHost{t: t}
	c := startService(t, h)
	// Ohne Token: kein Strom – sonst liefe er als Systemanfrage ohne Rechteprüfung.
	if _, err := readAll(context.Background(), c, "Item", "list"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ohne Token: %v", err)
	}
	if h.lastReq.Object == "Item" {
		t.Fatal("Host wurde ohne Anmeldung aufgerufen")
	}

	chunks, err := readAll(login(t, c, "leser"), c, "Item", "list")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 4 || chunks[0].GetColumns().GetNames()[1] != "user" || len(chunks[1].GetRows().GetRows()) != 2 {
		t.Fatalf("Strom: %v", chunks)
	}
	if end := chunks[3].GetEnd(); end.GetRows() != 3 || end.GetCursor() != "c-2" {
		t.Fatalf("Ende: %v", end)
	}
	// Als angemeldeter Benutzer, mit den Parametern als Payload.
	if h.lastCtx.UserID != "u-leser" || h.lastReq.Payload.(map[string]any)["limit"] != float64(3) {
		t.Fatalf("Aufruf: %+v %+v", h.lastCtx, h.lastReq)
	}
	if v := chunks[1].GetRows().GetRows()[0].AsSlice(); v[1] != "u-leser" {
		t.Fatalf("Zeile: %v", v)
	}

	if _, err := readAll(login(t, c, "leser"), c, "Item", "get"); status.Code(err) != codes.Unimplemented {
		t.Fatalf("kein Datenstrom: %v", err)
	}
}
