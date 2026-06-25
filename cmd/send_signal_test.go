package cmd

import "testing"

func TestNormalizeSignal(t *testing.T) {
	tests := map[string]string{
		"usr2":    "SIGUSR2",
		"SIGTERM": "SIGTERM",
		" term ":  "SIGTERM",
	}

	for input, expected := range tests {
		if actual := normalizeSignal(input); actual != expected {
			t.Fatalf("normalizeSignal(%q): expected %q, got %q", input, expected, actual)
		}
	}
}
