package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplateMetadataFromTemplateFilesJSON(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.707/template_files.json":
			return []byte(`[
				{"filename":"web_nothreads_release.zip","download_url":"https://cdn.example/web.zip","platform":"web","version":"0.6.707.nightly"}
			]`), nil
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, errHTTP404
		}
	}
	got, err := loadTemplateMetadata("0.6.707", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Filename != "web_nothreads_release.zip" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadTemplateMetadataPrefersTemplateFilesOverBundle(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.744/template_files.json":
			return []byte(`[
				{"filename":"linux_release.x86_64.zip","download_url":"https://cdn.example/linux.zip","platform":"linux","version":"0.6.744.nightly"}
			]`), nil
		case "https://cdn.blazium.app/nightly/0.6.744/templates.json",
			"https://cdn.blazium.app/nightly/0.6.744/details.json":
			t.Fatalf("should not fetch %s when template_files.json succeeds", url)
			return nil, errHTTP404
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, errHTTP404
		}
	}
	got, err := loadTemplateMetadata("0.6.744", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Platform != "linux" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadTemplateMetadataFallsBackToArrayTemplatesJSON(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.744/template_files.json":
			return nil, errHTTP404
		case "https://cdn.blazium.app/nightly/0.6.744/templates.json":
			// Transition era: per-file array lived at templates.json
			return []byte(`[
				{"filename":"android_debug.apk","download_url":"https://cdn.example/android.apk","platform":"android","version":"0.6.744.nightly"}
			]`), nil
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, errHTTP404
		}
	}
	got, err := loadTemplateMetadata("0.6.744", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Filename != "android_debug.apk" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadTemplateMetadataCerebroFallback(t *testing.T) {
	t.Setenv("BLAZIUM_CEREBRO_URL", "https://cerebro.test")
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.707/template_files.json",
			"https://cdn.blazium.app/nightly/0.6.707/templates.json",
			"https://cdn.blazium.app/nightly/0.6.707/details.json":
			return nil, errHTTP404
		case "https://cerebro.test/api/v1/templates/nightly/0.6.707":
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

func TestLoadTemplateMetadataCDNOnlyWithoutCerebroEnv(t *testing.T) {
	t.Setenv("BLAZIUM_CEREBRO_URL", "")
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.707/template_files.json",
			"https://cdn.blazium.app/nightly/0.6.707/templates.json",
			"https://cdn.blazium.app/nightly/0.6.707/details.json":
			return nil, errHTTP404
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, errHTTP404
		}
	}
	_, err := loadTemplateMetadata("0.6.707", true)
	if err == nil {
		t.Fatal("expected error when CDN catalogs missing")
	}
}

func TestLoadTemplateMetadataLegacyBundle(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.707/template_files.json":
			return nil, errHTTP404
		case "https://cdn.blazium.app/nightly/0.6.707/templates.json":
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
		case "https://cdn.blazium.app/nightly/0.6.707/details.json":
			return nil, errHTTP404
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, errHTTP404
		}
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

func TestLoadTemplateMetadataDetailsJSONFallback(t *testing.T) {
	old := httpGet
	defer func() { httpGet = old }()
	httpGet = func(url string) ([]byte, error) {
		switch url {
		case "https://cdn.blazium.app/nightly/0.6.744/template_files.json",
			"https://cdn.blazium.app/nightly/0.6.744/templates.json":
			return nil, errHTTP404
		case "https://cdn.blazium.app/nightly/0.6.744/details.json":
			return []byte(`{
				"base": {
					"filename": "Blazium_v0.6.744_export_templates.tpz",
					"url": "https://cdn.example/base.tpz",
					"checksum": {"256": "aaa"}
				},
				"mono": {
					"filename": "Blazium_v0.6.744_mono_export_templates.tpz",
					"url": "https://cdn.example/mono.tpz",
					"checksum": {"256": "bbb"}
				}
			}`), nil
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, errHTTP404
		}
	}
	got, err := loadTemplateMetadata("0.6.744", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Filename != "Blazium_v0.6.744_export_templates.tpz" {
		t.Fatalf("got %+v", got)
	}
}

func TestIsPerFileTemplateManifest(t *testing.T) {
	if !isPerFileTemplateManifest([]TemplateMetadata{{Filename: "linux_release.x86_64.zip"}}) {
		t.Fatal("zip should count as per-file")
	}
	if isPerFileTemplateManifest([]TemplateMetadata{{Filename: "Blazium_v0.6.744_export_templates.tpz"}}) {
		t.Fatal("tpz-only should not count as per-file")
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
		{Filename: "linux_debug.x86_64", Platform: "linux"},
		{Filename: "linux_release.x86_64", Platform: "linux"},
		{Filename: "windows_debug_x86_64.exe", Platform: "windows"},
		{Filename: "windows_debug_x86_64_console.exe", Platform: "windows"},
		{Filename: "windows_release_x86_64.exe", Platform: "windows"},
		{Filename: "windows_release_x86_64_console.exe", Platform: "windows"},
	}
	got := selectRequiredRuntimeTemplates(all, TemplateVariantDebug)
	if len(got) != 4 {
		t.Fatalf("debug len=%d got %+v", len(got), got)
	}
	want := map[string]bool{
		"web_nothreads_debug.zip":          true,
		"linux_debug.x86_64":               true,
		"windows_debug_x86_64.exe":         true,
		"windows_debug_x86_64_console.exe": true,
	}
	for _, m := range got {
		if !want[m.Filename] {
			t.Fatalf("unexpected pick: %s", m.Filename)
		}
	}

	gotRelease := selectRequiredRuntimeTemplates(all, TemplateVariantRelease)
	if len(gotRelease) != 4 {
		t.Fatalf("release len=%d", len(gotRelease))
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
