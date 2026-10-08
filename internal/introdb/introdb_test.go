package introdb

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/media" || q.Get("tmdb_id") != "1399" || q.Get("season") != "1" || q.Get("episode") != "2" || q.Get("duration_ms") != "3300000" {
			t.Errorf("request %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"tmdb_id":1399,"type":"tv","season":1,"episode":2,
			"intro":[{"start_ms":null,"end_ms":95000,"confidence":0.9}],
			"recap":[{"start_ms":0,"end_ms":40000}],
			"credits":[{"start_ms":3200000,"end_ms":null}]}`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	for range 2 { // the second comes from the cache
		got, err := c.Get(t.Context(), 1399, 1, 2, 3300)
		if err != nil {
			t.Fatal(err)
		}
		if got.Intro == nil || got.Intro.Start != -1 || got.Intro.End != 95 || got.Recap == nil || got.Recap.End != 40 || got.Credits == nil || got.Credits.Start != 3200 || got.Credits.End != -1 {
			t.Fatalf("got %+v %+v %+v", got.Intro, got.Recap, got.Credits)
		}
	}
	if calls != 1 {
		t.Errorf("asked %d times", calls)
	}
	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()
	if got, err := New(missing.URL).Get(t.Context(), 5, 1, 1, 0); err != nil || got.Intro != nil {
		t.Errorf("nothing submitted: %+v %v", got, err)
	}
}
