package cli

import "testing"

func TestVersionConstant(t *testing.T) {
	if Version == "" {
		t.Error("Version must not be empty")
	}
	if Version != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %q", Version)
	}
}
