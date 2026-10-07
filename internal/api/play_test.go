package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestPlaySaysWhyNothingPlays(t *testing.T) {
	cases := []struct {
		st   playStats
		want string
	}{
		{playStats{}, "No indexers are set up"},
		{playStats{indexers: 2, failed: []string{"A", "B"}, reasons: []string{"A: timed out", "B: Cloudflare"}}, "None of your indexers answered. A: timed out. B: Cloudflare."},
		{playStats{indexers: 1}, "they're all Usenet indexers"},
		{playStats{indexers: 2, torrentIndexers: 2, failed: []string{"X"}}, "found nothing for Film (X didn't answer)"},
		{playStats{indexers: 1, torrentIndexers: 1, results: 4}, "none are torrents"},
		{playStats{indexers: 1, torrentIndexers: 1, results: 4, torrents: 4}, "none were for Film"},
		{playStats{indexers: 1, torrentIndexers: 1, results: 4, torrents: 4, matching: 3}, "Premiumize doesn't have any of them ready"},
	}
	for _, c := range cases {
		checked := 0
		if c.st.matching > 0 {
			checked = c.st.matching
		}
		if got := c.st.message("Film", checked); !strings.Contains(got, c.want) {
			t.Errorf("%+v: %q, want it to say %q", c.st, got, c.want)
		}
	}
}

func TestPlayFallsBackWhenTheProfileRefusesAll(t *testing.T) {
	m := library.Movie{Title: "Some Movie", Year: 2020}
	results := []indexers.Result{
		{Title: "Some.Movie.2020.HDCAM.x264-GRP", Seeders: 900},
		{Title: "Some.Movie.2020.720p.WEB.x264-GRP", Seeders: 10},
		{Title: "Some.Movie.2020.2160p.WEB-DL.x265-GRP", Seeders: 5},
		{Title: "Some.Movie.1999.1080p.BluRay.x264-GRP", Seeders: 50},
	}
	got := rankAnyWatchable(results, playTarget{movie: &m})
	if len(got) != 2 || !strings.Contains(got[0].Title, "2160p") || !strings.Contains(got[1].Title, "720p") {
		titles := []string{}
		for _, r := range got {
			titles = append(titles, r.Title)
		}
		t.Fatalf("ranked %q; want 2160p then 720p, no cam, no other year", titles)
	}
}

func TestPlayWithNoIndexersExplains(t *testing.T) {
	s := newBareServer(t)
	s.premiumizeBase = fakePremiumizeCache(t).URL
	if err := s.Settings.Set(settings.KeyPremiumizeAPIKey, "k", true); err != nil {
		t.Fatal(err)
	}
	m, _ := s.MovieRepo.Add(library.Movie{TMDBID: 7, Title: "Some Movie", Year: 2020, Status: library.StatusMissing})
	playCache = playLists{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("id", fmt.Sprint(m.ID))
	w := httptest.NewRecorder()
	s.handlePlayMovie(w, r)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "No indexers are set up") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if _, ok := playCache.get("movie:7"); ok {
		t.Fatal("an empty answer was kept: adding an indexer wouldn't help for 15 minutes")
	}
}

func TestPlayDoesNotWaitForASlowSite(t *testing.T) {
	s := newBareServer(t)
	s.premiumizeBase = fakePremiumizeCache(t).URL
	if err := s.Settings.Set(settings.KeyPremiumizeAPIKey, "k", true); err != nil {
		t.Fatal(err)
	}
	fast := fakeTorznab(t)
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // answers only when the test ends
	}))
	t.Cleanup(func() { close(release); slow.Close() })
	for name, url := range map[string]string{"Fast": fast.URL, "Slow": slow.URL} {
		if _, err := s.IndexerRepo.Create(indexers.Instance{Name: name, BaseURL: url, APIKey: "x", Protocol: indexers.ProtocolTorrent, Enabled: true}, nil); err != nil {
			t.Fatal(err)
		}
	}
	m, _ := s.MovieRepo.Add(library.Movie{TMDBID: 9, Title: "Some Movie", Year: 2020, Status: library.StatusMissing})
	oldWait, oldMax := playSearchWait, playSearchMax
	playSearchWait, playSearchMax = 300*time.Millisecond, 2*time.Second
	playCache = playLists{}
	t.Cleanup(func() { playSearchWait, playSearchMax = oldWait, oldMax; playCache = playLists{} })

	call := func(q string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/"+q, nil)
		r.SetPathValue("id", fmt.Sprint(m.ID))
		w := httptest.NewRecorder()
		s.handlePlayMovie(w, r)
		return w
	}
	start := time.Now()
	if w := call("?prefetch=1"); w.Code != http.StatusNoContent {
		t.Fatalf("prefetch: %d %s", w.Code, w.Body)
	}
	if took := time.Since(start); took > 1500*time.Millisecond {
		t.Fatalf("the look-ahead waited %v for the slow site", took)
	}
	start = time.Now()
	if w := call(""); w.Code != http.StatusOK {
		t.Fatalf("play after the look-ahead: %d %s", w.Code, w.Body)
	}
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Fatalf("play after the look-ahead took %v; the list should have been ready", took)
	}
}
