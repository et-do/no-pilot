package logging

import "testing"

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want Level
	}{
		{"", LevelInfo},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"error", LevelError},
		{"debug", LevelDebug},
		{"unknown", LevelInfo},
	}

	for _, tc := range cases {
		if got := ParseLevel(tc.in); got != tc.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestEnabled(t *testing.T) {
	if !Enabled(LevelInfo, LevelError) {
		t.Fatal("expected info level to include error messages")
	}
	if !Enabled(LevelDebug, LevelInfo) {
		t.Fatal("expected debug level to include info messages")
	}
	if Enabled(LevelError, LevelInfo) {
		t.Fatal("did not expect error level to include info messages")
	}
}
