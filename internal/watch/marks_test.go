package watch_test

import (
	"path/filepath"
	"testing"

	"github.com/jhipsman/cueapp/internal/store"
	"github.com/jhipsman/cueapp/internal/watch"
)

func TestMarks(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (1, 'a', 'x')`); err != nil {
		t.Fatal(err)
	}
	r := watch.NewRepo(db)
	list, err := r.Profiles(1, "A")
	if err != nil {
		t.Fatal(err)
	}
	pid := list[0].ID

	if err := r.SetMark(pid, "movie", 603, watch.MarkUp, true); err != nil {
		t.Fatal(err)
	}
	if got := r.Thumb(pid, "movie", 603); got != "up" {
		t.Fatalf("thumb = %q", got)
	}
	// Down replaces up.
	_ = r.SetMark(pid, "movie", 603, watch.MarkDown, true)
	if got := r.Thumb(pid, "movie", 603); got != "down" {
		t.Fatalf("thumb after down = %q", got)
	}
	_ = r.SetMark(pid, "tv", 1399, watch.MarkHidden, true)
	m, err := r.Marks(pid)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Has("tv", 1399, watch.MarkHidden); !ok {
		t.Fatal("hidden mark missing")
	}
	if _, ok := m.Has("movie", 603, watch.MarkUp); ok {
		t.Fatal("up still there after down")
	}
	// Clearing the thumb clears either.
	_ = r.SetMark(pid, "movie", 603, watch.MarkUp, false)
	if got := r.Thumb(pid, "movie", 603); got != "" {
		t.Fatalf("thumb after clearing = %q", got)
	}
	if err := r.SetMark(pid, "movie", 1, "odd", true); err == nil {
		t.Fatal("unknown mark accepted")
	}
}
