package deezer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func fakeDeezer(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	answers := map[string]string{
		"/chart/0": `{"tracks":{"data":[{"id":1,"title":"Song","duration":200,"artist":{"id":9,"name":"Band"},"album":{"id":5,"title":"Record","cover_medium":"https://e-cdns-images.dzcdn.net/m.jpg"}}]},
			"albums":{"data":[{"id":5,"title":"Record","cover_xl":"https://e-cdns-images.dzcdn.net/xl.jpg","artist":{"id":9,"name":"Band"}}]},
			"artists":{"data":[{"id":9,"name":"Band","picture_xl":"https://e-cdns-images.dzcdn.net/a.jpg"}]}}`,
		"/editorial/0/releases": `{"data":[{"id":6,"title":"New One","release_date":"2026-11-01","artist":{"id":9,"name":"Band"}}]}`,
		"/album/5":              `{"id":5,"title":"Record","cover_xl":"xl","cover_medium":"m","release_date":"1999-05-01","record_type":"album","artist":{"id":9,"name":"Band"},"genres":{"data":[{"name":"Rock"}]}}`,
		"/album/5/tracks": `{"data":[{"id":11,"title":"One","duration":180,"track_position":1,"disk_number":1,"artist":{"id":9,"name":"Band"}},
			{"id":12,"title":"Two","duration":200,"track_position":2,"disk_number":1,"artist":{"id":9,"name":"Band"}}]}`,
		"/album/404": `{"error":{"type":"DataException","message":"no data","code":800}}`,
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, ok := answers[r.URL.Path]
		if !ok {
			body = `{"data":[]}`
		}
		_, _ = w.Write([]byte(body))
	}))
}

func TestHomeAlbumAndKeeping(t *testing.T) {
	var calls atomic.Int32
	srv := fakeDeezer(t, &calls)
	defer srv.Close()
	c := New(srv.URL)
	ctx := context.Background()

	h, err := c.Home(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.TopTracks) != 1 || h.TopTracks[0].AlbumID != 5 || h.TopTracks[0].Artist.Name != "Band" {
		t.Errorf("top tracks = %+v", h.TopTracks)
	}
	if len(h.TopAlbums) != 1 || h.TopAlbums[0].Cover == "" || len(h.NewReleases) != 1 || len(h.TopArtists) != 1 {
		t.Errorf("home = %+v", h)
	}

	a, err := c.Album(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if a.Year() != 1999 || a.Genre != "Rock" || len(a.Tracks) != 2 || a.Tracks[1].Number != 2 || a.Tracks[1].AlbumTitle != "Record" {
		t.Errorf("album = %+v", a)
	}

	before := calls.Load()
	if _, err := c.Album(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != before {
		t.Error("asked Deezer again for an album it answered moments ago")
	}

	if _, err := c.Album(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing album: err = %v, want ErrNotFound", err)
	}
}

func TestSearchEmpty(t *testing.T) {
	c := New("http://127.0.0.1:1")
	r, err := c.Search(context.Background(), "  ")
	if err != nil || r.Tracks == nil || len(r.Albums) != 0 {
		t.Errorf("empty search = %+v, %v", r, err)
	}
}
