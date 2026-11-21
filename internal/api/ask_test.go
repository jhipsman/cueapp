package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if s.askModel() != askModelSonnet || s.askModelName() != "sonnet" {
		t.Fatal("Ask Cue should default to Sonnet")
	}
	_ = s.Settings.Set(settings.KeyAnthropicModel, "opus", false)
	if s.askModel() != askModelOpus || s.askModelName() != "opus" {
		t.Fatal("opus not used when chosen")
	}
	_ = s.Settings.Set(settings.KeyAnthropicModel, "haiku", false)
	if s.askModel() != askModelHaiku || s.askModelName() != "haiku" {
		t.Fatal("haiku not used when chosen")
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

func TestAskFindOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch p := r.URL.Path; {
		case p == "/search/multi" && r.URL.Query().Get("query") == "Knives Out":
			reply(map[string]any{"results": []any{
				map[string]any{"media_type": "tv", "id": 9, "name": "Knives Out Live", "first_air_date": "2021-01-01", "poster_path": "/k.jpg"},
				map[string]any{"media_type": "movie", "id": 546554, "title": "Knives Out", "release_date": "2019-11-27", "poster_path": "/p.jpg"},
			}})
		case p == "/search/multi" && r.URL.Query().Get("query") == "Dune":
			reply(map[string]any{"results": []any{
				map[string]any{"media_type": "movie", "id": 841, "title": "Dune", "release_date": "1984-12-14", "poster_path": "/d1.jpg"},
				map[string]any{"media_type": "movie", "id": 438631, "title": "Dune", "release_date": "2021-09-15", "poster_path": "/d2.jpg"},
			}})
		case p == "/search/multi":
			reply(map[string]any{"results": []any{
				map[string]any{"media_type": "movie", "id": 1, "title": "Something Else Entirely", "release_date": "2001-01-01", "poster_path": "/x.jpg"},
			}})
		case p == "/movie/546554":
			reply(map[string]any{"id": 546554, "title": "Knives Out", "runtime": 131, "genres": []any{map[string]any{"id": 9648, "name": "Mystery"}}})
		case strings.HasPrefix(p, "/movie/"):
			reply(map[string]any{"id": 1, "runtime": 100})
		default:
			reply(map[string]any{"results": []any{}})
		}
	}))
	t.Cleanup(srv.Close)
	s := newBareServer(t)
	s.TestSetTMDBBaseURL("key", srv.URL)
	a := &asker{s: s, seen: map[string]watchCard{}}
	ctx := context.Background()

	f := a.findOne(ctx, s.TMDB(), askFindTitle{Title: "Knives Out", Year: 2019})
	if f == nil || f.card.TMDBID != 546554 || f.minutes != 131 || len(f.genres) != 1 || f.genres[0] != "Mystery" {
		t.Fatalf("Knives Out: %+v", f)
	}
	if f := a.findOne(ctx, s.TMDB(), askFindTitle{Title: "Dune", Year: 2021, Kind: "movie"}); f == nil || f.card.TMDBID != 438631 {
		t.Fatalf("Dune (2021) should pick the 2021 film: %+v", f)
	}
	if f := a.findOne(ctx, s.TMDB(), askFindTitle{Title: "A Film Nobody Made", Year: 1999}); f != nil {
		t.Fatalf("a different title came back as a match: %+v", f.card)
	}
	if !sameTitle("The Matrix", "matrix") || !titleContains("Dune: Part One", "Dune") || sameTitle("Dune", "Dune: Part Two") {
		t.Fatal("title matching")
	}
}
