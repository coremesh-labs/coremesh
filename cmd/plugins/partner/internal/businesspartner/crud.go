package businesspartner

import "github.com/coremesh-labs/coremesh/pkg/sdk/crud"

// Die Entities nutzen die generische CRUD-Engine des SDK (pkg/sdk/crud).
// Kurznamen für den Fachcode dieses Pakets:
type (
	record = crud.Record
	field  = crud.Field
	ref    = crud.Ref
	entity = crud.Entity
)

func str(v any) string                         { return crud.Str(v) }
func asBool(v any) bool                        { return crud.AsBool(v) }
func today() string                            { return crud.Today() }
func parseDate(v any) (string, error)          { return crud.ParseDate(v) }
func invalid(format string, args ...any) error { return crud.Invalid(format, args...) }
