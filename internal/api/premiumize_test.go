package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rdborg/mediarium/internal/indexers"
	"github.com/rdborg/mediarium/internal/settings"
)

// fakePremiumize answers the few Premiumize calls a download makes: the
// release is not cached, the transfer finishes on the second look, and its
// folder holds one file.
func fakePremiumize(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []string
		polls int
	)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/dl/movie.mkv" {
			_, _ = w.Write([]byte("movie"))
			return
		}
		calls = append(calls, r.URL.Path)
		reply := func(v map[string]any) {
			v["status"] = "success"
			_ = json.NewEncoder(w).Encode(v)
		}
		if r.URL.Query().Get("apikey") != "k" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "message": "Not logged in."})
			return
		}
		switch r.URL.Path {
		case "/account/info":
			reply(map[string]any{"customer_id": "42", "premium_until": 2000000000})
		case "/transfer/directdl":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "message": "not cached"})
		case "/transfer/create":
			reply(map[string]any{"id": "t1"})
		case "/transfer/list":
			polls++
			tr := map[string]any{"id": "t1", "status": "running", "progress": 0.3}
			if polls > 1 {
				tr = map[string]any{"id": "t1", "status": "finished", "progress": 1, "folder_id": "f1"}
			}
			reply(map[string]any{"transfers": []any{tr}})
		case "/folder/list":
			reply(map[string]any{"content": []any{map[string]any{"id": "i1", "name": "movie.mkv", "type": "file", "size": 5, "link": srv.URL + "/dl/movie.mkv"}}})
		default:
			reply(map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestPremiumizeDownload(t *testing.T) {
	s := newBareServer(t)
	srv, calls := fakePremiumize(t)
	s.premiumizeBase = srv.URL
	old := premiumizePollEvery
	premiumizePollEvery = 10 * time.Millisecond
	t.Cleanup(func() { premiumizePollEvery = old })

	if s.usePremiumize(indexers.ProtocolTorrent) {
		t.Fatal("Premiumize used before a key was saved")
	}
	if err := s.Settings.Set(settings.KeyPremiumizeAPIKey, "k", true); err != nil {
		t.Fatal(err)
	}
	if !s.usePremiumize(indexers.ProtocolTorrent) || s.usePremiumize(indexers.ProtocolUsenet) {
		t.Fatal("with a key, Premiumize should be used for torrents only by default")
	}

	dir := filepath.Join(s.downloadsIncompleteDir(), "queue-1")
	err := s.downloadPremiumize(context.Background(), 1, "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567", indexers.ProtocolTorrent, dir)
	if err != nil {
		t.Fatalf("downloadPremiumize: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "movie.mkv")); string(b) != "movie" {
		t.Fatalf("movie.mkv = %q", b)
	}
	got := strings.Join(*calls, " ")
	for _, want := range []string{"/transfer/create", "/folder/list", "/transfer/delete", "/folder/delete"} {
		if !strings.Contains(got, want) {
			t.Errorf("calls %q: missing %s", got, want)
		}
	}
}

func TestPremiumizeSettings(t *testing.T) {
	s := newBareServer(t)
	srv, _ := fakePremiumize(t)
	s.premiumizeBase = srv.URL

	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handlePutPremiumize(w, httptest.NewRequest(http.MethodPut, "/api/settings/premiumize", strings.NewReader(body)))
		return w
	}
	if w := put(`{"apiKey":"wrong"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("wrong key: status %d", w.Code)
	}
	if v, _ := s.Settings.Get(settings.KeyPremiumizeAPIKey); v != "" {
		t.Fatalf("a refused key was saved: %q", v)
	}
	w := put(`{"apiKey":"k","useFor":"both"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("good key: status %d %s", w.Code, w.Body)
	}
	var st premiumizeState
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if !st.Set || st.UseFor != "both" || st.CustomerID != "42" {
		t.Fatalf("state = %+v", st)
	}
	if !s.usePremiumize(indexers.ProtocolUsenet) {
		t.Fatal("useFor=both should cover Usenet")
	}
	if w := put(`{"useFor":"nonsense"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad useFor: status %d", w.Code)
	}
	if w := put(`{"apiKey":""}`); w.Code != http.StatusOK || s.premiumizeClient() != nil {
		t.Fatalf("remove key: status %d, still set %v", w.Code, s.premiumizeClient() != nil)
	}
}
