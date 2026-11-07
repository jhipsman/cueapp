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

// The TV app asks /tv.version, signed in or not, for the newest build's
// number: it forwards to the file published beside the APK.
func TestTVAppVersionForwardsBesideTheApp(t *testing.T) {
	s := newBareServer(t)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, httptest.NewRequest("GET", "/tv.version", nil))
	if w.Code != http.StatusFound || !strings.HasSuffix(w.Header().Get("Location"), "/cue-tv.version") {
		t.Fatalf("GET /tv.version: %d, Location %q", w.Code, w.Header().Get("Location"))
	}
	t.Setenv("CUE_TV_APK_URL", "https://example.com/builds/latest")
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, httptest.NewRequest("GET", "/tv.version", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /tv.version with an APK address not ending .apk: %d, want 404", w.Code)
	}
}
