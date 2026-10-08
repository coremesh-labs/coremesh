package main

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/coremesh-labs/coremesh/pkg/consoleapi/console/v1"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Read reicht einen Datenstrom (sdk.Reader) an die CLI weiter – Block für
// Block, ohne das Ergebnis im Speicher zu sammeln.
func (s *service) Read(req *consolev1.ExecuteRequest, stream consolev1.ConsoleService_ReadServer) error {
	object, action := req.GetTargetObject(), req.GetTargetAction()
	if object == "" || action == "" {
		return status.Error(codes.InvalidArgument, "target_object und target_action sind Pflicht")
	}
	ctx := stream.Context()
	cctx := s.userCtx(ctx, current(ctx))
	end, err := s.host.Read(cctx, sdk.Request{Object: object, Action: action, Payload: req.GetParameters().AsMap()}, chunkWriter{stream})
	if err != nil {
		return toStatus(err)
	}
	return stream.Send(&consolev1.ReadChunk{Part: &consolev1.ReadChunk_End{End: &consolev1.ReadSummary{Rows: end.Rows, Cursor: end.Cursor}}})
}

type chunkWriter struct {
	stream consolev1.ConsoleService_ReadServer
}

func (w chunkWriter) Header(h sdk.ReadHeader) error {
	return w.stream.Send(&consolev1.ReadChunk{Part: &consolev1.ReadChunk_Columns{Columns: &consolev1.ReadColumns{
		Names: h.Columns, Metadata: h.Metadata}}})
}

func (w chunkWriter) Rows(rows [][]any) error {
	out := make([]*structpb.ListValue, len(rows))
	for i, r := range rows {
		values := make([]*structpb.Value, len(r))
		for j, v := range r {
			pv, err := toValue(v)
			if err != nil {
				return fmt.Errorf("Zeile %d, Spalte %d: %w", i+1, j+1, err)
			}
			values[j] = pv
		}
		out[i] = &structpb.ListValue{Values: values}
	}
	return w.stream.Send(&consolev1.ReadChunk{Part: &consolev1.ReadChunk_Rows{Rows: &consolev1.ReadRows{Rows: out}}})
}
