package api

import (
	"strings"
	"testing"
)

func TestToWebVTT(t *testing.T) {
	srt := "\xef\xbb\xbf1\r\n00:00:01,500 --> 00:00:03,000\r\nHello {\\i1}there{\\i0}\r\n\r\n2\r\n00:01:02,000 --> 00:01:04,250\r\nSecond line\r\n"
	got, err := toWebVTT([]byte(srt))
	if err != nil {
		t.Fatal(err)
	}
	want := "WEBVTT\n\n00:00:01.500 --> 00:00:03.000\nHello there\n\n00:01:02.000 --> 00:01:04.250\nSecond line\n"
	if string(got) != want {
		t.Errorf("got %q", got)
	}
	latin := []byte("1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9\n")
	got, err = toWebVTT(latin)
	if err != nil || !strings.Contains(string(got), "Café") {
		t.Errorf("latin-1: %q %v", got, err)
	}
	if _, err := toWebVTT([]byte("not a subtitle")); err == nil {
		t.Error("no cues should fail")
	}
}

func TestShiftVTT(t *testing.T) {
	vtt := "WEBVTT\n\n00:00:05.000 --> 00:00:07.000\nGone\n\n00:01:10.000 --> 00:01:12.500\nKept\n"
	got := string(shiftVTT([]byte(vtt), 60_000))
	want := "WEBVTT\n\n00:00:10.000 --> 00:00:12.500\nKept\n"
	if got != want {
		t.Errorf("got %q", got)
	}
}
