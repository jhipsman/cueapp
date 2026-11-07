package api

import (
	"net/http"
	"os"
	"strings"
)

// defaultTVAppURL is where GitHub publishes the latest TV app build (see
// .github/workflows/android.yml). CUE_TV_APK_URL points elsewhere.
const defaultTVAppURL = "https://github.com/jhipsman/cueapp/releases/download/tv-latest/cue-tv.apk"

// GET /tv.apk: forwards to the latest TV app, so a Fire TV's Downloader app
// only needs http://<server>:8264/tv.apk. Public: Downloader can't sign in,
// and the app itself is public.
func (s *Server) handleTVApp(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, tvAppURL(), http.StatusFound)
}

// GET /tv.version: forwards to the build number of that APK, a small text
// file published beside it (cue-tv.version next to cue-tv.apk). The TV app
// reads it to offer its own update. Public, like /tv.apk.
func (s *Server) handleTVAppVersion(w http.ResponseWriter, r *http.Request) {
	target := tvAppURL()
	if !strings.HasSuffix(target, ".apk") {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, strings.TrimSuffix(target, ".apk")+".version", http.StatusFound)
}

func tvAppURL() string {
	target := strings.TrimSpace(os.Getenv("CUE_TV_APK_URL"))
	if !strings.HasPrefix(target, "https://") {
		return defaultTVAppURL
	}
	return target
}
