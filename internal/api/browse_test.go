package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// fakeBrowseTMDB answers discover with titles named after what was asked,
// so a test can see which query fed which row.
func fakeBrowseTMDB(t *testing.T) (*httptest.Server, *[]string) {
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		q := r.URL.Query()
		switch {
		case strings.HasPrefix(r.URL.Path, "/watch/providers/"):
			fmt.Fprint(w, `{"results":[{"provider_id":8,"logo_path":"/n.jpg"}]}`)
		case strings.HasPrefix(r.URL.Path, "/collection/"):
			fmt.Fprint(w, `{"id":10,"name":"Star Wars Collection","poster_path":"/sw.jpg","parts":[
			  {"id":2,"title":"Empire","poster_path":"/b.jpg","release_date":"1980-05-17"},
			  {"id":1,"title":"A New Hope","poster_path":"/a.jpg","release_date":"1977-05-25"}]}`)
		case r.URL.Path == "/discover/movie":
			tag := q.Get("with_genres") + q.Get("with_keywords") + q.Get("sort_by")
			fmt.Fprintf(w, `{"page":1,"results":[{"id":1,"title":"F1 %[1]s","poster_path":"/1.jpg"},{"id":2,"title":"F2 %[1]s","poster_path":"/2.jpg"},{"id":3,"title":"F3","poster_path":"/3.jpg"}]}`, tag)
		case r.URL.Path == "/discover/tv":
			tag := q.Get("with_networks") + q.Get("with_genres")
			fmt.Fprintf(w, `{"page":1,"results":[{"id":11,"name":"S1 %[1]s","poster_path":"/s1.jpg"},{"id":12,"name":"S2 %[1]s","poster_path":"/s2.jpg"}]}`, tag)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	return srv, &asked
}

func TestBrowseService(t *testing.T) {
	tm, asked := fakeBrowseTMDB(t)
	defer tm.Close()
	s := &Server{tmdb: metadata.NewWithBaseURL("k", tm.URL)}
	req := httptest.NewRequest("GET", "/api/watch/browse/service/netflix", nil)
	req.SetPathValue("key", "netflix")
	w := httptest.NewRecorder()
	s.handleBrowseService(w, req)
	var out struct {
		Name string
		Rows []browseRow
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Name != "Netflix" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	byKey := map[string]browseRow{}
	for _, r := range out.Rows {
		byKey[r.Key] = r
	}
	// Films and shows mixed, a film then a show.
	top := byKey["top"]
	if top.Title != "Top 10 on Netflix today" || len(top.Items) != 5 || top.Items[0].Kind != "movie" || top.Items[1].Kind != "tv" {
		t.Fatalf("top: %+v", top)
	}
	// Originals: Netflix's own shows (network 213), shows only. The fake
	// has two, under the four a row needs, so the row is left out.
	if _, ok := byKey["originals"]; ok {
		t.Fatalf("a two-title row should be left out: %+v", byKey["originals"])
	}
	found := false
	for _, a := range *asked {
		if strings.Contains(a, "with_networks=213") && strings.Contains(a, "with_watch_providers=8") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no originals ask in %v", *asked)
	}
	if _, ok := byKey["genre-Comedy"]; !ok {
		t.Fatalf("rows: %v", byKey)
	}
	// Unknown service: 404.
	req = httptest.NewRequest("GET", "/api/watch/browse/service/nope", nil)
	req.SetPathValue("key", "nope")
	w = httptest.NewRecorder()
	s.handleBrowseService(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown service: %d", w.Code)
	}
}

func TestBrowseCollectionsAndFranchise(t *testing.T) {
	tm, asked := fakeBrowseTMDB(t)
	defer tm.Close()
	s := &Server{tmdb: metadata.NewWithBaseURL("k", tm.URL)}

	req := httptest.NewRequest("GET", "/api/watch/browse/collection/family", nil)
	req.SetPathValue("key", "family")
	w := httptest.NewRecorder()
	s.handleBrowseCollection(w, req)
	var col struct {
		Title string
		Items []watchCard
	}
	if json.Unmarshal(w.Body.Bytes(), &col); col.Title != "Family movie night" || len(col.Items) != 3 {
		t.Fatalf("family: %s", w.Body.String())
	}
	ok := false
	for _, a := range *asked {
		if strings.Contains(a, "certification.lte=PG") && strings.Contains(a, "with_genres=10751") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("family asked %v", *asked)
	}

	req = httptest.NewRequest("GET", "/api/watch/browse/franchise/10", nil)
	req.SetPathValue("id", "10")
	w = httptest.NewRecorder()
	s.handleBrowseFranchise(w, req)
	var fr struct {
		Name  string
		Items []watchCard
	}
	if json.Unmarshal(w.Body.Bytes(), &fr); fr.Name != "Star Wars" || len(fr.Items) != 2 || fr.Items[0].Title != "A New Hope" {
		t.Fatalf("franchise: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	s.handleBrowse(w, httptest.NewRequest("GET", "/api/watch/browse", nil))
	var b struct {
		Services    []browseService
		Collections []browseCollection
		Franchises  []browseFranchise
	}
	if json.Unmarshal(w.Body.Bytes(), &b); len(b.Services) != 8 || len(b.Franchises) == 0 || b.Franchises[0].Name != "Star Wars" || len(b.Collections) < 4 {
		t.Fatalf("browse: %s", w.Body.String())
	}
}

func TestInSeason(t *testing.T) {
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 12, 0, 0, 0, time.UTC) }
	if !inSeason("holiday", d(12, 3)) || !inSeason("holiday", d(11, 20)) || inSeason("holiday", d(11, 2)) {
		t.Error("holiday")
	}
	if !inSeason("halloween", d(10, 30)) || inSeason("halloween", d(11, 1)) || !inSeason("", d(3, 1)) {
		t.Error("halloween")
	}
}
