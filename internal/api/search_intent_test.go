package api

import "testing"

func TestReadSearch(t *testing.T) {
	cases := []struct {
		q, title string
		leftover string
	}{
		{"comedy", "Comedy movies and shows", ""},
		{"funny 90s movies", "Comedy movies from the 1990s", ""},
		{"best sci-fi shows on netflix", "Best sci-fi shows on Netflix", ""},
		{"horror", "Horror movies", ""},
		{"new comedies", "New comedy movies and shows", ""},
		{"zombie movies", "Movies about zombie", "zombie"},
		{"romcoms from 2005", "Romantic comedy movies and shows from 2005", ""},
		{"rom com 80's", "Romantic comedy movies and shows from the 1980s", ""},
		{"true crime docs", "True crime movies and shows", ""},
		{"shows on hulu", "Shows on Hulu", ""},
		{"2000s action", "Action movies and shows from the 2000s", ""},
	}
	for _, c := range cases {
		in := readSearch(c.q)
		if got := in.Title(); got != c.title && !(c.leftover != "" && in.Leftover == c.leftover) {
			t.Errorf("%q: %q (leftover %q), want %q", c.q, got, in.Leftover, c.title)
		}
	}
	// A plain title says nothing beyond its name.
	for _, q := range []string{"breaking bad", "the bear", "oppenheimer"} {
		if in := readSearch(q); in.Understood() {
			t.Errorf("%q read as %+v", q, in)
		}
	}
}
