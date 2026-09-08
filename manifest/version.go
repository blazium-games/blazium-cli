package manifest

import (
	"fmt"
	"strings"
)

const CDNBaseURL = "https://cdn.blazium.app/cli"

// VersionedObjectURLs are the public binaries that mark a CLI version as already published.
func VersionedObjectURLs(version string) []string {
	v := strings.TrimSpace(version)
	if v == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("%s/linux/x86_64/%s/blazium-cli", CDNBaseURL, v),
		fmt.Sprintf("%s/linux/x86_32/%s/blazium-cli", CDNBaseURL, v),
		fmt.Sprintf("%s/windows/x86_64/%s/blazium-cli.exe", CDNBaseURL, v),
		fmt.Sprintf("%s/windows/x86_32/%s/blazium-cli.exe", CDNBaseURL, v),
		fmt.Sprintf("%s/darwin/x86_64/%s/blazium-cli", CDNBaseURL, v),
	}
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
