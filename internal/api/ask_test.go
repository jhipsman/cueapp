package api

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jhipsman/cueapp/internal/settings"
	"github.com/jhipsman/cueapp/internal/store"
	"github.com/jhipsman/cueapp/internal/watch"
)

func TestAskKeyAndPrompt(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &Server{Settings: settings.New(db, nil)}
	t.Setenv("ANTHROPIC_API_KEY", "")
	if s.anthropicKey() != "" {
		t.Fatal("a key with none set")
	}
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	if s.anthropicKey() != "from-env" {
		t.Fatal("environment key not used")
	}
	_ = s.Settings.Set(settings.KeyAnthropicAPIKey, " saved ", false)
	if s.anthropicKey() != "saved" {
		t.Fatal("the saved key should win")
	}
	if s.askModel() != askModelHaiku {
		t.Fatal("Ask Cue should default to Haiku")
	}
	_ = s.Settings.Set(settings.KeyAnthropicModel, "opus", false)
	if s.askModel() != askModelOpus {
		t.Fatal("opus not used when chosen")
	}
	kid := askSystem(watch.Profile{Name: "Kid", MaxRating: "PG"})
	if !strings.Contains(kid, "child's profile limited to PG") || !strings.Contains(kid, `"Kid"`) {
		t.Fatal("kids' prompt missing its limit")
	}
	if strings.Contains(askSystem(watch.Profile{Name: "Mom"}), "child's profile") {
		t.Fatal("grown-up prompt says child")
	}
	a := &asker{seen: map[string]watchCard{}}
	if got := a.note(nil); got != "No titles found." {
		t.Fatalf("empty note: %q", got)
	}
	if got := a.note([]watchCard{{Kind: "movie", TMDBID: 603, Title: "The Matrix"}}); !strings.Contains(got, `"id":603`) {
		t.Fatalf("note: %q", got)
	}
	if _, ok := a.seen[cardKey("movie", 603)]; !ok {
		t.Fatal("noted title not remembered")
	}
}
