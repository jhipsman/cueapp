package watch_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rdborg/mediarium/internal/store"
	"github.com/rdborg/mediarium/internal/watch"
)

func TestProgressAndList(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (1, 'a', 'x'), (2, 'b', 'x')`); err != nil {
		t.Fatal(err)
	}
	r := watch.NewRepo(db)
	// Progress belongs to profiles: profile 1 is account 1's main, 2 account 2's.
	if _, err := r.Profiles(1, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Profiles(2, "b"); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	save := func(user int64, p watch.Progress) {
		t.Helper()
		if err := r.SaveProgress(user, p); err != nil {
			t.Fatal(err)
		}
	}
	save(1, watch.Progress{Kind: watch.KindTV, TMDBID: 10, Season: 1, Episode: 1, Position: 1300, Duration: 1320, UpdatedAt: base})
	save(1, watch.Progress{Kind: watch.KindTV, TMDBID: 10, Season: 1, Episode: 2, Position: 60, Duration: 1320, UpdatedAt: base.Add(time.Hour)})
	save(1, watch.Progress{Kind: watch.KindMovie, TMDBID: 20, Position: 600, Duration: 6000, Title: "Movie", UpdatedAt: base.Add(30 * time.Minute)})
	save(2, watch.Progress{Kind: watch.KindMovie, TMDBID: 30, Position: 5, Duration: 100, UpdatedAt: base})
	// Saving again updates the same row.
	save(1, watch.Progress{Kind: watch.KindMovie, TMDBID: 20, Position: 900, Duration: 6000, Title: "Movie", UpdatedAt: base.Add(40 * time.Minute)})

	recent, err := r.Recent(1, 10)
	if err != nil || len(recent) != 2 {
		t.Fatalf("Recent = %+v, %v", recent, err)
	}
	if recent[0].TMDBID != 10 || recent[0].Episode != 2 || recent[1].TMDBID != 20 || recent[1].Position != 900 {
		t.Fatalf("Recent order/content = %+v", recent)
	}
	p, ok, err := r.GetProgress(1, watch.KindTV, 10, 1, 1)
	if err != nil || !ok || !p.Finished() {
		t.Fatalf("episode 1 = %+v %v %v; want finished", p, ok, err)
	}
	if eps, _ := r.ShowProgress(1, 10); len(eps) != 2 {
		t.Fatalf("ShowProgress = %d episodes", len(eps))
	}
	if _, ok, _ := r.GetProgress(2, watch.KindMovie, 20, 0, 0); ok {
		t.Fatal("one account sees another's progress")
	}

	if err := r.AddToList(1, watch.ListItem{Kind: watch.KindMovie, TMDBID: 20, Title: "Movie"}); err != nil {
		t.Fatal(err)
	}
	if !r.InList(1, watch.KindMovie, 20) || r.InList(2, watch.KindMovie, 20) {
		t.Fatal("InList wrong")
	}
	if list, _ := r.List(1); len(list) != 1 || list[0].Title != "Movie" {
		t.Fatalf("List = %+v", list)
	}
	if err := r.RemoveFromList(1, watch.KindMovie, 20); err != nil || r.InList(1, watch.KindMovie, 20) {
		t.Fatal("RemoveFromList didn't remove")
	}
	if err := r.ForgetTitle(1, watch.KindTV, 10); err != nil {
		t.Fatal(err)
	}
	if recent, _ := r.Recent(1, 10); len(recent) != 1 {
		t.Fatalf("after ForgetTitle: %+v", recent)
	}
}

func TestProfiles(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (1, 'dad', 'x'), (2, 'other', 'x')`); err != nil {
		t.Fatal(err)
	}
	r := watch.NewRepo(db)
	list, err := r.Profiles(1, "Dad")
	if err != nil || len(list) != 1 || !list[0].Main || list[0].Name != "Dad" {
		t.Fatalf("first profiles = %+v, %v", list, err)
	}
	main := list[0]
	kid, err := r.AddProfile(1, "  Sam  ", "purple")
	if err != nil || kid.Name != "Sam" || kid.Avatar != "purple" || kid.Main || kid.HasPIN {
		t.Fatalf("AddProfile = %+v, %v", kid, err)
	}
	if _, err := r.AddProfile(1, "", "red"); !errors.Is(err, watch.ErrBadName) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := r.Profile(2, kid.ID); !errors.Is(err, watch.ErrProfileNotFound) {
		t.Fatalf("another account's profile: %v", err)
	}

	pin := "1234"
	before := main.Secret()
	locked, err := r.UpdateProfile(1, main.ID, watch.ProfileChange{PIN: &pin})
	if err != nil || !locked.HasPIN || !locked.CheckPIN("1234") || locked.CheckPIN("0000") || locked.Secret() == before {
		t.Fatalf("PIN: %+v, %v", locked, err)
	}
	bad := "12a4"
	if _, err := r.UpdateProfile(1, main.ID, watch.ProfileChange{PIN: &bad}); !errors.Is(err, watch.ErrBadPIN) {
		t.Fatalf("bad PIN: %v", err)
	}
	none := ""
	if p, _ := r.UpdateProfile(1, main.ID, watch.ProfileChange{PIN: &none}); p.HasPIN || !p.CheckPIN("") {
		t.Fatalf("PIN not removed: %+v", p)
	}

	if err := r.RemoveProfile(1, main.ID); !errors.Is(err, watch.ErrMainProfile) {
		t.Fatalf("removing the main profile: %v", err)
	}
	if err := r.SaveProgress(kid.ID, watch.Progress{Kind: watch.KindMovie, TMDBID: 5, Position: 1, Duration: 10}); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveProfile(1, kid.ID); err != nil {
		t.Fatal(err)
	}
	if recent, _ := r.Recent(kid.ID, 10); len(recent) != 0 {
		t.Fatal("a removed profile's progress stayed")
	}
	for i := 0; i < watch.MaxProfiles-1; i++ {
		if _, err := r.AddProfile(1, "P", "red"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.AddProfile(1, "One too many", "red"); !errors.Is(err, watch.ErrTooManyProfiles) {
		t.Fatalf("over the limit: %v", err)
	}
}
