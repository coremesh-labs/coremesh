package main

import "testing"

func TestWorkOnly(t *testing.T) {
	m := moduleInfo{Name: "partner", Customizing: []string{"Group"},
		Objects: []moduleObject{{Object: "Group"}, {Object: "Partner"}}}
	if got := m.workOnly(&user{}); len(got.Objects) != 2 {
		t.Fatalf("ohne Person: %v", got.Objects)
	}
	got := m.workOnly(&user{PersonaHideSettings: true})
	if len(got.Objects) != 1 || got.Objects[0].Object != "Partner" {
		t.Fatalf("Person ohne Einstellungen: %v", got.Objects)
	}
	if len(m.Objects) != 2 {
		t.Fatal("Original verändert")
	}
}
