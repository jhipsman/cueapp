package api

import (
	"testing"
	"time"
)

func TestPartyVersionFirst(t *testing.T) {
	p := &party{code: "VERSTEST", members: map[string]*partyMember{}, subs: map[chan []byte]string{}, active: time.Now()}
	parties.Lock()
	parties.byCode[p.code] = p
	parties.Unlock()
	t.Cleanup(func() {
		parties.Lock()
		delete(parties.byCode, p.code)
		parties.Unlock()
	})
	cands := []playCandidate{
		{Release: "Film.2160p.DV.HEVC.TrueHD", URL: "u1"},
		{Release: "Film.1080p.DDP5.1.H.264", URL: "u2"},
		{Release: "Film.1080p.AAC.x264", URL: "u3"},
		{Release: "Film.1080p.x265", URL: "u4", Hash: "abc"},
	}
	first := func(c []playCandidate) string { return c[0].Release }
	// With Premiumize, its converted copy is the easiest for everyone.
	if got := first(partyVersionFirst("verstest", cands, true)); got != "Film.1080p.x265" {
		t.Fatalf("chosen = %s", got)
	}
	// Later players get the same one first, whatever order they'd use.
	rev := []playCandidate{cands[3], cands[2], cands[1], cands[0]}
	if got := first(partyVersionFirst("VERSTEST", rev, true)); got != "Film.1080p.x265" {
		t.Fatalf("second player got %s", got)
	}
	// Without Premiumize (a new party): browser-friendly picture and sound.
	p.release = ""
	if got := first(partyVersionFirst("VERSTEST", cands, false)); got != "Film.1080p.AAC.x264" {
		t.Fatalf("without Premiumize: %s", got)
	}
	// No such party: unchanged.
	if got := first(partyVersionFirst("NOPE", cands, true)); got != cands[0].Release {
		t.Fatalf("unknown party changed the order: %s", got)
	}
}
