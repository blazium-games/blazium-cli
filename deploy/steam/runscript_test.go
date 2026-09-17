package steam

import (
	"strings"
	"testing"
)

func TestRedactSteamcmdLog(t *testing.T) {
	got := redactSteamcmdLog("login user secretpass ABCDE failed", "secretpass", "ABCDE")
	if strings.Contains(got, "secretpass") || strings.Contains(got, "ABCDE") {
		t.Fatalf("%q", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("%q", got)
	}
}

func TestSteamcmdQuote(t *testing.T) {
	if steamcmdQuote("simple") != "simple" {
		t.Fatal(steamcmdQuote("simple"))
	}
	if steamcmdQuote(`a b`) != `"a b"` {
		t.Fatal(steamcmdQuote("a b"))
	}
}
