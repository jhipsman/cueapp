package backup_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/backup"
)

func TestSavedBackupsAreKeptToTheLimitAndValid(t *testing.T) {
	db, dir := newInstall(t, "saved")
	start := time.Date(2026, 10, 3, 3, 0, 0, 0, time.Local)
	for i := 0; i < 4; i++ {
		if _, err := backup.Save(context.Background(), db, dir, "1.4.0", start.AddDate(0, 0, i)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := backup.ListSaved(dir)
	if err != nil || len(list) != 4 {
		t.Fatalf("list: %+v (err %v)", list, err)
	}
	if list[0].Name != backup.FileName(start.AddDate(0, 0, 3)) {
		t.Errorf("newest first, got %s", list[0].Name)
	}
	if n, err := backup.Prune(dir, 2); err != nil || n != 2 {
		t.Fatalf("pruned %d (err %v)", n, err)
	}
	list, _ = backup.ListSaved(dir)
	if len(list) != 2 || list[1].Name != backup.FileName(start.AddDate(0, 0, 2)) {
		t.Fatalf("kept the wrong ones: %+v", list)
	}

	// A saved backup is a normal backup: it passes the restore checks.
	p, err := backup.PathOf(dir, list[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backup.Validate(t.TempDir(), p); err != nil {
		t.Fatalf("a saved backup should validate: %v", err)
	}
	if info, _ := os.Stat(p); info.Mode().Perm()&0o077 != 0 && os.PathSeparator == '/' {
		t.Errorf("a backup holds every secret and must be private, mode %v", info.Mode())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, backup.SavedDir, ".saving-*")); len(leftovers) != 0 {
		t.Errorf("temporary files left: %v", leftovers)
	}
}

func TestPathOfRefusesOtherNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../app.db", "secret.key", "cue-backup-x.zip", "", "mediarium-backup-20261003-030000.zip/../../a"} {
		if _, err := backup.PathOf(dir, name); err == nil {
			t.Errorf("%q should be refused", name)
		}
	}
}

func TestBackupsSavedBeforeTheRenameAreListedByDate(t *testing.T) {
	db, dir := newInstall(t, "legacy")
	day := time.Date(2026, 10, 3, 3, 0, 0, 0, time.Local)
	p, err := backup.Save(context.Background(), db, dir, "2.2.0", day.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	newer := filepath.Base(p)
	// The same backup under the name used before Cue 2.2, a day older.
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	old := "mediarium-backup-" + day.Format("20060102-150405") + ".zip"
	if err := os.WriteFile(filepath.Join(dir, backup.SavedDir, old), data, 0o600); err != nil {
		t.Fatal(err)
	}

	list, err := backup.ListSaved(dir)
	if err != nil || len(list) != 2 || list[0].Name != newer || list[1].Name != old || !list[1].CreatedAt.Equal(day) {
		t.Fatalf("list: %+v (err %v)", list, err)
	}
	if _, err := backup.PathOf(dir, old); err != nil {
		t.Errorf("an old backup should still be found: %v", err)
	}
	if n, err := backup.Prune(dir, 1); err != nil || n != 1 {
		t.Fatalf("pruned %d (err %v)", n, err)
	}
	if list, _ := backup.ListSaved(dir); len(list) != 1 || list[0].Name != newer {
		t.Errorf("the older backup should be the one pruned: %+v", list)
	}
}
