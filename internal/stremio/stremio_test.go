package stremio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBaseURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://comet.elfhosted.com/eyJrZXkiOiJ4In0=/manifest.json": "https://comet.elfhosted.com/eyJrZXkiOiJ4In0=",
		"stremio://torrentio.strem.fun/manifest.json":                "https://torrentio.strem.fun",
		" https://addon.example/abc/configure ":                      "https://addon.example/abc",
	} {
		if got, err := BaseURL(in); err != nil || got != want {
			t.Errorf("BaseURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "comet", "ftp://x/manifest.json"} {
		if _, err := BaseURL(bad); err == nil {
			t.Errorf("BaseURL(%q) accepted", bad)
		}
	}
}

func TestManifestAndStreams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cfg/manifest.json":
			_, _ = w.Write([]byte(`{"id":"comet","name":"Comet","version":"2.0","resources":["stream"],"types":["movie","series"]}`))
		case "/cfg/stream/series/tt0944947:1:2.json":
			_, _ = w.Write([]byte(`{"streams":[
				{"name":"[PM+] Comet 1080p","description":"Game.of.Thrones.S01E02.1080p.BluRay.x264\n💾 2.1 GB","url":"https://comet.example/playback/abc","behaviorHints":{"filename":"Game.of.Thrones.S01E02.1080p.BluRay.x264.mkv","videoSize":2254857830}},
				{"name":"Torrent","title":"Game.of.Thrones.S01.2160p\n👤 40","infoHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","fileIdx":1}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a, err := New(srv.URL + "/cfg/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.Manifest(context.Background())
	if err != nil || m.Name != "Comet" || !m.ServesStreams() {
		t.Fatalf("manifest = %+v, %v", m, err)
	}
	ss, err := a.Streams(context.Background(), "series", "tt0944947:1:2")
	if err != nil || len(ss) != 2 {
		t.Fatalf("streams = %+v, %v", ss, err)
	}
	if ss[0].Label() != "Game.of.Thrones.S01E02.1080p.BluRay.x264.mkv" || ss[0].URL == "" || ss[0].Hints.VideoSize != 2254857830 {
		t.Fatalf("first stream = %+v", ss[0])
	}
	if ss[1].Label() != "Game.of.Thrones.S01.2160p" || ss[1].InfoHash == "" || ss[1].FileIdx == nil || *ss[1].FileIdx != 1 {
		t.Fatalf("second stream = %+v", ss[1])
	}
	if _, err := a.Streams(context.Background(), "movie", "tt0000000"); err == nil {
		t.Fatal("a 404 wasn't an error")
	}
}
