package api

import (
	"testing"
	"time"
)

func TestNoticedTally(t *testing.T) {
	tl := &noticedTally{seen: map[noticedKey][]noticedHit{}, filed: map[noticedKey]int64{}}
	k := noticedKey{"movie", 603, 0, 0}
	now := time.Now()
	open := true
	still := func(int64) bool { return open }
	if tl.add(k, 1, now, still) {
		t.Fatal("one move filed a report")
	}
	if !tl.add(k, 2, now, still) {
		t.Fatal("two people moving on didn't file")
	}
	tl.filedAs(k, 9)
	for i := 0; i < 5; i++ {
		if tl.add(k, 3, now, still) {
			t.Fatal("filed again while the first is open")
		}
	}
	// Answered: counting starts again; old moves fall out of the week.
	open = false
	if tl.add(k, 1, now, still) || tl.add(k, 1, now, still) {
		t.Fatal("filed before three moves")
	}
	if !tl.add(k, 1, now, still) {
		t.Fatal("three moves didn't file")
	}
	k2 := noticedKey{"tv", 1, 1, 1}
	tl.add(k2, 1, now.Add(-8*24*time.Hour), still)
	if tl.add(k2, 2, now, still) {
		t.Fatal("a move from last week counted")
	}
}
