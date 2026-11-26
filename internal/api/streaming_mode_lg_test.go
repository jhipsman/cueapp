package api

import "testing"

func TestLGAudioFirst(t *testing.T) {
	in := []playCandidate{
		{Release: "Film.2024.2160p.WEB-DL.DTS-HD.MA.5.1.HEVC"},
		{Release: "Film.2024.1080p.BluRay.TrueHD.Atmos.x264"},
		{Release: "Film.2024.2160p.WEB-DL.DDP5.1.Atmos.HEVC"},
		{Release: "Film.2024.1080p.WEB-DL.AAC.x264"},
	}
	out := lgAudioFirst(in)
	want := []string{in[2].Release, in[3].Release, in[0].Release, in[1].Release}
	for i := range want {
		if out[i].Release != want[i] {
			t.Fatalf("order %d: %q, want %q", i, out[i].Release, want[i])
		}
	}
}
