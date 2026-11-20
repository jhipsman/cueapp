package crashes

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/store"
)

const rowCrash = `thread main
java.lang.IllegalStateException: Release should only be called once
	at androidx.compose.foundation.lazy.layout.LazyLayoutPinnableItem.release(LazyLayoutPinnableItem.kt:142)
	at androidx.compose.ui.focus.FocusRestorerNode.onExit(FocusRestorer.kt:88)
	at app.cue.tv.home.PartsKt.CardRow(Parts.kt:233)
	at android.os.Looper.loop(Looper.java:288)
`

func open(t *testing.T) *Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return Open(db)
}

func TestDescribe(t *testing.T) {
	h, p := Describe(rowCrash)
	if h != "java.lang.IllegalStateException: Release should only be called once" {
		t.Errorf("headline %q", h)
	}
	if p != "app.cue.tv.home.PartsKt.CardRow(Parts.kt:233)" {
		t.Errorf("place %q", p)
	}
	web := "TypeError: Cannot read properties of undefined (reading 'title')\n    at Row (https://cue.local/assets/index-Ab12Cd34.js:40:1201)\n    at https://cue.local/src/pages/Library.tsx:12:3"
	if h, p := Describe(web); h != "TypeError: Cannot read properties of undefined (reading 'title')" || !strings.Contains(p, "/src/pages/Library.tsx") {
		t.Errorf("web: %q %q", h, p)
	}
	if _, p := Describe("Boom\n\tat java.lang.Thread.run(Thread.java:1)"); p != "java.lang.Thread.run(Thread.java:1)" {
		t.Errorf("no Cue frame: %q", p)
	}
}

func TestSignatureIgnoresLineNumbersAndMessage(t *testing.T) {
	moved := strings.ReplaceAll(rowCrash, "Parts.kt:233", "Parts.kt:240")
	moved = strings.ReplaceAll(moved, "once", "once (item 7)")
	if Signature("tv", rowCrash) != Signature("tv", moved) {
		t.Error("a moved line or a different message made a new crash")
	}
	if Signature("tv", rowCrash) == Signature("web", rowCrash) {
		t.Error("two apps share a crash")
	}
	other := strings.ReplaceAll(rowCrash, "IllegalStateException", "NullPointerException")
	if Signature("tv", rowCrash) == Signature("tv", other) {
		t.Error("a different exception folded in")
	}
}

func TestRecordFoldsAndReopens(t *testing.T) {
	s := open(t)
	now := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })

	c, news, err := s.Record(Report{App: "tv", Version: "1.4", Device: "TCL 55C (Android 12)", Trace: rowCrash})
	if err != nil || !news || c.Count != 1 || c.Place == "" {
		t.Fatalf("first: %+v %v %v", c, news, err)
	}
	now = now.Add(time.Hour)
	c, news, err = s.Record(Report{App: "tv", Version: "1.5", Device: "Fire TV Stick 4K (Android 9)", Trace: strings.ReplaceAll(rowCrash, ":233", ":250")})
	if err != nil || news || c.Count != 2 || len(c.Devices) != 2 || len(c.Versions) != 2 || c.Version != "1.5" {
		t.Fatalf("repeat: %+v %v %v", c, news, err)
	}
	if !c.LastAt.Equal(now) || c.FirstAt.Equal(now) {
		t.Errorf("times %v %v", c.FirstAt, c.LastAt)
	}
	if err := s.SetFixed(c.ID, true); err != nil || s.OpenCount() != 0 {
		t.Fatalf("fixed: %v %d", err, s.OpenCount())
	}
	c, news, _ = s.Record(Report{App: "tv", Trace: rowCrash})
	if !news || c.FixedAt != "" || c.Count != 3 {
		t.Errorf("back after a fix: %+v %v", c, news)
	}
	if err := s.SetFixed(999, true); err != ErrNotFound {
		t.Errorf("unknown id: %v", err)
	}
	list, _ := s.List()
	if len(list) != 1 {
		t.Fatalf("list %d", len(list))
	}
	if err := s.Delete(c.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Errorf("deleted crash still listed")
	}
}

func TestRecordKeepsAtMostMaxRows(t *testing.T) {
	s := open(t)
	for i := 0; i < MaxRows+5; i++ {
		trace := "E" + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + ": boom\n\tat app.cue.tv.F" + strings.Repeat("y", i) + "(F.kt:1)"
		if _, _, err := s.Record(Report{App: "tv", Trace: trace}); err != nil {
			t.Fatal(err)
		}
	}
	if list, _ := s.List(); len(list) != MaxRows {
		t.Errorf("kept %d", len(list))
	}
}
