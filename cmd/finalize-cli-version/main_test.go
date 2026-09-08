package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFinalizeCLIVersionSkipProbe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go run integration test")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	cmd := exec.Command("go", "run", ".",
		"--computed", "0.0.43",
		"--baseline", "0.1.0",
		"--cdn-latest", "0.1.3",
		"--skip-probe",
	)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	got := strings.TrimSpace(lines[len(lines)-1])
	if got != "0.1.4" {
		t.Fatalf("got %q want 0.1.4\n%s", got, out)
	}
}
