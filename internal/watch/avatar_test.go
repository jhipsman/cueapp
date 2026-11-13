package watch

import "testing"

func TestCleanAvatar(t *testing.T) {
	for in, want := range map[string]string{
		"red":          "red",
		"red/fox":      "red/fox",
		"red/dragon":   "red",      // not a picture Cue has
		"mauve/fox":    "teal/fox", // not a color Cue has
		"":             "teal",
		"/rocket":      "teal/rocket",
		"blue/fox/cat": "blue",
	} {
		if got := cleanAvatar(in); got != want {
			t.Errorf("cleanAvatar(%q) = %q, want %q", in, got, want)
		}
	}
}
