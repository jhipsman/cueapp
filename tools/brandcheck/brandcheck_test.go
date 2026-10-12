// Package brandcheck holds a test that keeps the old name, Mediarium, out of
// the repository, except where it is kept on purpose so existing installs keep
// working (and in the credit to the project Cue is a fork of).
package brandcheck

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// allowed lists the files that may say the old name, and why. Add a file here
// only for the same kinds of reason.
var allowed = map[string]string{
	"LICENSE":       "Mediarium's copyright notice, which the AGPL requires us to keep",
	"docs/LEGAL.md": "where Cue comes from",
	"CHANGELOG.md":  "\"Mediarium is now Cue\" and what the rename kept working",

	"internal/config/config.go":                  "MEDIARIUM_ environment variables still work",
	"internal/config/pause_test.go":              "tests that",
	"docs/reference/environment.md":              "says so (generated from tools/docgen)",
	"tools/docgen/main.go":                       "writes that",
	"docs/INSTALL.md":                            "says so, and the old container name",
	"docker/entrypoint.sh":                       "old variable names, and updates from before the rename answer \"mediarium\"",
	"internal/selfupdate/selfupdate.go":          "reads that answer",
	"internal/selfupdate/selfupdate_test.go":     "tests that",
	"internal/selfupdate/entrypoint_test.go":     "tests that",
	"tools/push-update.sh":                       "MEDIARIUM_API_KEY still works",
	"internal/api/scripts.go":                    "scripts get MEDIARIUM_ variables too",
	"internal/api/scripts_test.go":               "tests that",
	"docs/scripts.md":                            "says so",
	"docs/FEATURES.md":                           "the recycle bin and script variables from before the rename",
	"internal/cleanup/trash.go":                  "moves .mediarium-trash into .cue-trash",
	"internal/auth/middleware.go":                "the old sign-in cookie still works",
	"internal/auth/cookie_test.go":               "tests that",
	"internal/backup/backup.go":                  "backups made before the rename restore",
	"internal/backup/saved.go":                   "saved backups from before the rename are listed",
	"internal/backup/backup_test.go":             "tests that",
	"internal/backup/saved_test.go":              "tests that",
	"internal/download/resume.go":                "downloads paused before the rename carry on",
	"internal/download/filename.go":              "a posted file can't take the old markers' names",
	"internal/download/download_fuzz_test.go":    "tests that",
	"internal/api/download_guard.go":             "renames the old download markers",
	"internal/api/download_markers_test.go":      "tests that",
	"internal/indexers/cardigann_download.go":    "old mediarium-indexer:// links still work",
	"internal/indexers/cardigann_engine_test.go": "tests that",
	"internal/mediaservers/client.go":            "Plex, Jellyfin and Emby keep seeing the same device",
	"internal/api/calendar_feed.go":              "calendar apps keep the same events",
	"internal/api/setup.go":                      "the container's name on installs from before the rename",
	"web/src/pages/Onboarding.tsx":               "the same",
	"web/src/updateSteps.ts":                     "the same",
	"tools/brandcheck/brandcheck_test.go":        "this check",
	"web/src/legacyStorage.ts":                   "moves this browser's settings to their new names",
}

// skipped are folders that are never checked: database migrations are never
// edited once released, and the rest is not ours or is generated.
var skipped = []string{".git", "node_modules", "web/dist", "db/migrations", "build", ".gradle"}

var oldName = regexp.MustCompile(`(?i)mediarium`)

func TestNoStrayMediarium(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("not at the repository root: %v", err)
	}
	var stray []string
	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			for _, s := range skipped {
				if rel == s || strings.HasSuffix(rel, "/"+s) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(data, 0) >= 0 {
			return nil // not a text file
		}
		if !oldName.Match(data) && !oldName.MatchString(rel) {
			return nil
		}
		if _, ok := allowed[rel]; ok {
			seen[rel] = true
			return nil
		}
		stray = append(stray, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(stray)
	for _, f := range stray {
		t.Errorf("%s says \"mediarium\": use Cue, or add it to the list in tools/brandcheck with the reason it must keep the old name", f)
	}
	for f := range allowed {
		if !seen[f] {
			t.Errorf("%s no longer says \"mediarium\": take it off the list in tools/brandcheck", f)
		}
	}
}
