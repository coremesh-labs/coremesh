package sdk

import "errors"

// Sentinel-Fehler, die über die Prozessgrenze übertragen und auf der
// Gegenseite wieder mit errors.Is erkannt werden.
var (
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrNotFound         = errors.New("not found")
	ErrAlreadyExists    = errors.New("already exists")
	ErrPermissionDenied = errors.New("permission denied")
	ErrUnimplemented    = errors.New("unimplemented")
	ErrUnavailable      = errors.New("unavailable")
	// ErrFailedPrecondition meldet z. B. eine zu tiefe Plugin-Aufrufkette.
	ErrFailedPrecondition = errors.New("failed precondition")
	// ErrTxAborted: Die Transaktion wurde zurückgerollt (von einem Teilnehmer,
	// wegen Zeitüberschreitung oder Absturz) und ist nicht mehr nutzbar.
	ErrTxAborted = errors.New("transaction aborted")
	// ErrResourceExhausted: z. B. zu viele offene Transaktionen.
	ErrResourceExhausted = errors.New("resource exhausted")
)
