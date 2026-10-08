package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rdborg/mediarium/internal/archiveorg"
	"github.com/rdborg/mediarium/internal/youtube"
)

func TestHasTitle(t *testing.T) {
	cases := []struct {
		text, title string
		want        bool
	}{
		{"Fraggle Rock S01E05 The Thirty-Minute Work Week", "Fraggle Rock", true},
		{"Its A Big Big World - Episode 12", "It's a Big Big World", true},
		{"Johnny & the Sprites 1x03", "Johnny and the Sprites", true},
		{"Fraggle Puppets compilation", "Fraggle Rock", false},
		{"Roxy Hunter and the Mystery of the Moody Ghost", "Roxy Hunter and the Mystery of the Moody Ghost", true},
	}
	for _, c := range cases {
		if got := hasTitle(c.text, c.title); got != c.want {
			t.Errorf("hasTitle(%q, %q) = %v", c.text, c.title, got)
		}
	}
}

func TestIsEpisode(t *testing.T) {
	w := fallbackWant{episode: true, season: 1, number: 5, name: "The Thirty-Minute Work Week"}
	cases := []struct {
		text, file string
		want       bool
	}{
		{"Fraggle Rock S01E05", "", true},
		{"Fraggle Rock s1 e5", "", true},
		{"Fraggle Rock 1x05", "", true},
		{"Fraggle Rock S01E06", "", false},
		{"Fraggle Rock - The Thirty Minute Work Week", "", true},
		{"Fraggle Rock Season 1", "05 - Something.mp4", true},
		{"Fraggle Rock Season 1", "Episode 5.mp4", true},
		{"Fraggle Rock Season 2", "05 - Something.mp4", false},
		{"Fraggle Rock Season 1 Complete", "Fraggle Rock S01E05.mp4", true},
		{"Fraggle Rock Season 1 Complete", "Fraggle Rock S01E04.mp4", false},
		{"Fraggle Rock", "random.mp4", false},
	}
	for _, c := range cases {
		if got := isEpisode(c.text, c.file, w); got != c.want {
			t.Errorf("isEpisode(%q, %q) = %v", c.text, c.file, got)
		}
	}
}

func TestArchiveCandidatesEpisode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/advancedsearch.php":
			if !strings.Contains(r.URL.Query().Get("q"), `title:("Fraggle Rock")`) {
				t.Errorf("query %q", r.URL.Query().Get("q"))
			}
			_, _ = w.Write([]byte(`{"response":{"docs":[
				{"identifier":"fr-s1","title":"Fraggle Rock Season 1"},
				{"identifier":"fr-s01e09","title":"Fraggle Rock S01E09"},
				{"identifier":"other","title":"Something Else"}]}}`))
		case r.URL.Path == "/metadata/fr-s1":
			_, _ = w.Write([]byte(`{"metadata":{"title":"Fraggle Rock Season 1"},"files":[
				{"name":"04 - Wembley.mkv","source":"original","size":"300000000","length":"1500"},
				{"name":"05 - Work Week.mkv","source":"original","size":"300000000","length":"1500"},
				{"name":"05 - Work Week.mp4","source":"derivative","size":"100000000","length":"1500"},
				{"name":"05 - Work Week sample.mp4","source":"derivative","size":"1000","length":"30"}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	got := archiveCandidates(context.Background(), archiveorg.New(srv.URL), fallbackWant{title: "Fraggle Rock", episode: true, season: 1, number: 5})
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	c := got[0]
	if c.URL != srv.URL+"/download/fr-s1/05%20-%20Work%20Week.mp4" || c.Source != sourceArchive || !c.Direct {
		t.Errorf("got %+v", c)
	}
}

func TestYouTubeCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "k" {
			t.Errorf("no key")
		}
		switch r.URL.Path {
		case "/search":
			if q := r.URL.Query().Get("q"); q != "Johnny and the Sprites Rhyme Time" {
				t.Errorf("q = %q", q)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{
				map[string]any{"id": map[string]string{"videoId": "good"}, "snippet": map[string]string{"title": "Johnny and the Sprites - Rhyme Time | Full Episode", "channelTitle": "Disney Junior"}},
				map[string]any{"id": map[string]string{"videoId": "clip"}, "snippet": map[string]string{"title": "Johnny and the Sprites Rhyme Time clip", "channelTitle": "x"}},
				map[string]any{"id": map[string]string{"videoId": "short"}, "snippet": map[string]string{"title": "Johnny and the Sprites Rhyme Time", "channelTitle": "x"}},
				map[string]any{"id": map[string]string{"videoId": "blocked"}, "snippet": map[string]string{"title": "Johnny and the Sprites Rhyme Time", "channelTitle": "x"}},
			}})
		case "/videos":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{
				map[string]any{"id": "good", "contentDetails": map[string]string{"duration": "PT22M10S"}, "status": map[string]bool{"embeddable": true}},
				map[string]any{"id": "clip", "contentDetails": map[string]string{"duration": "PT22M"}, "status": map[string]bool{"embeddable": true}},
				map[string]any{"id": "short", "contentDetails": map[string]string{"duration": "PT2M"}, "status": map[string]bool{"embeddable": true}},
				map[string]any{"id": "blocked", "contentDetails": map[string]string{"duration": "PT22M"}, "status": map[string]bool{"embeddable": false}},
			}})
		}
	}))
	defer srv.Close()
	got := youtubeCandidates(context.Background(), youtube.New("k", srv.URL), fallbackWant{title: "Johnny and the Sprites", episode: true, season: 1, number: 3, name: "Rhyme Time"})
	if len(got) != 1 || got[0].YouTube != "good" || got[0].Source != sourceYouTube {
		t.Fatalf("got %+v", got)
	}
}

func TestCompilationParts(t *testing.T) {
	got := compilationParts("Fraggle Rock: Scared Silly", `Join the Fraggle gang in three frightfully delightful episodes! Fraggle Rock: Scared Silly is bursting with Halloween fun! Include 3 ghostly episodes: “Terrible Tunnel,” “Scared Silly” and “A Dark & Stormy Night.”`)
	var names []string
	for _, p := range got {
		if p.title != "Fraggle Rock" || !p.episode {
			t.Errorf("part %+v", p)
		}
		names = append(names, p.name)
	}
	if strings.Join(names, "|") != "Scared Silly|Terrible Tunnel|A Dark & Stormy Night" {
		t.Errorf("names = %q", names)
	}
	if compilationParts("Labyrinth", "") != nil {
		t.Error("a plain film isn't a set of episodes")
	}
	w := got[0]
	if !isEpisode("Fraggle Rock Season 3", "Fraggle Rock 3x13 - Scared Silly.mp4", w) || isEpisode("Fraggle Rock Season 3", "Fraggle Rock 3x12 - Wembley.mp4", w) {
		t.Error("episode by name")
	}
}
