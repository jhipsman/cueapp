package api

import "testing"

func TestNotEnglish(t *testing.T) {
	cases := []struct {
		release string
		extra   string
		want    bool
	}{
		{"Movie.2019.1080p.BluRay.x264", "", false},                  // untagged: English
		{"Movie.2019.1080p.BluRay.ITA.AC3.x264", "", true},           // Italian only
		{"Movie.2019.1080p.BluRay.ITA.ENG.x264", "", false},          // Italian and English
		{"Movie.2019.MULTI.1080p.BluRay.x264", "", false},            // several, English among them
		{"Movie.2019.1080p.WEB-DL.LATINO.x264", "", true},            // Spanish
		{"Movie.2019.1080p.WEB-DL.x264", "💾 2 GB 🔎 Comet\n🇮🇹", true}, // the add-on's flag: Italian
		{"Movie.2019.1080p.WEB-DL.x264", "💾 2 GB\n🇬🇧 / 🇮🇹", false},   // English among the flags
		{"Movie.2019.1080p.WEB-DL.x264", "🇫🇷 Multi", false},          // the add-on says multi
		{"The.Italian.Job.2003.1080p.BluRay.x264", "", false},        // "Italian" in the title
	}
	for _, c := range cases {
		if got := notEnglish(c.release, c.extra); got != c.want {
			t.Errorf("notEnglish(%q, %q) = %v, want %v", c.release, c.extra, got, c.want)
		}
	}
}

func TestKeepEnglish(t *testing.T) {
	out, other := keepEnglish([]playCandidate{{Release: "a", NotEnglish: true}, {Release: "b"}})
	if other || len(out) != 1 || out[0].Release != "b" {
		t.Fatalf("got %+v %v", out, other)
	}
	if out, other := keepEnglish([]playCandidate{{Release: "a", NotEnglish: true}}); !other || len(out) != 0 {
		t.Fatalf("only other languages: %+v %v", out, other)
	}
}
