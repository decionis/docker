package version

import "testing"

func TestVersionIsSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must never be empty; the default is 0.0.0-dev")
	}
}
