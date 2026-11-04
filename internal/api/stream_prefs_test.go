package api

import (
	"testing"

	"github.com/jhipsman/cueapp/internal/watch"
)

const gb = int64(1) << 30

// TestRankByPrefs: with Cue's defaults, a 4K web version at the size
// streaming services use comes before the add-on's first pick (a 4K
// Remux), and 1080p only when no 4K is right; the other settings move
// versions as they say.
func TestRankByPrefs(t *testing.T) {
	film := 120 * 60.0
	cands := []playCandidate{
		{Release: "Night.Harbor.2010.2160p.BluRay.REMUX.HEVC.DV.TrueHD.Atmos.7.1-GRP", Size: 62 * gb},
		{Release: "Night.Harbor.2010.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265-GRP", Size: 18 * gb},
		{Release: "Night.Harbor.2010.1080p.WEB-DL.DDP5.1.H.264-GRP", Size: 6 * gb},
		{Release: "Night.Harbor.2010.2160p.WEB-DL.DDP5.1.SDR.H.265-GRP", Size: 14 * gb},
	}
	first := func(p watch.StreamPrefs) string { return rankByPrefs(cands, p, film)[0].Release }

	if got := first(watch.DefaultStreamPrefs); got != cands[1].Release {
		t.Errorf("defaults: %s", got)
	}
	p := watch.DefaultStreamPrefs
	p.Quality = "1080p"
	if got := first(p); got != cands[2].Release {
		t.Errorf("1080p: %s", got)
	}
	p = watch.DefaultStreamPrefs
	p.DolbyVision = "avoid"
	if got := first(p); got != cands[3].Release {
		t.Errorf("no Dolby Vision: %s", got)
	}
	p = watch.DefaultStreamPrefs
	p.Size, p.Remux, p.LightSound = "any", "allow", false
	if got := first(p); got != cands[0].Release {
		t.Errorf("biggest allowed: %s", got)
	}
	p = watch.DefaultStreamPrefs
	p.Size = "10"
	if got := first(p); got != cands[2].Release {
		t.Errorf("10 GB limit: %s", got)
	}
	// The Internet Archive and YouTube stay last.
	withFallback := append([]playCandidate{{Release: "archive", Direct: true}}, cands...)
	if got := rankByPrefs(withFallback, watch.DefaultStreamPrefs, film); !got[len(got)-1].Direct {
		t.Errorf("fallback moved up: %+v", got[0])
	}
}
