package manifest

import (
	"os"
	"path/filepath"
	"strings"
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

func TestNextPublishVersion(t *testing.T) {
	t.Parallel()

	occupied := func(taken ...string) func(string) bool {
		set := map[string]struct{}{}
		for _, v := range taken {
			set[v] = struct{}{}
		}
		return func(v string) bool {
			_, ok := set[v]
			return ok
		}
	}

	cases := []struct {
		name      string
		computed  string
		baseline  string
		cdnLatest string
		taken     []string
		want      string
	}{
		{
			name:      "public line starts at 0.1.0 then bumps past CDN latest",
			computed:  "0.0.43",
			baseline:  "0.1.0",
			cdnLatest: "0.1.3",
			want:      "0.1.4",
		},
		{
			name:      "skip version whose CDN objects already exist",
			computed:  "0.0.43",
			baseline:  "0.1.0",
			cdnLatest: "0.1.3",
			taken:     []string{"0.1.4"},
			want:      "0.1.5",
		},
		{
			name:      "skip a run of occupied versions",
			computed:  "0.0.43",
			baseline:  "0.1.0",
			cdnLatest: "0.1.3",
			taken:     []string{"0.1.4", "0.1.5"},
			want:      "0.1.6",
		},
		{
			name:      "keep computed when it is already newer and free",
			computed:  "0.2.0",
			baseline:  "0.1.0",
			cdnLatest: "0.1.3",
			want:      "0.2.0",
		},
		{
			name:      "bump computed when that version is already on the CDN",
			computed:  "0.2.0",
			baseline:  "0.1.0",
			cdnLatest: "0.1.3",
			taken:     []string{"0.2.0"},
			want:      "0.2.1",
		},
		{
			name:     "empty CDN latest uses max computed baseline",
			computed: "0.0.10",
			baseline: "0.1.0",
			want:     "0.1.0",
		},
		{
			name:      "treat JSON null latest as empty",
			computed:  "0.1.0",
			baseline:  "0.1.0",
			cdnLatest: "null",
			want:      "0.1.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NextPublishVersion(tc.computed, tc.baseline, tc.cdnLatest, occupied(tc.taken...))
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestVersionedObjectURLs(t *testing.T) {
	t.Parallel()
	urls := VersionedObjectURLs("0.1.4")
	if len(urls) != 5 {
		t.Fatalf("urls=%d", len(urls))
	}
	want := "https://cdn.blazium.app/cli/windows/x86_32/0.1.4/blazium-cli.exe"
	found := false
	for _, u := range urls {
		if u == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing %s in %v", want, urls)
	}
	if VersionedObjectURLs(" ") != nil {
		t.Fatal("expected nil for blank version")
	}
}

func TestVersionedUploads(t *testing.T) {
	t.Parallel()
	uploads := VersionedUploads("0.1.6")
	if len(uploads) != 8 {
		t.Fatalf("uploads=%d", len(uploads))
	}
	wantKey := "cli/windows/x86_32/0.1.6/blazium-cli.exe"
	found := false
	for _, u := range uploads {
		if u.RelPath == "windows/x86_32/blazium-cli.exe" {
			if u.ObjectKey != wantKey {
				t.Fatalf("object key=%q want %q", u.ObjectKey, wantKey)
			}
			if u.URL != CDNBaseURL+"/windows/x86_32/0.1.6/blazium-cli.exe" {
				t.Fatalf("url=%q", u.URL)
			}
			found = true
		}
		if strings.Contains(u.ObjectKey, "//") {
			t.Fatalf("double slash in %q", u.ObjectKey)
		}
	}
	if !found {
		t.Fatal("missing windows x86_32 upload")
	}
	if VersionedUploads(" ") != nil {
		t.Fatal("expected nil for blank version")
	}
}
