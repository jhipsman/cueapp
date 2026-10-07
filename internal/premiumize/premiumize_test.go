package premiumize

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI is a small stand-in for the Premiumize API and its file server.
type fakeAPI struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	key    string
	files  map[string]string // download path -> content
	polls  int
	ranges []string
	calls  []string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, key: "good-key", files: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) json(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.HasPrefix(r.URL.Path, "/dl/") {
		body, ok := f.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		f.ranges = append(f.ranges, r.Header.Get("Range"))
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(body))
		return
	}
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	if r.URL.Query().Get("apikey") != f.key {
		f.json(w, map[string]any{"status": "error", "message": "Not logged in."})
		return
	}
	link := func(p string) string { return f.srv.URL + p }
	switch r.URL.Path {
	case "/account/info":
		f.json(w, map[string]any{"status": "success", "customer_id": "123", "premium_until": 2000000000, "limit_used": 0.25})
	case "/transfer/directdl":
		_ = r.ParseForm()
		if !strings.Contains(r.PostForm.Get("src"), "cached") {
			f.json(w, map[string]any{"status": "error", "message": "content not in cache"})
			return
		}
		f.json(w, map[string]any{"status": "success", "content": []map[string]any{
			{"path": "Movie/movie.mkv", "size": len(f.files["/dl/movie.mkv"]), "link": link("/dl/movie.mkv")},
		}})
	case "/transfer/create":
		f.json(w, map[string]any{"status": "success", "id": "tr1", "name": "x", "type": "torrent"})
	case "/transfer/list":
		f.polls++
		t := map[string]any{"id": "tr1", "status": "running", "progress": 0.5}
		if f.polls >= 2 {
			t = map[string]any{"id": "tr1", "status": "finished", "progress": 1, "folder_id": "root"}
		}
		f.json(w, map[string]any{"status": "success", "transfers": []any{t}})
	case "/folder/list":
		switch r.URL.Query().Get("id") {
		case "root":
			f.json(w, map[string]any{"status": "success", "content": []map[string]any{
				{"id": "a", "name": "movie.mkv", "type": "file", "size": len(f.files["/dl/movie.mkv"]), "link": link("/dl/movie.mkv")},
				{"id": "sub", "name": "Subs", "type": "folder"},
			}})
		case "sub":
			f.json(w, map[string]any{"status": "success", "content": []map[string]any{
				{"id": "b", "name": "en.srt", "type": "file", "size": len(f.files["/dl/en.srt"]), "link": link("/dl/en.srt")},
			}})
		}
	case "/transfer/delete", "/folder/delete", "/item/delete":
		f.json(w, map[string]any{"status": "success"})
	default:
		f.json(w, map[string]any{"status": "error", "message": "unknown " + r.URL.Path})
	}
}

func TestAccountInfoAndBadKey(t *testing.T) {
	f := newFakeAPI(t)
	acct, err := New("good-key", f.srv.URL).AccountInfo(context.Background())
	if err != nil {
		t.Fatalf("AccountInfo: %v", err)
	}
	if acct.CustomerID != "123" || acct.PremiumUntil != 2000000000 || acct.LimitUsed != 0.25 {
		t.Fatalf("account = %+v", acct)
	}
	_, err = New("wrong", f.srv.URL).AccountInfo(context.Background())
	if !errors.Is(err, ErrBadKey) {
		t.Fatalf("bad key: err = %v, want ErrBadKey", err)
	}
}

func TestTransferFilesWalksFolders(t *testing.T) {
	f := newFakeAPI(t)
	f.files["/dl/movie.mkv"] = "movie-bytes"
	f.files["/dl/en.srt"] = "subtitle"
	c := New("good-key", f.srv.URL)
	ctx := context.Background()

	if _, err := c.DirectDL(ctx, "magnet:?xt=urn:btih:new"); !errors.Is(err, ErrNotCached) {
		t.Fatalf("DirectDL uncached: err = %v, want ErrNotCached", err)
	}
	id, err := c.CreateTransfer(ctx, "magnet:?xt=urn:btih:new")
	if err != nil || id != "tr1" {
		t.Fatalf("CreateTransfer = %q, %v", id, err)
	}
	tr, err := c.Transfer(ctx, id)
	if err != nil || tr.Done() || tr.Failed() || tr.Progress != 0.5 {
		t.Fatalf("first poll = %+v, %v", tr, err)
	}
	tr, err = c.Transfer(ctx, id)
	if err != nil || !tr.Done() {
		t.Fatalf("second poll = %+v, %v", tr, err)
	}
	files, err := c.Files(ctx, tr)
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(files) != 2 || files[0].Path != "movie.mkv" || files[1].Path != "Subs/en.srt" || files[1].Size != 8 {
		t.Fatalf("files = %+v", files)
	}

	dir := t.TempDir()
	var last, total int64
	err = Fetch(ctx, files, dir, FetchOptions{Progress: func(d, tot int64) { last, total = d, tot }})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Subs", "en.srt")); string(b) != "subtitle" {
		t.Fatalf("en.srt = %q", b)
	}
	if last != total || total != int64(len("movie-bytes")+len("subtitle")) {
		t.Fatalf("progress %d of %d", last, total)
	}
}

func TestDirectDLCached(t *testing.T) {
	f := newFakeAPI(t)
	f.files["/dl/movie.mkv"] = "0123456789"
	files, err := New("good-key", f.srv.URL).DirectDL(context.Background(), "magnet:?xt=urn:btih:cached")
	if err != nil || len(files) != 1 || files[0].Path != "Movie/movie.mkv" || files[0].Size != 10 {
		t.Fatalf("DirectDL = %+v, %v", files, err)
	}
}

func TestFetchResumesAndSkipsWholeFiles(t *testing.T) {
	f := newFakeAPI(t)
	f.files["/dl/movie.mkv"] = "0123456789"
	f.files["/dl/en.srt"] = "subtitle"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "movie.mkv"), []byte("01234"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "en.srt"), []byte("subtitle"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := []File{
		{Path: "movie.mkv", Size: 10, Link: f.srv.URL + "/dl/movie.mkv"},
		{Path: "en.srt", Size: 8, Link: f.srv.URL + "/dl/en.srt"},
	}
	if err := Fetch(context.Background(), files, dir, FetchOptions{}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "movie.mkv")); string(b) != "0123456789" {
		t.Fatalf("movie.mkv = %q", b)
	}
	if len(f.ranges) != 1 || f.ranges[0] != "bytes=5-" {
		t.Fatalf("requests = %q, want one ranged request for the movie only", f.ranges)
	}
}

func TestSafePathRefusesEscapes(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"", "/", "../x", "a/../../x", "..\\x", "a/./b/.."} {
		if p, err := SafePath(dir, bad); err == nil && !strings.HasPrefix(p, dir+string(filepath.Separator)) {
			t.Errorf("SafePath(%q) = %q, escapes the folder", bad, p)
		}
	}
	for _, bad := range []string{"", "../x", "..\\x"} {
		if _, err := SafePath(dir, bad); err == nil {
			t.Errorf("SafePath(%q) accepted", bad)
		}
	}
	p, err := SafePath(dir, "/Show/Season 1/e01.mkv")
	if err != nil || p != filepath.Join(dir, "Show", "Season 1", "e01.mkv") {
		t.Fatalf("SafePath = %q, %v", p, err)
	}
}

func TestErrorMessagesDropTheKey(t *testing.T) {
	c := New("secret-key-123", "http://127.0.0.1:1")
	_, err := c.AccountInfo(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret-key-123") {
		t.Fatalf("err = %v, want an error without the key", err)
	}
}
