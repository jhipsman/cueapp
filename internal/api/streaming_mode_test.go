package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamingOnlySwitch(t *testing.T) {
	s := newBareServer(t)
	if s.streamingOnly() || !s.automationEnabled() {
		t.Fatal("a server made without CUE_STREAMING_ONLY should keep the full manager")
	}
	s.cfg.StreamingOnly = true // as the real app starts
	if !s.streamingOnly() || s.automationEnabled() {
		t.Fatal("streaming only by default: the automatic searches should rest")
	}
	if !s.modulesPayload().StreamingOnly {
		t.Fatal("the modules answer doesn't say streaming only")
	}
	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handlePutStreamingSettings(w, httptest.NewRequest("PUT", "/", strings.NewReader(body)))
		return w
	}
	if w := put(`{"streamingOnly":false,"maxResolution":"720"}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"maxResolution":"720"`) {
		t.Fatalf("put: %d %s", w.Code, w.Body)
	}
	if s.streamingOnly() || s.playbackMaxRes() != 720 {
		t.Fatal("switching it off in Settings should win over the default")
	}
	if w := put(`{"maxResolution":"4k"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad resolution: %d", w.Code)
	}
}

func TestPreferResolutionPutsBigVersionsLast(t *testing.T) {
	in := []playCandidate{
		{Release: "Film.2020.2160p.WEB-DL"}, {Release: "Film.2020.1080p.WEB-DL"},
		{Release: "Film.2020.720p.WEB"}, {Release: "Film.2020.2160p.BluRay.REMUX"}, {Release: "Film.2020.WEBRip"},
	}
	got := preferResolution(in, 1080)
	order := []string{}
	for _, c := range got {
		order = append(order, c.Release)
	}
	want := "Film.2020.1080p.WEB-DL,Film.2020.720p.WEB,Film.2020.WEBRip,Film.2020.2160p.WEB-DL,Film.2020.2160p.BluRay.REMUX"
	if strings.Join(order, ",") != want {
		t.Fatalf("order = %v", order)
	}
	if got := preferResolution(in, 0); got[0].Release != in[0].Release {
		t.Fatal("no cap should keep the order")
	}
}

func TestBrowserAudioFirst(t *testing.T) {
	cands := []playCandidate{
		{Release: "Movie.2019.2160p.UHD.BluRay.TrueHD.Atmos.7.1.x265", URL: "u1"},
		{Release: "Movie.2019.1080p.WEB-DL.DDP5.1.H.264", URL: "u2"},
		{Release: "Movie.2019.1080p.BluRay.DTS-HD.MA.5.1", URL: "u3"},
		{Release: "Movie.2019.1080p.WEBRip.AAC5.1.x264", URL: "u4"},
		{Release: "Movie.2019.2160p.BluRay.DTS", Hash: "abc"}, // Premiumize's own stream: fine
		{Release: "Movie.2019.720p.HDTV.x264", URL: "u5"},
		{Release: "Addams.Family.1991.1080p", URL: "u6"}, // "dd" inside a word isn't Dolby
	}
	var got []string
	for _, c := range browserAudioFirst(cands, false) {
		got = append(got, c.URL+c.Hash)
	}
	want := "u4 abc u5 u6 u1 u2 u3"
	if strings.Join(got, " ") != want {
		t.Fatalf("order = %v, want %s", got, want)
	}
}
