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
