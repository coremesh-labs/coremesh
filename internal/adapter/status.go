package adapter

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

var sentinelCodes = []struct {
	err  error
	code codes.Code
}{
	{sdk.ErrInvalidArgument, codes.InvalidArgument},
	{sdk.ErrNotFound, codes.NotFound},
	{sdk.ErrAlreadyExists, codes.AlreadyExists},
	{sdk.ErrPermissionDenied, codes.PermissionDenied},
	{sdk.ErrUnimplemented, codes.Unimplemented},
	{sdk.ErrUnavailable, codes.Unavailable},
	{sdk.ErrFailedPrecondition, codes.FailedPrecondition},
	{sdk.ErrTxAborted, codes.Aborted},
	{sdk.ErrResourceExhausted, codes.ResourceExhausted},
	{context.Canceled, codes.Canceled},
	{context.DeadlineExceeded, codes.DeadlineExceeded},
}

// toStatus wandelt einen Go-Fehler in einen gRPC-Status für die Übertragung.
func toStatus(err error) error {
	if err == nil {
		return nil
	}
	for _, s := range sentinelCodes {
		if errors.Is(err, s.err) {
			return status.Error(s.code, err.Error())
		}
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Error(codes.Internal, err.Error())
}

// fromStatus wandelt einen empfangenen gRPC-Status zurück in einen Fehler, der
// mit errors.Is auf den passenden sdk.Err*-Sentinel matcht.
func fromStatus(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	for _, s := range sentinelCodes {
		if st.Code() == s.code {
			return &statusError{st: st, sentinel: s.err}
		}
	}
	return err
}

type statusError struct {
	st       *status.Status
	sentinel error
}

func (e *statusError) Error() string              { return e.st.Message() }
func (e *statusError) Unwrap() error              { return e.sentinel }
func (e *statusError) GRPCStatus() *status.Status { return e.st }
