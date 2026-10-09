package crud

import "testing"

func TestFoldText(t *testing.T) {
	for in, want := range map[string]string{"Schloßallee": "schlossallee", "Schlossallee": "schlossallee", "MÜLLER": "mueller", "Mueller": "mueller"} {
		if got := foldText(in); got != want {
			t.Errorf("%s: %s, erwartet %s", in, got, want)
		}
	}
}
