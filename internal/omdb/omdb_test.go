package omdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRatingsAndCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case r.URL.Query().Get("apikey") != "good":
			_, _ = w.Write([]byte(`{"Response":"False","Error":"Invalid API key!"}`))
		case r.URL.Query().Get("i") == "tt0111161":
			_, _ = w.Write([]byte(`{"Response":"True","imdbRating":"9.3","imdbVotes":"2,900,000","Metascore":"N/A",
				"Ratings":[{"Source":"Internet Movie Database","Value":"9.3/10"},{"Source":"Rotten Tomatoes","Value":"89%"}]}`))
		default:
			_, _ = w.Write([]byte(`{"Response":"False","Error":"Incorrect IMDb ID."}`))
		}
	}))
	defer srv.Close()

	c := New("good", srv.URL)
	r, err := c.Ratings(context.Background(), "tt0111161")
	if err != nil || r.IMDb != "9.3" || r.RottenTomatoes != "89%" || r.Metacritic != "" || r.IMDbVotes != "2,900,000" {
		t.Fatalf("Ratings = %+v, %v", r, err)
	}
	if _, err := c.Ratings(context.Background(), "tt0111161"); err != nil || calls != 1 {
		t.Fatalf("second lookup: err %v, %d calls; want it from the cache", err, calls)
	}
	if _, err := New("bad", srv.URL).Ratings(context.Background(), "tt0111161"); !errors.Is(err, ErrBadKey) {
		t.Fatalf("bad key: %v", err)
	}
	if _, err := c.Ratings(context.Background(), "nope"); err == nil {
		t.Fatal("accepted a non-IMDb id")
	}
}
