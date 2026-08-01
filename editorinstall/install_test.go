package editorinstall

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallTemplatesFromTPZ(t *testing.T) {
	dir := t.TempDir()
	tpz := filepath.Join(dir, "templates.tpz")
	destRoot := filepath.Join(dir, "export_templates")

	if err := writeFixtureTPZ(tpz, map[string][]byte{
		"version.txt":                []byte("0.6.707 stable"),
		"web_nothreads_release.zip":  []byte("web"),
		"linux_release.x86_64.zip":   []byte("linux"),
		"windows_release.x86_64.zip": []byte("win"),
	}); err != nil {
		t.Fatal(err)
	}

	version, err := InstallTemplatesFromTPZ(tpz, destRoot)
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.6.707 stable" {
		t.Fatalf("version %q", version)
	}
	for _, name := range []string{
		"0.6.707 stable/web_nothreads_release.zip",
		"0.6.707 stable/linux_release.x86_64.zip",
		"0.6.707/web_nothreads_release.zip",
	} {
		if _, err := os.Stat(filepath.Join(destRoot, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestInstallEditorTree(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "editor.zip")
	destDir := filepath.Join(dir, "0.6.707")

	binDir := filepath.Join(dir, "extract", "Blazium_v0.6.707_linux.x86_64")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "blazium.linuxbsd.editor.x86_64")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := createZip(zipPath, map[string]string{
		"Blazium_v0.6.707_linux.x86_64/blazium.linuxbsd.editor.x86_64": binPath,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := InstallEditorTree(zipPath, destDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("binary missing: %v", err)
	}
}

func TestTemplateShortVersion(t *testing.T) {
	if got := TemplateShortVersion("0.6.707.stable.custom"); got != "0.6.707.stable.custom" {
		t.Fatalf("got %q", got)
	}
	if got := TemplateShortVersion("0.6.707 stable custom"); got != "0.6.707" {
		t.Fatalf("got %q want 0.6.707", got)
	}
}

func TestInstallTemplateFilesAlias(t *testing.T) {
	dir := t.TempDir()
	destRoot := filepath.Join(dir, "export_templates")
	src := filepath.Join(dir, "linux_release.x86_64.zip")
	if err := os.WriteFile(src, []byte("linux"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallTemplateFiles(destRoot, "0.6.707 stable (4.3.2.stable.custom_build).abc123", []string{src}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "0.6.707/linux_release.x86_64.zip")); err != nil {
		t.Fatal(err)
	}
}

func writeFixtureTPZ(path string, files map[string][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	for name, data := range files {
		entry, err := w.Create(name)
		if err != nil {
			_ = w.Close()
			_ = f.Close()
			return err
		}
		if _, err := entry.Write(data); err != nil {
			_ = w.Close()
			_ = f.Close()
			return err
		}
	}
	if err := w.Close(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func createZip(zipPath string, src map[string]string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	for name, srcPath := range src {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			_ = w.Close()
			_ = f.Close()
			return err
		}
		entry, err := w.Create(name)
		if err != nil {
			_ = w.Close()
			_ = f.Close()
			return err
		}
		if _, err := entry.Write(data); err != nil {
			_ = w.Close()
			_ = f.Close()
			return err
		}
	}
	if err := w.Close(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
