package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jhipsman/cueapp/internal/download"
)

func TestMarkersFromBeforeTheRenameAreAdopted(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{".mediarium-downloaded", ".mediarium-progress"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("3"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !keptResume(dir) {
		t.Fatal("a partial download from before the rename should be found")
	}
	for _, name := range []string{downloadedMarker, download.ResumeFile} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != "3" {
			t.Errorf("%s: %q (err %v)", name, b, err)
		}
	}
	for _, name := range []string{".mediarium-downloaded", ".mediarium-progress"} {
		if fileExists(filepath.Join(dir, name)) {
			t.Errorf("%s should have been renamed", name)
		}
	}

	// A marker under the new name is never replaced by an old one.
	os.WriteFile(filepath.Join(dir, ".mediarium-downloaded"), []byte("9"), 0o644)
	adoptLegacyMarkers(dir)
	if b, _ := os.ReadFile(filepath.Join(dir, downloadedMarker)); string(b) != "3" {
		t.Errorf("the new marker was replaced: %q", b)
	}
}
