package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTVCrashReport(t *testing.T) {
	s := &Server{}
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"app":"tv","version":"1.2","device":"AFTMM","trace":"java.lang.IllegalArgumentException: Key was already used"}`, http.StatusNoContent},
		{`{"app":"tv","trace":"  "}`, http.StatusBadRequest},
		{`not json`, http.StatusBadRequest},
		{`{"trace":"` + strings.Repeat("x", 40000) + `"}`, http.StatusNoContent},
	} {
		w := httptest.NewRecorder()
		s.handleTVCrash(w, httptest.NewRequest("POST", "/api/play/crash", strings.NewReader(c.body)))
		if w.Code != c.want {
			t.Errorf("%.40s: got %d, want %d", c.body, w.Code, c.want)
		}
	}
}
