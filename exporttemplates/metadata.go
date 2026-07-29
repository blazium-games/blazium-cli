package exporttemplates

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
)

// Entry matches cdn.blazium.app/{channel}/{version}/template_files.json entries.
type Entry struct {
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	Sha256      string `json:"sha256"`
	Size        int    `json:"size"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	Mono        bool   `json:"mono"`
	Version     string `json:"version"`
}

// HTTPGet is overridable for tests.
var HTTPGet = cdn.HTTPGet

type cerebroTemplatesResponse struct {
	Success bool    `json:"success"`
	Data    []Entry `json:"data"`
}

func deployTypeName(isNightly bool) string {
	return cdn.ChannelName(isNightly)
}

func templateFilesJSONURL(version string, isNightly bool) string {
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/template_files.json", cdn.ChannelName(isNightly), version)
}

func templatesJSONURL(version string, isNightly bool) string {
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/templates.json", cdn.ChannelName(isNightly), version)
}

func detailsJSONURL(version string, isNightly bool) string {
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/details.json", cdn.ChannelName(isNightly), version)
}

func cerebroTemplatesURL(deployType, version string) string {
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("BLAZIUM_CEREBRO_URL")), "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/templates/%s/%s", base, deployType, version)
}

// LoadMetadata resolves export template catalog entries from CDN.
func LoadMetadata(version string, isNightly bool) ([]Entry, error) {
	candidates := []string{
		templateFilesJSONURL(version, isNightly),
		templatesJSONURL(version, isNightly),
		detailsJSONURL(version, isNightly),
	}

	var bundleFallback []Entry
	var lastErr error
	for _, url := range candidates {
		body, err := HTTPGet(url)
		if err != nil {
			if isHTTPNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("fetch template metadata from %s: %w", url, err)
		}
		entries, parseErr := parseMetadataJSON(body)
		if parseErr != nil {
			lastErr = fmt.Errorf("parse %s: %w", url, parseErr)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		if isPerFileManifest(entries) {
			return entries, nil
		}
		if bundleFallback == nil {
			bundleFallback = entries
		}
	}

	if url := cerebroTemplatesURL(deployTypeName(isNightly), version); url != "" {
		cerebroEntries, cerebroErr := loadMetadataFromCerebro(deployTypeName(isNightly), version)
		if cerebroErr == nil && len(cerebroEntries) > 0 {
			return cerebroEntries, nil
		}
		if cerebroErr != nil {
			lastErr = fmt.Errorf("internal registry fallback: %w", cerebroErr)
		}
	}
	if len(bundleFallback) > 0 {
		return bundleFallback, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no template metadata found for %s (%s)", version, deployTypeName(isNightly))
}

func isPerFileManifest(entries []Entry) bool {
	for _, e := range entries {
		name := strings.ToLower(strings.TrimSpace(e.Filename))
		if name == "" {
			continue
		}
		if !strings.HasSuffix(name, ".tpz") {
			return true
		}
	}
	return false
}

func parseMetadataJSON(body []byte) ([]Entry, error) {
	var entries []Entry
	if err := json.Unmarshal(body, &entries); err == nil {
		return entries, nil
	}

	var wrapped struct {
		Files []Entry `json:"files"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Files) > 0 {
		return wrapped.Files, nil
	}

	var legacy struct {
		Base *legacyBundle `json:"base"`
		Mono *legacyBundle `json:"mono"`
	}
	if err := json.Unmarshal(body, &legacy); err != nil {
		return nil, err
	}
	return legacyBundlesToEntries(legacy.Base, legacy.Mono), nil
}

type legacyBundle struct {
	Filename string `json:"filename"`
	Filesize string `json:"filesize"`
	URL      string `json:"url"`
	Checksum struct {
		Sha256 string `json:"256"`
	} `json:"checksum"`
}

func legacyBundlesToEntries(base, mono *legacyBundle) []Entry {
	var out []Entry
	if base != nil && base.Filename != "" {
		out = append(out, legacyBundleToEntry(*base, false))
	}
	if mono != nil && mono.Filename != "" {
		out = append(out, legacyBundleToEntry(*mono, true))
	}
	return out
}

func legacyBundleToEntry(b legacyBundle, mono bool) Entry {
	return Entry{
		Filename:    b.Filename,
		DownloadURL: b.URL,
		Sha256:      b.Checksum.Sha256,
		Mono:        mono,
	}
}

func loadMetadataFromCerebro(deployType, version string) ([]Entry, error) {
	url := cerebroTemplatesURL(deployType, version)
	body, err := HTTPGet(url)
	if err != nil {
		return nil, fmt.Errorf("fetch template registry from %s: %w", url, err)
	}
	var resp cerebroTemplatesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse template registry: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("template registry request failed")
	}
	return resp.Data, nil
}

func isHTTPNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}
