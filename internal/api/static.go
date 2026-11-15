package api

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/web"
)

// frontendHandler serves the embedded React build, falling back to
// index.html for any path that isn't a real static asset — required for
// client-side routing (search/library/settings/onboarding are all
// client-side routes).
func frontendHandler() http.Handler {
	sub, err := fs.Sub(web.DistFS, web.DistDir)
	if err != nil {
		// Should be unreachable: web.DistFS always embeds at least the
		// placeholder dist/index.html checked into the repo.
		panic("web: embedded dist not found: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(sub, trimLeadingSlash(r.URL.Path)); err != nil {
			if r.URL.Path == "/bookshelf" || strings.HasPrefix(r.URL.Path, "/bookshelf/") {
				// Cue Books installs as an app of its own: the same
				// page, with its own name and web app manifest.
				if page, err := fs.ReadFile(sub, "index.html"); err == nil {
					w.Header().Set("Cache-Control", "no-cache")
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write(bookshelfPage(page))
					return
				}
			}
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		// Built assets have content-hashed names, so they can be kept
		// forever; the page that points at them must be re-checked every
		// time so an update is picked up straight away instead of the
		// browser running last version's JavaScript.
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if strings.HasSuffix(r.URL.Path, ".glb") && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			// The front page's 3D phone: sent compressed (about half the
			// size). Proxies such as Caddy don't compress this kind of file.
			if gz := gzipped(sub, trimLeadingSlash(r.URL.Path)); gz != nil {
				w.Header().Set("Content-Type", "model/gltf-binary")
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Vary", "Accept-Encoding")
				http.ServeContent(w, r, "", startTime, bytes.NewReader(gz))
				return
			}
		}
		if strings.HasSuffix(r.URL.Path, ".webmanifest") {
			// What browsers expect for "Add to home screen".
			w.Header().Set("Content-Type", "application/manifest+json")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func trimLeadingSlash(p string) string {
	if p == "" || p == "/" {
		return "."
	}
	if p[0] == '/' {
		return p[1:]
	}
	return p
}

// bookshelfPage is index.html for Cue Books: its own web app manifest
// and title, so a browser offers to install it as a separate app.
func bookshelfPage(page []byte) []byte {
	out := bytes.Replace(page, []byte(`href="/manifest.webmanifest"`), []byte(`href="/bookshelf.webmanifest"`), 1)
	return titleTag.ReplaceAll(out, []byte("<title>Cue Books</title>"))
}

var titleTag = regexp.MustCompile(`<title>[^<]*</title>`)

// startTime is when this Cue started: the embedded files' modification time.
var startTime = time.Now()

// gzipCache holds compressed copies of embedded files, made on first ask.
var gzipCache sync.Map

// gzipped is an embedded file compressed with gzip, or nil.
func gzipped(fsys fs.FS, name string) []byte {
	if v, ok := gzipCache.Load(name); ok {
		return v.([]byte)
	}
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(raw); err != nil || zw.Close() != nil {
		return nil
	}
	gzipCache.Store(name, buf.Bytes())
	return buf.Bytes()
}
