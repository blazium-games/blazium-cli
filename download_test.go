package main_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestDownloadTemplateHelp(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "template", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run help: %v\n%s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "Download templates or editors") {
		t.Errorf("expected help output, got: %s", string(out))
	}
}

func TestDownloadTemplateMissingDest(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "template")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected error for missing destination, got none")
	}
	if !strings.Contains(string(out), "requires at least 2 arg") {
		t.Errorf("expected error about arguments, got: %s", string(out))
	}
}

func TestRootHelp(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run root help: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Blazium CLI help") {
		t.Errorf("expected root help output, got: %s", string(out))
	}
}

func TestDownloadHelp(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run download help: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Download templates or editors") {
		t.Errorf("expected download help output, got: %s", string(out))
	}
}

func TestDownloadEditorMissingPlatformArch(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "editor", ".")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected error for missing --platform and --arch, got none")
	}
	if !strings.Contains(string(out), "requires both --platform and --arch flags") {
		t.Errorf("expected error about platform/arch, got: %s", string(out))
	}
}

func TestDownloadEditorWithPlatformArch(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "editor", "--platform", "linux", "--arch", "x86_64", "./tmp")
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "Error loading editor metadata") {
		t.Fatalf("unexpected error: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Downloading editor from") && !strings.Contains(string(out), "Error loading editor metadata") {
		t.Errorf("expected download or metadata error, got: %s", string(out))
	}
	fmt.Print(string(out))
}

func _TestDownloadTemplateMono(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "template", "--mono", "./tmp")
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "Downloading template from") {
		t.Fatalf("unexpected error: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Downloading template from") && !strings.Contains(string(out), "Error downloading template") {
		t.Errorf("expected mono template download, got: %s", string(out))
	}
}

func _TestDownloadTemplateWithVersion(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "template", "--get-version", "0.5.68", "./tmp")
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "Downloading template from") {
		t.Fatalf("unexpected error: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Downloading template from") && !strings.Contains(string(out), "Error downloading template") {
		t.Errorf("expected template download with version, got: %s", string(out))
	}
}

func _TestDownloadSilentFlag(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "download", "template", "--silent", "./tmp")
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "Downloading template from") {
		t.Fatalf("unexpected error: %v\n%s", err, string(out))
	}
	if strings.Contains(string(out), "Downloading template from") {
		t.Errorf("expected no output due to --silent, got: %s", string(out))
	}
}
