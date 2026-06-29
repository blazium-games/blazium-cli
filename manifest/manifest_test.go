package manifest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMergeVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blazium-cli")
	if err := os.WriteFile(path, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	doc := Document{Versions: map[string]Version{}}
	if err := MergeVersion(&doc, "1.2.3", time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC), []BuildInput{{
		Platform: "linux",
		Arch:     "x86_64",
		Filename: "blazium-cli",
		Path:     path,
		BaseURL:  "https://cdn.blazium.app/cli/linux/1.2.3",
		SigURL:   "https://cdn.blazium.app/cli/linux/1.2.3/blazium-cli.sig",
		Signing:  "gpg",
	}}); err != nil {
		t.Fatal(err)
	}
	if doc.Latest != "1.2.3" {
		t.Fatalf("latest=%q", doc.Latest)
	}
	if len(doc.Versions["1.2.3"].Downloads) != 1 {
		t.Fatalf("downloads=%+v", doc.Versions["1.2.3"].Downloads)
	}
}

func TestMergeDocumentsPreservesHistory(t *testing.T) {
	base := Document{
		Latest: "1.0.0",
		Versions: map[string]Version{
			"1.0.0": {ReleasedOn: "2026-01-01T00:00:00Z"},
		},
	}
	update := Document{
		Latest: "1.2.0",
		Versions: map[string]Version{
			"1.2.0": {ReleasedOn: "2026-06-01T00:00:00Z"},
		},
	}
	merged := MergeDocuments(base, update)
	if _, ok := merged.Versions["1.0.0"]; !ok {
		t.Fatal("lost historical version")
	}
	if merged.Latest != "1.2.0" {
		t.Fatalf("latest=%q", merged.Latest)
	}
}

func TestCompareSemver(t *testing.T) {
	if compareSemver("1.2.3", "1.2.2") <= 0 {
		t.Fatal("expected 1.2.3 > 1.2.2")
	}
}
