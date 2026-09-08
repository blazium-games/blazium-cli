package manifest

import (
	"fmt"
	"strings"
)

const CDNBaseURL = "https://cdn.blazium.app/cli"

// VersionedUpload is one local artifact that belongs at a versioned CDN object key.
type VersionedUpload struct {
	RelPath   string
	ObjectKey string
	URL       string
}

// VersionedUploads lists binaries (and unix signatures) for a publish version.
func VersionedUploads(version string) []VersionedUpload {
	v := strings.TrimSpace(version)
	if v == "" {
		return nil
	}
	files := []struct {
		rel  string
		name string
	}{
		{"linux/x86_64/blazium-cli", "blazium-cli"},
		{"linux/x86_64/blazium-cli.sig", "blazium-cli.sig"},
		{"linux/x86_32/blazium-cli", "blazium-cli"},
		{"linux/x86_32/blazium-cli.sig", "blazium-cli.sig"},
		{"windows/x86_64/blazium-cli.exe", "blazium-cli.exe"},
		{"windows/x86_32/blazium-cli.exe", "blazium-cli.exe"},
		{"darwin/x86_64/blazium-cli", "blazium-cli"},
		{"darwin/x86_64/blazium-cli.sig", "blazium-cli.sig"},
	}
	out := make([]VersionedUpload, 0, len(files))
	for _, f := range files {
		dir := strings.TrimSuffix(f.rel, "/"+f.name)
		out = append(out, VersionedUpload{
			RelPath:   f.rel,
			ObjectKey: fmt.Sprintf("cli/%s/%s/%s", dir, v, f.name),
			URL:       fmt.Sprintf("%s/%s/%s/%s", CDNBaseURL, dir, v, f.name),
		})
	}
	return out
}

// VersionedObjectURLs are the public binaries that mark a CLI version as already published.
func VersionedObjectURLs(version string) []string {
	var urls []string
	for _, u := range VersionedUploads(version) {
		if strings.HasSuffix(u.RelPath, ".sig") {
			continue
		}
		urls = append(urls, u.URL)
	}
	return urls
}

// NextPublishVersion picks the next CLI semver to upload.
//
// It takes max(computed, baseline), bumps past cdnLatest when that is not older,
// then keeps bumping while occupied reports that versioned CDN objects already exist.
// That avoids overwriting a failed prior publish (Windows Authenticode hashes change
// each sign, and the CDN can keep serving the previous object).
func NextPublishVersion(computed, baseline, cdnLatest string, occupied func(string) bool) string {
	computed = strings.TrimSpace(computed)
	baseline = strings.TrimSpace(baseline)
	cdnLatest = strings.TrimSpace(cdnLatest)
	if cdnLatest == "null" {
		cdnLatest = ""
	}

	candidate := computed
	if candidate == "" || (baseline != "" && compareSemver(baseline, candidate) > 0) {
		candidate = baseline
	}
	if candidate == "" {
		return ""
	}

	if cdnLatest != "" && compareSemver(cdnLatest, candidate) >= 0 {
		candidate = bumpPatch(cdnLatest)
	}

	for occupied != nil && occupied(candidate) {
		next := bumpPatch(candidate)
		if next == candidate {
			break
		}
		candidate = next
	}
	return candidate
}

func bumpPatch(v string) string {
	parts := parseParts(v)
	for len(parts) < 3 {
		parts = append(parts, 0)
	}
	parts[2]++
	return fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2])
}
