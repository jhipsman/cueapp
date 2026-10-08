package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Fire TV's Downloader app opens /tv.apk without signing in: it must
// forward to the app, not show the web pages.
func TestTVAppAddressForwardsToTheApp(t *testing.T) {
	s := newBareServer(t)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, httptest.NewRequest("GET", "/tv.apk", nil))
	if w.Code != http.StatusFound || !strings.HasSuffix(w.Header().Get("Location"), "/cue-tv.apk") {
		t.Fatalf("GET /tv.apk: %d, Location %q", w.Code, w.Header().Get("Location"))
	}
}
