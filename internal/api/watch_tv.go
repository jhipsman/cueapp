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
	target := strings.TrimSpace(os.Getenv("CUE_TV_APK_URL"))
	if !strings.HasPrefix(target, "https://") {
		target = defaultTVAppURL
	}
	http.Redirect(w, r, target, http.StatusFound)
}
