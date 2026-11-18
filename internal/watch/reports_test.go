package watch_test

import (
	"path/filepath"
	"testing"

	"github.com/jhipsman/cueapp/internal/store"
	"github.com/jhipsman/cueapp/internal/watch"
)

func TestReports(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (1, 'owner', 'x', 1), (2, 'kid', 'x', 0)`); err != nil {
		t.Fatal(err)
	}
	r := watch.NewRepo(db)
	owner, _ := r.Profiles(1, "Owner")
	kid, _ := r.Profiles(2, "Kid")
	if len(owner) == 0 || len(kid) == 0 {
		t.Fatal("no profiles")
	}

	rp, err := r.AddReport(watch.Report{ProfileID: kid[0].ID, Kind: "movie", TMDBID: 603, Title: "The Matrix", Release: "The.Matrix.1999.1080p", Problem: "sound", Note: "no sound"})
	if err != nil {
		t.Fatal(err)
	}
	if rp.Status != "open" || rp.Who != "Kid" {
		t.Fatalf("new report: %+v", rp)
	}
	if n := r.OpenReports(); n != 1 {
		t.Fatalf("open = %d", n)
	}
	if mine, _ := r.Reports(owner[0].ID); len(mine) != 0 {
		t.Fatalf("owner's own reports: %d", len(mine))
	}
	if all, _ := r.Reports(0); len(all) != 1 {
		t.Fatalf("all reports: %d", len(all))
	}
	if owners, _ := r.OwnerProfiles(); len(owners) != 1 || owners[0] != owner[0].ID {
		t.Fatalf("owners = %v", owners)
	}

	// Nothing to hear until it's answered; then once.
	if news, _ := r.ReportNews(kid[0].ID); len(news) != 0 {
		t.Fatal("news before an answer")
	}
	if _, err := r.ResolveReport(rp.ID, "fixed", "Swapped the version"); err != nil {
		t.Fatal(err)
	}
	news, _ := r.ReportNews(kid[0].ID)
	if len(news) != 1 || news[0].Reply != "Swapped the version" || news[0].Status != "fixed" {
		t.Fatalf("news = %+v", news)
	}
	if again, _ := r.ReportNews(kid[0].ID); len(again) != 0 {
		t.Fatal("news shown twice")
	}
	if r.OpenReports() != 0 {
		t.Fatal("still open")
	}
	if _, err := r.Report(9999); err != watch.ErrReportNotFound {
		t.Fatalf("missing report: %v", err)
	}
}
