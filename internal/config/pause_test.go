package config

import "testing"

func TestPauseAutomationVariable(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"no", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{" yes ", true},
		{"on", true},
	}
	for _, tc := range tests {
		t.Run("value "+tc.value, func(t *testing.T) {
			t.Setenv("CUE_PAUSE_AUTOMATION", tc.value)
			if got := Load().PauseAutomation; got != tc.want {
				t.Fatalf("CUE_PAUSE_AUTOMATION=%q gave %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// A variable set under its name from before the rename still works, and the
// new name wins when both are set.
func TestLegacyVariableNames(t *testing.T) {
	t.Setenv("CUE_PAUSE_AUTOMATION", "")
	t.Setenv("MEDIARIUM_PAUSE_AUTOMATION", "1")
	t.Setenv("MEDIARIUM_IMAGE_VERSION", "2.1.9")
	if cfg := Load(); !cfg.PauseAutomation || cfg.ImageVersion != "2.1.9" {
		t.Fatalf("old names: pause %v, image version %q", cfg.PauseAutomation, cfg.ImageVersion)
	}
	t.Setenv("CUE_IMAGE_VERSION", "2.2.0")
	if got := Load().ImageVersion; got != "2.2.0" {
		t.Fatalf("CUE_IMAGE_VERSION should win, got %q", got)
	}
}
