package api_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

const tvRowCrash = "thread main\njava.lang.IllegalStateException: Release should only be called once\n\tat androidx.compose.ui.focus.FocusRestorerNode.onExit(FocusRestorer.kt:88)\n\tat app.cue.tv.home.PartsKt.CardRow(Parts.kt:233)\n"

func TestCrashReportsAreKeptAndAlertOnce(t *testing.T) {
	_, base, client := loginNewServer(t)
	hook, rec := newHookRecorder(t, 200)
	postJSON[map[string]any](t, client, base+"/api/notifications", map[string]any{
		"name": "Phone", "type": "webhook", "url": hook.URL, "events": []string{"health"},
	}, http.StatusCreated)
	postJSONMethod[map[string]any](t, client, http.MethodPut, base+"/api/system/problems/notify", map[string]any{"enabled": true}, http.StatusOK)

	report := func(device, trace string) {
		t.Helper()
		postJSON[map[string]any](t, client, base+"/api/play/crash", map[string]any{"app": "tv", "version": "1.4", "device": device, "trace": trace}, http.StatusNoContent)
	}
	report("TCL 55C (Android 12)", tvRowCrash)
	report("Fire TV Stick 4K (Android 9)", strings.Replace(tvRowCrash, ":233", ":240", 1))

	list := getJSON[map[string]any](t, client, base+"/api/system/crashes")
	items, _ := list["items"].([]any)
	if len(items) != 1 || list["open"] != float64(1) || list["alerts"] != true {
		t.Fatalf("list: %+v", list)
	}
	c := items[0].(map[string]any)
	if c["count"] != float64(2) || len(c["devices"].([]any)) != 2 || c["place"] != "app.cue.tv.home.PartsKt.CardRow(Parts.kt:240)" {
		t.Fatalf("crash: %+v", c)
	}
	waitFor(t, "the first alert", func() bool { return rec.count() == 1 })
	if !strings.Contains(rec.bodies[0], "Cue for TV crashed") {
		t.Errorf("alert: %s", rec.bodies[0])
	}

	id := int(c["id"].(float64))
	url := base + "/api/system/crashes/" + strconv.Itoa(id)
	fixed := postJSON[map[string]any](t, client, url+"/fixed", map[string]any{"fixed": true}, http.StatusOK)
	if fixed["fixedAt"] == "" || fixed["fixedAt"] == nil {
		t.Fatalf("fixed: %+v", fixed)
	}
	report("TCL 55C (Android 12)", tvRowCrash) // back after a fix: alerts again
	waitFor(t, "the alert after a fix", func() bool { return rec.count() == 2 })
	if got := getJSON[map[string]any](t, client, base+"/api/system/crashes"); got["open"] != float64(1) {
		t.Errorf("not open again: %+v", got)
	}

	if status, _ := doStatus(t, client, http.MethodDelete, url); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if status, _ := doStatus(t, client, http.MethodDelete, url); status != http.StatusNotFound {
		t.Fatalf("delete again: %d", status)
	}
	if got := postJSONMethod[map[string]any](t, client, http.MethodPut, base+"/api/system/crashes/alerts", map[string]any{"enabled": false}, http.StatusOK); got["alerts"] != false {
		t.Fatalf("alerts off: %+v", got)
	}
	report("TCL 55C (Android 12)", "java.lang.NullPointerException\n\tat app.cue.tv.Other.run(Other.kt:1)")
	time.Sleep(200 * time.Millisecond)
	if rec.count() != 2 {
		t.Errorf("alerted while switched off: %d", rec.count())
	}
}
