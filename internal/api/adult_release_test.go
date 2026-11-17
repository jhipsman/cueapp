package api

import (
	"testing"

	"github.com/jhipsman/cueapp/internal/indexers"
	"github.com/jhipsman/cueapp/internal/library"
)

func TestAdultReleasesNeverMatch(t *testing.T) {
	kids := library.Movie{Title: "Anastasia", Year: 2020}
	for name, want := range map[string]bool{
		"Anastasia.2020.1080p.WEB-DL.x264-GRP":        true,
		"Anastasia 2020 720p WEBRip":                  true,
		"Anastasia.2020.XXX.1080p.MP4-WRB":            false,
		"Brazzers.Anastasia.2020.1080p":               false,
		"Anastasia.2020.PORN.720p":                    false,
		"Anastasia (2020) [1080p] [OnlyFans]":         false,
		"Anastasia.2020.Hentai.1080p":                 false,
		"Anastasia.2020.Japanese.1080p.BluRay.x264":   true, // "Japanese" isn't "jav"
		"Anastasia.2020.1080p.Uncensored.BluRay.x264": false,
	} {
		if got := matchesMovie(name, kids); got != want {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
	// A site's XXX category counts too, whatever the name.
	got := matchingResults([]indexers.Result{
		{Title: "Anastasia.2020.1080p.WEB-DL", Categories: []int{6040}},
		{Title: "Anastasia.2020.1080p.WEB-DL", Categories: []int{2040}},
	}, kids)
	if len(got) != 1 || got[0].Categories[0] != 2040 {
		t.Fatalf("matching: %+v", got)
	}
	if c := withoutAdult([]playCandidate{{Release: "Anastasia.2020.XXX.1080p"}, {Release: "Anastasia.2020.1080p.WEB"}}); len(c) != 1 {
		t.Fatalf("candidates: %+v", c)
	}
}

func TestWrongVersion(t *testing.T) {
	s := newBareServer(t)
	m := library.Movie{TMDBID: 476389, Title: "Anastasia: Once Upon a Time", Year: 2020}
	tgt := movieTarget(m)
	cands := []playCandidate{{Release: "Anastasia.2020.1080p.WEB"}, {Release: "Anastasia.Once.Upon.a.Time.2020.1080p.WEB"}}
	if got := s.withoutWrong(cands, tgt); len(got) != 2 {
		t.Fatalf("nothing marked yet: %d", len(got))
	}
	if err := s.WatchRepo.MarkWrong("movie", 476389, 0, 0, "Anastasia.2020.1080p.WEB"); err != nil {
		t.Fatal(err)
	}
	got := s.withoutWrong(cands, tgt)
	if len(got) != 1 || got[0].Release != "Anastasia.Once.Upon.a.Time.2020.1080p.WEB" {
		t.Fatalf("after marking: %+v", got)
	}
	// Another film keeps it.
	if got := s.withoutWrong(cands, movieTarget(library.Movie{TMDBID: 1, Title: "Other"})); len(got) != 2 {
		t.Fatalf("another film lost it: %d", len(got))
	}
}
