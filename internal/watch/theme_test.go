package watch

import "testing"

func TestThemeClean(t *testing.T) {
	got := Theme{Accent: "#FF0088", Glow: "loud", Background: "tinted"}.Clean()
	want := Theme{Accent: "#ff0088", Glow: "soft", Background: "tinted"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := parseTheme(""); got != DefaultTheme {
		t.Errorf("empty = %+v", got)
	}
	if got := (Theme{Accent: "red; background:url(x)"}).Clean(); got.Accent != DefaultTheme.Accent {
		t.Errorf("bad accent kept: %+v", got)
	}
}
