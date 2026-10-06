package adapter

import (
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/camel/coremesh/pkg/sdk"
)

// toValue wandelt beliebige JSON-darstellbare Go-Werte in google.protobuf.Value.
// Structs und typisierte Maps/Slices gehen den Umweg über JSON.
func toValue(v any) (*structpb.Value, error) {
	switch v := v.(type) {
	case nil:
		return structpb.NewNullValue(), nil
	case *structpb.Value:
		return v, nil
	}
	if pv, err := structpb.NewValue(v); err == nil {
		return pv, nil
	}
	var generic any
	if err := sdk.Decode(v, &generic); err != nil {
		return nil, err
	}
	return structpb.NewValue(generic)
}

func fromValue(v *structpb.Value) any {
	if v == nil {
		return nil
	}
	return v.AsInterface()
}

func toValues(vs []any) ([]*structpb.Value, error) {
	out := make([]*structpb.Value, len(vs))
	for i, v := range vs {
		pv, err := toValue(v)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		out[i] = pv
	}
	return out, nil
}

func fromValues(vs []*structpb.Value) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = fromValue(v)
	}
	return out
}

func toStruct(m map[string]any) (*structpb.Struct, error) {
	if s, err := structpb.NewStruct(m); err == nil {
		return s, nil
	}
	var generic map[string]any
	if err := sdk.Decode(m, &generic); err != nil {
		return nil, err
	}
	return structpb.NewStruct(generic)
}
