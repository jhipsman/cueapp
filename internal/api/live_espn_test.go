package api

import (
	"testing"

	"github.com/jhipsman/cueapp/internal/iptv"
)

func TestMatchChannel(t *testing.T) {
	chans := []iptv.Channel{
		{ID: "a", Num: 4, Name: "US: CBS News 24/7"},
		{ID: "b", Num: 5, Name: "US: CBS New York HD"},
		{ID: "c", Num: 9, Name: "US: CBS Sports Network"},
		{ID: "d", Num: 20, Name: "US: ESPN2 HD"},
		{ID: "e", Num: 21, Name: "US: ESPN HD"},
		{ID: "f", Num: 30, Name: "US: FOX 5 New York"},
		{ID: "g", Num: 31, Name: "US: FOX Sports 1"},
		{ID: "h", Num: 32, Name: "US: FOX Business"},
		{ID: "i", Num: 3, Name: "US: CBS Los Angeles"},
	}
	for network, want := range map[string]string{
		"CBS": "i", "ESPN": "e", "ESPN2": "d", "FOX": "f", "FS1": "g", "CBSSN": "c", "Prime Video": "", "NBC": "",
	} {
		got := matchChannel(chans, network, nil)
		id := ""
		if got != nil {
			id = got.ID
		}
		if id != want {
			t.Errorf("%s: channel %q, want %q", network, id, want)
		}
	}
	// A favorite channel wins over a lower number.
	if got := matchChannel(chans, "CBS", map[string]bool{"b": true}); got == nil || got.ID != "b" {
		t.Fatalf("favorite: %+v", got)
	}
}
