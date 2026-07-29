package cdn

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// EditorMetadata represents CDN editors.json entries.
type EditorMetadata struct {
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	Sha512      string `json:"sha512"`
	Sha256      string `json:"sha256"`
	Size        int    `json:"size"`
	Timestamp   string `json:"timestamp"`
}

const nightlyVersionsURL = "https://blazium.app/api/versions/data/nightly"

// HTTPGet is overridable for tests.
var HTTPGet = func(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// ChannelName returns nightly or release.
func ChannelName(isNightly bool) string {
	if isNightly {
		return "nightly"
	}
	return "release"
}

// LoadEditorMetadata fetches editors.json for a version.
func LoadEditorMetadata(version string, isNightly bool) ([]EditorMetadata, error) {
	releaseType := ChannelName(isNightly)
	metadataURL := fmt.Sprintf("https://cdn.blazium.app/%s/%s/editors.json", releaseType, version)
	metadataContent, err := HTTPGet(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching editor metadata from %s: %w", metadataURL, err)
	}
	var list []EditorMetadata
	if err := json.Unmarshal(metadataContent, &list); err != nil {
		return nil, fmt.Errorf("error parsing editor metadata JSON: %w", err)
	}
	return list, nil
}

// FindEditorMetadata picks a matching editor archive from metadata.
func FindEditorMetadata(list []EditorMetadata, version, platform, arch string, isMono bool) (*EditorMetadata, error) {
	for i := range list {
		if FilenameMatchesEditor(list[i].Filename, version, platform, arch, isMono) {
			return &list[i], nil
		}
	}
	return nil, errors.New("no matching editor metadata found")
}

// FilenameMatchesEditor reports whether filename matches the requested editor.
func FilenameMatchesEditor(filename, version, platform, arch string, isMono bool) bool {
	name := strings.ToLower(filename)
	if !strings.Contains(name, strings.ToLower(version)) {
		return false
	}
	if !containsAny(name, platformTokens(platform)) {
		return false
	}
	if !containsAny(name, archTokens(arch)) {
		return false
	}
	isMonoFile := strings.Contains(name, ".mono")
	return isMono == isMonoFile
}

func platformTokens(platform string) []string {
	switch strings.ToLower(platform) {
	case "linux", "linuxbsd":
		return []string{"linux"}
	case "windows":
		return []string{"windows"}
	case "macos", "darwin":
		return []string{"macos", "darwin"}
	default:
		return []string{strings.ToLower(platform)}
	}
}

func archTokens(arch string) []string {
	switch strings.ToLower(arch) {
	case "x86_64", "amd64", "64bit":
		return []string{"x86_64", "64bit", "x64"}
	case "x86_32", "x86", "32bit", "i386":
		return []string{"x86_32", "32bit"}
	case "arm64", "aarch64":
		return []string{"arm64", "aarch64"}
	default:
		return []string{strings.ToLower(arch)}
	}
}

func containsAny(value string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

// DownloadFile downloads url to dest.
func DownloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to download file: " + resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

// TemplatesTPZURL builds the export templates .tpz URL.
func TemplatesTPZURL(version string, isMono, isNightly bool) string {
	releaseType := ChannelName(isNightly)
	suffix := "_export_templates.tpz"
	if isMono {
		suffix = "_mono_export_templates.tpz"
	}
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/Blazium_v%s%s", releaseType, version, version, suffix)
}

type nightlyEntry struct {
	DeployType string `json:"deploy_type"`
	Version    string `json:"version"`
}

// ResolveLatestNightly returns the highest semver from blazium.app nightly API.
func ResolveLatestNightly() (string, error) {
	body, err := HTTPGet(nightlyVersionsURL)
	if err != nil {
		return "", fmt.Errorf("fetch nightly versions: %w", err)
	}
	var entries []nightlyEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return "", fmt.Errorf("parse nightly versions: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no nightly versions returned")
	}
	best := entries[0].Version
	for _, e := range entries[1:] {
		if CompareSemver(e.Version, best) > 0 {
			best = e.Version
		}
	}
	if strings.TrimSpace(best) == "" {
		return "", fmt.Errorf("empty nightly version")
	}
	return best, nil
}

// CompareSemver compares dotted version strings.
func CompareSemver(a, b string) int {
	ap := parseVersionParts(a)
	bp := parseVersionParts(b)
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseVersionParts(v string) []int {
	v = strings.TrimSpace(strings.Split(v, "-")[0])
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// ResolveInstallVersion maps aliases (nightly, lts, latest) to a concrete version.
func ResolveInstallVersion(version, channel, defaultRelease string) (baseVersion string, isNightly bool, err error) {
	v := strings.TrimSpace(version)
	ch := strings.ToLower(strings.TrimSpace(channel))

	switch strings.ToLower(v) {
	case "nightly", "latest":
		baseVersion, err = ResolveLatestNightly()
		return baseVersion, true, err
	case "lts":
		if defaultRelease == "" {
			return "", false, fmt.Errorf("no default LTS/release version configured")
		}
		return strings.TrimSpace(defaultRelease), false, nil
	case "":
		if ch == "nightly" {
			baseVersion, err = ResolveLatestNightly()
			return baseVersion, true, err
		}
		if defaultRelease == "" {
			return "", false, fmt.Errorf("version is required (e.g. 0.6.714, nightly, lts)")
		}
		return strings.TrimSpace(defaultRelease), ch == "nightly", nil
	}

	isNightly = ch == "nightly" || strings.Contains(v, "-nightly")
	if strings.Contains(v, "-nightly") {
		return strings.Split(v, "-")[0], true, nil
	}
	if ch == "nightly" {
		return v, true, nil
	}
	return v, isNightly, nil
}

// DefaultPlatformArch returns GOOS/GOARCH mapped to CDN tokens.
func DefaultPlatformArch() (platform, arch string) {
	switch runtime.GOOS {
	case "windows":
		platform = "windows"
	case "darwin":
		platform = "macos"
	default:
		platform = "linux"
	}
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	default:
		arch = runtime.GOARCH
	}
	return platform, arch
}
