package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplateMetadataFromCDN(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		if url != "https://cdn.blazium.app/nightly/0.6.707/templates.json" {
			t.Fatalf("unexpected url %s", url)
		}
		return []byte(`[
			{"filename":"web_nothreads_release.zip","download_url":"https://cdn.example/web.zip","platform":"web","version":"0.6.707 stable"}
		]`), nil
	}
	got, err := loadTemplateMetadata("0.6.707", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Filename != "web_nothreads_release.zip" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadTemplateMetadataCerebroFallback(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.707/templates.json":
			return nil, errHTTP404
		case "https://blazium.app/api/templates/nightly/0.6.707":
			return []byte(`{"success":true,"data":[{"filename":"linux_release.x86_64.zip","download_url":"https://cdn.example/linux.zip","platform":"linux","arch":"x86_64"}]}`), nil
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, errHTTP404
		}
	}
	got, err := loadTemplateMetadata("0.6.707", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Platform != "linux" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadTemplateMetadataLegacyBundle(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		return []byte(`{
			"base": {
				"filename": "Blazium_v0.6.707_export_templates.tpz",
				"url": "https://cdn.example/base.tpz",
				"checksum": {"256": "abc123"}
			},
			"mono": {
				"filename": "Blazium_v0.6.707_mono_export_templates.tpz",
				"url": "https://cdn.example/mono.tpz",
				"checksum": {"256": "def456"}
			}
		}`), nil
	}
	got, err := loadTemplateMetadata("0.6.707", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d got %+v", len(got), got)
	}
	if !got[1].Mono || got[1].Sha256 != "def456" {
		t.Fatalf("mono bundle: %+v", got[1])
	}
}

func TestFilterTemplateMetadataByPlatform(t *testing.T) {
	all := []TemplateMetadata{
		{Filename: "web_nothreads_release.zip", Platform: "web"},
		{Filename: "linux_release.x86_64.zip", Platform: "linux"},
	}
	got := filterTemplateMetadata(all, templateFilterOptions{platform: "web"})
	if len(got) != 1 || got[0].Platform != "web" {
		t.Fatalf("got %+v", got)
	}
}

func TestSelectRequiredRuntimeTemplates(t *testing.T) {
	all := []TemplateMetadata{
		{Filename: "web_nothreads_debug.zip", Platform: "web"},
		{Filename: "web_nothreads_release.zip", Platform: "web"},
		{Filename: "linux_release.x86_64.zip", Platform: "linux"},
		{Filename: "windows_release.x86_64.zip", Platform: "windows"},
	}
	got := selectRequiredRuntimeTemplates(all)
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Filename != "web_nothreads_release.zip" {
		t.Fatalf("web pick: %+v", got[0])
	}
}

func TestInstallTemplateFiles(t *testing.T) {
	dir := t.TempDir()
	destRoot := filepath.Join(dir, "export_templates")
	src := filepath.Join(dir, "web_nothreads_release.zip")
	if err := os.WriteFile(src, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallTemplateFiles(destRoot, "0.6.707 stable", []string{src}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"0.6.707 stable/web_nothreads_release.zip",
		"0.6.707 stable/version.txt",
		"0.6.707/web_nothreads_release.zip",
	} {
		if _, err := os.Stat(filepath.Join(destRoot, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

var errHTTP404 = &httpStatusError{status: "404 Not Found"}

type httpStatusError struct{ status string }

func (e *httpStatusError) Error() string { return "failed to download file: HTTP " + e.status }
