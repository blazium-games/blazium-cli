package output

import "testing"

func TestResolveFormatJSONWins(t *testing.T) {
	got, quiet := ResolveFormat("tsv", true)
	if got != "json" || !quiet {
		t.Fatalf("ResolveFormat(tsv, true) = %q, quiet=%v; want json, true", got, quiet)
	}
	got, quiet = ResolveFormat("human", true)
	if got != "json" || !quiet {
		t.Fatalf("ResolveFormat(human, true) = %q, quiet=%v; want json, true", got, quiet)
	}
}

func TestResolveFormatWithoutJSON(t *testing.T) {
	got, quiet := ResolveFormat("tsv", false)
	if got != "tsv" || quiet {
		t.Fatalf("ResolveFormat(tsv, false) = %q, quiet=%v; want tsv, false", got, quiet)
	}
	got, quiet = ResolveFormat("", false)
	if got != "human" || quiet {
		t.Fatalf("ResolveFormat(empty, false) = %q, quiet=%v; want human, false", got, quiet)
	}
}

func TestErrorWrittenTracking(t *testing.T) {
	ResetErrorWritten()
	if ErrorWritten() {
		t.Fatal("expected ErrorWritten false after reset")
	}
	ErrorJSON("human", errString("boom"))
	if !ErrorWritten() {
		t.Fatal("expected ErrorWritten true after ErrorJSON")
	}
	ResetErrorWritten()
	if ErrorWritten() {
		t.Fatal("expected ErrorWritten false after second reset")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
