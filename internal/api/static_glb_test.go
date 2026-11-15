package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// A 3D model is sent gzipped to a browser that accepts it, and decodes to
// the original file.
func TestGzippedModel(t *testing.T) {
	orig := bytes.Repeat([]byte("glTF model bytes "), 5000)
	fsys := fstest.MapFS{"landing/x.glb": {Data: orig}}
	gz := gzipped(fsys, "landing/x.glb")
	if gz == nil || len(gz) >= len(orig) {
		t.Fatalf("gzipped: %d bytes from %d", len(gz), len(orig))
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	back, _ := io.ReadAll(zr)
	if !bytes.Equal(back, orig) {
		t.Fatal("decoded model differs")
	}
	if gzipped(fsys, "landing/missing.glb") != nil {
		t.Error("a missing file should give nil")
	}
	// Through the real handler: whatever the embedded build holds, a .glb
	// asked for with gzip either comes back gzipped or falls through.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/landing/iphone.glb", nil)
	r.Header.Set("Accept-Encoding", "gzip, br")
	frontendHandler().ServeHTTP(w, r)
	if w.Code == http.StatusOK && w.Header().Get("Content-Encoding") == "gzip" && w.Header().Get("Content-Type") != "model/gltf-binary" {
		t.Errorf("content type %q", w.Header().Get("Content-Type"))
	}
}
