package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rdborg/mediarium/internal/indexers"
	"github.com/rdborg/mediarium/internal/library"
	"github.com/rdborg/mediarium/internal/premiumize"
	"github.com/rdborg/mediarium/internal/settings"
)

const (
	hash4K    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hash1080  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	hash720   = "cccccccccccccccccccccccccccccccccccccccc"
	gigabytes = int64(1) << 30
)

// fakeTorznab answers every search with the same three releases of one movie.
func fakeTorznab(t *testing.T) *httptest.Server {
	t.Helper()
	item := func(title, hash string, size int64) string {
		return fmt.Sprintf(`<item><title>%s</title><guid>%s</guid><link>magnet:?xt=urn:btih:%s</link>
<enclosure url="magnet:?xt=urn:btih:%s" length="%d" type="application/x-bittorrent"/>
<torznab:attr name="infohash" value="%s"/><torznab:attr name="seeders" value="50"/><torznab:attr name="category" value="2000"/></item>`,
			title, hash, hash, hash, size, hash)
	}
	feed := `<?xml version="1.0"?><rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed"><channel>` +
		item("Some.Movie.2020.2160p.WEB-DL.x265-GRP", hash4K, 20*gigabytes) +
		item("Some.Movie.2020.1080p.BluRay.x264-GRP", hash1080, 10*gigabytes) +
		item("Some.Movie.2020.720p.WEB.x264-GRP", hash720, 4*gigabytes) +
		item("Other.Film.2020.1080p.WEB.x264-GRP", "dddddddddddddddddddddddddddddddddddddddd", 8*gigabytes) +
		`</channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(feed))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakePremiumizeCache has the 1080p and 720p releases cached, not the 4K one.
func fakePremiumizeCache(t *testing.T) *httptest.Server {
	t.Helper()
	cached := map[string]bool{hash1080: true, hash720: true}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/cache/check":
			var resp []bool
			for _, h := range r.Form["items[]"] {
				resp = append(resp, cached[h])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "response": resp})
		case "/transfer/directdl":
			src := r.Form.Get("src")
			h := src[strings.LastIndex(src, ":")+1:]
			if !cached[h] {
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "message": "not cached"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "content": []map[string]any{
				{"path": "Some.Movie/sample.mkv", "size": 50 << 20, "link": "https://cdn.example/sample-" + h},
				{"path": "Some.Movie/Some.Movie.mkv", "size": 9 << 30, "link": "https://cdn.example/" + h, "stream_link": "https://cdn.example/" + h + ".mp4"},
			}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "message": "unexpected " + r.URL.Path})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPlayMoviePicksBestCachedRelease(t *testing.T) {
	s := newBareServer(t)
	s.premiumizeBase = fakePremiumizeCache(t).URL
	if err := s.Settings.Set(settings.KeyPremiumizeAPIKey, "k", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.IndexerRepo.Create(indexers.Instance{Name: "T", BaseURL: fakeTorznab(t).URL, APIKey: "x", Protocol: indexers.ProtocolTorrent, Enabled: true}, nil); err != nil {
		t.Fatal(err)
	}
	m, err := s.MovieRepo.Add(library.Movie{TMDBID: 1, Title: "Some Movie", Year: 2020, Status: library.StatusMissing})
	if err != nil {
		t.Fatal(err)
	}
	playCache = playLists{}
	t.Cleanup(func() { playCache = playLists{} })

	play := func(query string) (int, playAnswer, string) {
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/play/movies/%d%s", m.ID, query), nil)
		r.SetPathValue("id", fmt.Sprint(m.ID))
		w := httptest.NewRecorder()
		s.handlePlayMovie(w, r)
		var a playAnswer
		_ = json.Unmarshal(w.Body.Bytes(), &a)
		return w.Code, a, w.Body.String()
	}

	code, first, body := play("")
	if code != http.StatusOK {
		t.Fatalf("play: %d %s", code, body)
	}
	if strings.Contains(first.Release, "2160p") || strings.Contains(first.Release, "Other") {
		t.Fatalf("played %q: not cached, or not this movie", first.Release)
	}
	if first.Option != 1 || first.Options < 1 || first.Options > 2 {
		t.Fatalf("option %d of %d", first.Option, first.Options)
	}
	if !strings.HasSuffix(first.URL, "/"+hashOf(first.Release)) || first.FileName != "Some.Movie.mkv" || !strings.HasSuffix(first.StreamURL, ".mp4") {
		t.Fatalf("answer = %+v: want the main file, not the sample", first)
	}
	if first.Options == 2 {
		code, second, body := play("?option=2")
		if code != http.StatusOK || second.Release == first.Release || second.Option != 2 {
			t.Fatalf("option 2: %d %s", code, body)
		}
	}
	if code, _, _ := play("?option=9"); code != http.StatusNotFound {
		t.Fatalf("option past the end: status %d", code)
	}
}

func hashOf(release string) string {
	switch {
	case strings.Contains(release, "1080p"):
		return hash1080
	case strings.Contains(release, "720p"):
		return hash720
	}
	return hash4K
}

func TestPlayNeedsPremiumize(t *testing.T) {
	s := newBareServer(t)
	m, _ := s.MovieRepo.Add(library.Movie{TMDBID: 1, Title: "Some Movie", Year: 2020, Status: library.StatusMissing})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("id", fmt.Sprint(m.ID))
	w := httptest.NewRecorder()
	s.handlePlayMovie(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409 without a Premiumize key", w.Code)
	}
}

func TestPickPlayFileFindsEpisodeInPack(t *testing.T) {
	files := []premiumize.File{
		{Path: "Show.S01/Show.S01E01.1080p.mkv", Size: 2 << 30, Link: "e1"},
		{Path: "Show.S01/Show.S01E02.1080p.mkv", Size: 1 << 30, Link: "e2"},
		{Path: "Show.S01/Show.S01E02.sample.mkv", Size: 1 << 20, Link: "sample"},
		{Path: "Show.S01/Show.S01E02.nfo", Size: 1 << 10, Link: "nfo"},
	}
	f, ok := pickPlayFile(files, playTarget{series: &library.Series{Title: "Show"}, season: 1, episode: 2}, nil)
	if !ok || f.Link != "e2" {
		t.Fatalf("picked %+v, %v; want episode 2", f, ok)
	}
	if _, ok := pickPlayFile(files, playTarget{series: &library.Series{Title: "Show"}, season: 1, episode: 5}, nil); ok {
		t.Fatal("found an episode the pack doesn't have")
	}
}

func TestNormalizeHash(t *testing.T) {
	hexHash := "0123456789ABCDEF0123456789ABCDEF01234567"
	if got := normalizeHash(hexHash); got != strings.ToLower(hexHash) {
		t.Fatalf("hex: %q", got)
	}
	// The same hash in base32, as some magnet links carry it.
	if got := normalizeHash("AERUKZ4JVPG66AJDIVTYTK6N54ASGRLH"); got != strings.ToLower(hexHash) {
		t.Fatalf("base32: %q", got)
	}
	if normalizeHash("nothex") != "" {
		t.Fatal("accepted junk")
	}
}
