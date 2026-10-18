package metamodel

import "testing"

func TestLookupFor(t *testing.T) {
	l := &Lookup{Object: "RentObject", ValueField: "object_id", By: "object_type",
		Variants: map[string]Lookup{"Building": {Object: "Building", ValueField: "building_id"}}}
	values := map[string]string{"object_type": "Building"}
	if got := l.For(func(k string) string { return values[k] }); got.Object != "Building" || got.ValueField != "building_id" {
		t.Fatalf("Variante Building: %+v", got)
	}
	values["object_type"] = "RentObject"
	if got := l.For(func(k string) string { return values[k] }); got.Object != "RentObject" {
		t.Fatalf("ohne Variante: %+v", got)
	}
	if got := (&Lookup{Object: "X"}).For(nil); got.Object != "X" {
		t.Fatalf("ohne By: %+v", got)
	}
}
