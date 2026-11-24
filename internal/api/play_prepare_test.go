package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/indexers"
	"github.com/jhipsman/cueapp/internal/library"
	"github.com/jhipsman/cueapp/internal/settings"
)

// Day one: nothing is in Premiumize's cloud yet. Get it ready sends the best
// version to be fetched; once the fetch is done, Play plays it.
func TestGetItReadyOnDayOne(t *testing.T) {
	var mu sync.Mutex
	cached := map[string]bool{}
	var sent []string
	done := false
	pm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		defer mu.Unlock()
		reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/cache/check":
			var resp []bool
			for _, h := range r.Form["items[]"] {
				resp = append(resp, cached[h])
			}
			reply(map[string]any{"status": "success", "response": resp})
		case "/transfer/create":
			sent = append(sent, r.Form.Get("src"))
			reply(map[string]any{"status": "success", "id": "tr1"})
		case "/transfer/list":
			status, progress := "running", 0.4
			if done {
				status, progress = "finished", 1
			}
			reply(map[string]any{"status": "success", "transfers": []map[string]any{{"id": "tr1", "status": status, "progress": progress}}})
		case "/transfer/directdl":
			src := r.Form.Get("src")
			h := src[strings.LastIndex(src, ":")+1:]
			if !cached[h] {
				reply(map[string]any{"status": "error", "message": "not cached"})
				return
			}
			reply(map[string]any{"status": "success", "content": []map[string]any{
				{"path": "Some.Movie/Some.Movie.mkv", "size": 9 << 30, "link": "https://cdn.example/" + h},
			}})
		default:
			reply(map[string]any{"status": "error", "message": "unexpected " + r.URL.Path})
		}
	}))
	t.Cleanup(pm.Close)

	s := newBareServer(t)
	s.premiumizeBase = pm.URL
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
	preparesMu.Lock()
	prepares = map[string]*prepareJob{}
	preparesMu.Unlock()
	old := prepareEvery
	prepareEvery = 20 * time.Millisecond
	t.Cleanup(func() { prepareEvery = old })

	play := func() (int, string) {
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/play/movies/%d", m.ID), nil)
		r.SetPathValue("id", fmt.Sprint(m.ID))
		w := httptest.NewRecorder()
		s.handlePlayMovie(w, r)
		return w.Code, w.Body.String()
	}
	if code, body := play(); code != http.StatusNotFound || !strings.Contains(body, "Get it ready") {
		t.Fatalf("before: %d %s", code, body)
	}

	st, err := s.startPrepare(context.Background(), movieTarget(m), 0, "/watch/movie/1")
	if err != nil || st.State != "preparing" {
		t.Fatalf("prepare: %+v %v", st, err)
	}
	mu.Lock()
	if len(sent) != 1 || !strings.Contains(sent[0], "btih:") {
		t.Fatalf("sent %v", sent)
	}
	// Asked again while it's fetching: the same fetch, not another.
	mu.Unlock()
	if st2, _ := s.startPrepare(context.Background(), movieTarget(m), 0, ""); st2.State != "preparing" {
		t.Fatalf("again: %+v", st2)
	}
	mu.Lock()
	if len(sent) != 1 {
		t.Fatalf("sent twice: %v", sent)
	}
	// Premiumize finishes: it's in the cloud.
	h := sent[0][strings.Index(sent[0], "btih:")+5 : strings.Index(sent[0], "&")]
	cached[h], done = true, true
	mu.Unlock()

	deadline := time.Now().Add(3 * time.Second)
	for prepareJobFor(movieTarget(m).key).status().State != "ready" {
		if time.Now().After(deadline) {
			t.Fatalf("never ready: %+v", prepareJobFor(movieTarget(m).key).status())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if code, body := play(); code != http.StatusOK || !strings.Contains(body, h) {
		t.Fatalf("after: %d %s", code, body)
	}
}
