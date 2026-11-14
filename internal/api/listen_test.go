package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/deezer"
	"github.com/jhipsman/cueapp/internal/indexers"
	"github.com/jhipsman/cueapp/internal/premiumize"
	"github.com/jhipsman/cueapp/internal/settings"
)

func TestPlainAlbumTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Abbey Road (Remastered 2019)":      "Abbey Road",
		"25 (Deluxe Edition)":               "25",
		"Midnights [Explicit]":              "Midnights",
		"Songs in the Key of Life":          "Songs in the Key of Life",
		"(What's the Story) Morning Glory?": "(What's the Story) Morning Glory?",
	} {
		if got := plainAlbumTitle(in); got != want {
			t.Errorf("plainAlbumTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchAlbumFiles(t *testing.T) {
	tracks := []deezer.Track{
		{ID: 1, Title: "One", Number: 1, Disc: 1},
		{ID: 2, Title: "Everyone Else", Number: 2, Disc: 1},
		{ID: 3, Title: "Last Song", Number: 1, Disc: 2},
	}
	files := []premiumize.File{
		{Path: "Band - Record (2020) [MP3 320]/cover.jpg", Link: "https://cdn/x/cover"},
		{Path: "Band - Record (2020) [MP3 320]/CD2/01 - Last Song.mp3", Link: "https://cdn/x/d2-1"},
		{Path: "Band - Record (2020) [MP3 320]/CD1/02 - Everyone Else.mp3", Link: "https://cdn/x/d1-2"},
		{Path: "Band - Record (2020) [MP3 320]/CD1/01 - One.mp3", Link: "https://cdn/x/d1-1"},
	}
	got := matchAlbumFiles(tracks, files)
	want := map[int64]string{1: "https://cdn/x/d1-1", 2: "https://cdn/x/d1-2", 3: "https://cdn/x/d2-1"}
	if len(got) != 3 {
		t.Fatalf("matched %d songs: %+v", len(got), got)
	}
	for _, s := range got {
		if want[s.ID] != s.URL {
			t.Errorf("song %d -> %s, want %s", s.ID, s.URL, want[s.ID])
		}
	}

	// Numbered only, "1-03" style discs, and a second copy in another format.
	files = []premiumize.File{
		{Path: "Rec/1-01 Track.flac", Link: "f1"}, {Path: "Rec/1-02 Track.flac", Link: "f2"}, {Path: "Rec/2-01 Track.flac", Link: "f3"},
		{Path: "Rec/ogg/1-01.ogg", Link: "o1"},
	}
	got = matchAlbumFiles(tracks, files)
	if len(got) != 3 || got[0].URL != "f1" || got[1].URL != "f2" || got[2].URL != "f3" {
		t.Errorf("by disc and number: %+v", got)
	}

	// No sound files at all.
	if got := matchAlbumFiles(tracks, []premiumize.File{{Path: "a.nfo", Link: "x"}}); got != nil {
		t.Errorf("no audio: %+v", got)
	}
}

func TestRankAlbumReleases(t *testing.T) {
	res := []indexers.Result{
		{Title: "Band - Record (2020) [FLAC 24bit]", Seeders: 90},
		{Title: "Band - Record (2020) [MP3 320]", Seeders: 5},
		{Title: "Band - Discography (1990-2020) [MP3 320]", Seeders: 300},
		{Title: "Other Band - Record (2020) [MP3 320]", Seeders: 300},
		{Title: "Band - Record (2020) [FLAC]", Seeders: 40},
	}
	got := rankAlbumReleases(res, "Band", "Record", 2020)
	var titles []string
	for _, r := range got {
		titles = append(titles, r.Title)
	}
	want := "Band - Record (2020) [MP3 320] | Band - Record (2020) [FLAC] | Band - Record (2020) [FLAC 24bit]"
	if strings.Join(titles, " | ") != want {
		t.Errorf("order = %v", titles)
	}
}

// From the album's page to its songs: searched on the torrent sites, the
// release Premiumize has is the one played, each song paired with its file.
func TestListenSoundFindsCachedRelease(t *testing.T) {
	const (
		hashMP3  = "1111111111111111111111111111111111111111"
		hashFLAC = "2222222222222222222222222222222222222222"
	)
	dz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/album/77":
			_, _ = w.Write([]byte(`{"id":77,"title":"Record (Deluxe Edition)","release_date":"2020-01-01","artist":{"id":9,"name":"Band"}}`))
		case "/album/77/tracks":
			_, _ = w.Write([]byte(`{"data":[{"id":701,"title":"One","track_position":1,"disk_number":1},{"id":702,"title":"Two","track_position":2,"disk_number":1}]}`))
		default:
			_, _ = w.Write([]byte(`{"data":[]}`))
		}
	}))
	t.Cleanup(dz.Close)
	saved := listenCatalog() // the real one, made now, so it's put back as it was
	listenCatalogC = deezer.New(dz.URL)
	t.Cleanup(func() { listenCatalogC = saved })

	item := func(title, hash string) string {
		return fmt.Sprintf(`<item><title>%s</title><guid>%s</guid><link>magnet:?xt=urn:btih:%s</link>
<enclosure url="magnet:?xt=urn:btih:%s" length="100000000" type="application/x-bittorrent"/>
<torznab:attr name="infohash" value="%s"/><torznab:attr name="seeders" value="20"/><torznab:attr name="category" value="3000"/></item>`, title, hash, hash, hash, hash)
	}
	feed := `<?xml version="1.0"?><rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed"><channel>` +
		item("Band - Record (2020) [MP3 320]", hashMP3) + item("Band - Record (2020) [FLAC]", hashFLAC) + `</channel></rss>`
	tz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(feed))
	}))
	t.Cleanup(tz.Close)

	pm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/cache/check":
			var resp []bool
			for _, h := range r.Form["items[]"] {
				resp = append(resp, h == hashFLAC) // only the FLAC release is ready
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "response": resp})
		case "/transfer/directdl":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "content": []map[string]any{
				{"path": "Band - Record/02 - Two.flac", "size": 30 << 20, "link": "https://cdn.example/two.flac"},
				{"path": "Band - Record/01 - One.flac", "size": 30 << 20, "link": "https://cdn.example/one.flac"},
			}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "message": "unexpected " + r.URL.Path})
		}
	}))
	t.Cleanup(pm.Close)

	s := newBareServer(t)
	s.premiumizeBase = pm.URL
	if err := s.Settings.Set(settings.KeyPremiumizeAPIKey, "k", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.IndexerRepo.Create(indexers.Instance{Name: "T", BaseURL: tz.URL, APIKey: "x", Protocol: indexers.ProtocolTorrent, Enabled: true}, nil); err != nil {
		t.Fatal(err)
	}
	listenState.Lock()
	delete(listenState.albums, 77)
	listenState.Unlock()

	ask := func() albumSound {
		r := httptest.NewRequest(http.MethodGet, "/api/listen/album/77/sound", nil)
		r.SetPathValue("id", "77")
		w := httptest.NewRecorder()
		s.handleListenSound(w, r)
		var a albumSound
		_ = json.Unmarshal(w.Body.Bytes(), &a)
		return a
	}
	var got albumSound
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if got = ask(); got.Status != "looking" {
			break
		}
	}
	if got.Status != "ready" || !strings.Contains(got.Release, "FLAC") {
		t.Fatalf("sound = %+v, want the cached FLAC release ready", got)
	}
	if len(got.Tracks) != 2 || got.Tracks[0].ID != 701 || got.Tracks[0].URL != "https://cdn.example/one.flac" || got.Tracks[1].URL != "https://cdn.example/two.flac" {
		t.Errorf("songs = %+v", got.Tracks)
	}
}

// Public sites' names often have no dash: the artist's and album's words
// in the name still count, after the properly named ones.
func TestRankAlbumReleasesWithoutDash(t *testing.T) {
	res := []indexers.Result{
		{Title: "Taylor Swift The Tortured Poets Department 2024 Mp3 320kbps PMEDIA", Seeders: 300},
		{Title: "Taylor Swift - The Tortured Poets Department (2024) [FLAC]", Seeders: 10},
		{Title: "Taylor Swift 1989 Taylors Version 2023 FLAC", Seeders: 500},
		{Title: "Taylor Swift Discography 2006-2024 MP3", Seeders: 900},
	}
	got := rankAlbumReleases(res, "Taylor Swift", "The Tortured Poets Department", 2024)
	if len(got) != 2 || !strings.Contains(got[0].Title, " - ") || !strings.Contains(got[1].Title, "PMEDIA") {
		t.Errorf("got %+v: want the dashed one, then the undashed one; not 1989 or the discography", got)
	}
}

func TestNothingFoundSaysWhichStep(t *testing.T) {
	for _, c := range []struct {
		sites           int
		failed          []string
		found, torrents int
		want            string
	}{
		{2, []string{"A", "B"}, 0, 0, "None of your torrent sites answered"},
		{2, nil, 0, 0, "found nothing at all"},
		{2, nil, 4, 0, "none are torrents"},
		{2, nil, 9, 9, "none of them is Record by Band"},
	} {
		if got := nothingFound("Band", "Record", c.sites, c.failed, c.found, c.torrents); !strings.Contains(got, c.want) {
			t.Errorf("%+v: %q, want it to say %q", c, got, c.want)
		}
	}
}
