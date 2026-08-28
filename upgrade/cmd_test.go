package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

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

func TestDownloadTempPath(t *testing.T) {
	p, err := DownloadTempPath("0.0.42")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(p)
	if !strings.Contains(filepath.Base(p), "blazium-cli-0.0.42-") {
		t.Fatalf("unexpected name %q", p)
	}
	if !strings.HasSuffix(p, ".download") {
		t.Fatalf("expected .download suffix: %q", p)
	}
	dir := filepath.Dir(p)
	tmp := os.TempDir()
	rel, err := filepath.Rel(tmp, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		// CreateTemp may resolve to a cleaned temp path; ensure writable parent.
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("temp dir missing: %v", err)
		}
	}
}

func TestIsPermissionError(t *testing.T) {
	if !IsPermissionError(os.ErrPermission) {
		t.Fatal("os.ErrPermission")
	}
	if !IsPermissionError(&os.PathError{Op: "open", Path: "x", Err: syscall.EACCES}) {
		t.Fatal("EACCES PathError")
	}
	if !IsPermissionError(errors.New("open C:\\Program Files\\Blazium\\blazium-cli.exe.new: Access is denied.")) {
		t.Fatal("windows access denied string")
	}
	if IsPermissionError(errors.New("file not found")) {
		t.Fatal("non-permission should be false")
	}
}

func TestReplaceExecutableWritable(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "blazium-cli")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "new.bin")
	if err := os.WriteFile(src, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceExecutable(dest, src); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("got %q", got)
	}
}

func TestReplaceExecutableMissingDest(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "bin", "crash_reporter")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	src := filepath.Join(dir, "new.bin")
	if err := os.WriteFile(src, []byte("first-install"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceExecutable(dest, src); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first-install" {
		t.Fatalf("got %q", got)
	}
}

func TestPsQuote(t *testing.T) {
	if got := psQuote(`C:\Program Files\a`); got != `'C:\Program Files\a'` {
		t.Fatalf("got %q", got)
	}
	if got := psQuote(`it's`); got != `'it''s'` {
		t.Fatalf("got %q", got)
	}
}
