package upgrade

import "testing"

func TestArchMatches(t *testing.T) {
	if !archMatches("x86_64", "amd64") {
		t.Fatal("expected arch alias match")
	}
	if !archMatches("amd64", "x86_64") {
		t.Fatal("expected reverse arch alias match")
	}
	if archMatches("arm64", "amd64") {
		t.Fatal("unexpected match")
	}
}
