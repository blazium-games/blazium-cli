package config

import (
	"os"
	"testing"
)

func TestExpandString(t *testing.T) {
	t.Setenv("BLAZIUM_STEAM_USERNAME", "builder")
	t.Setenv("EMPTY_ONE", "")
	got, err := ExpandString("user=${BLAZIUM_STEAM_USERNAME} $$ok ${MISSING:-fallback}")
	if err != nil {
		t.Fatal(err)
	}
	if got != "user=builder $ok fallback" {
		t.Fatalf("%q", got)
	}
	if _, err := ExpandString("${NOT_SET}"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ExpandString("${EMPTY_ONE}"); err == nil {
		t.Fatal("expected empty error")
	}
	os.Unsetenv("EMPTY_ONE")
}

func TestBareDollar(t *testing.T) {
	if _, err := ExpandString("$FOO"); err == nil {
		t.Fatal("expected error")
	}
}
