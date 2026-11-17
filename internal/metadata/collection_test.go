package metadata_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jhipsman/cueapp/internal/metadata"
)

func TestGetCollectionInOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collection/2344" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"id":2344,"name":"The Matrix Collection","parts":[
			{"id":3,"title":"Matrix 5","release_date":""},
			{"id":2,"title":"The Matrix Reloaded","release_date":"2003-05-15"},
			{"id":1,"title":"The Matrix","release_date":"1999-03-31"}]}`))
	}))
	defer srv.Close()
	col, err := metadata.NewWithBaseURL("k", srv.URL).GetCollection(context.Background(), 2344)
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "The Matrix Collection" || len(col.Parts) != 3 || col.Parts[0].TMDBID != 1 || col.Parts[1].TMDBID != 2 || col.Parts[2].TMDBID != 3 {
		t.Fatalf("collection = %+v", col)
	}
}
