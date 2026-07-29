package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TemplateMetadata matches cdn.blazium.app/{channel}/{version}/template_files.json
// entries (and Cerebro GET /api/v1/templates/{deploy_type}/{version}).
type TemplateMetadata struct {
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	Sha256      string `json:"sha256"`
	Size        int    `json:"size"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	Mono        bool   `json:"mono"`
	Version     string `json:"version"`
}

type cerebroTemplatesResponse struct {
	Success bool               `json:"success"`
	Data    []TemplateMetadata `json:"data"`
}

func deployTypeName(isNightly bool) string {
	return channelName(isNightly)
}

// templateFilesJSONURL is the per-file Cerebro/CDN manifest written by ci_cd.
func templateFilesJSONURL(version string, isNightly bool) string {
	ch := channelName(isNightly)
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/template_files.json", ch, version)
}

// templatesJSONURL is legacy: older nightlies stored the per-file array here;
// current ci_cd restores the Godot {base,mono} bundle at this path.
func templatesJSONURL(version string, isNightly bool) string {
	ch := channelName(isNightly)
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/templates.json", ch, version)
}

// detailsJSONURL is the Godot Export Template Manager bundle schema.
func detailsJSONURL(version string, isNightly bool) string {
	ch := channelName(isNightly)
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/details.json", ch, version)
}

func cerebroTemplatesURL(deployType, version string) string {
	// Undocumented internal override for Blazium engineers only (not public SoT).
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("BLAZIUM_CEREBRO_URL")), "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/templates/%s/%s", base, deployType, version)
}

func loadTemplateMetadata(version string, isNightly bool) ([]TemplateMetadata, error) {
	// CDN-only public path: template_files.json → templates.json → details.json.
	candidates := []string{
		templateFilesJSONURL(version, isNightly),
		templatesJSONURL(version, isNightly),
		detailsJSONURL(version, isNightly),
	}

	var bundleFallback []TemplateMetadata
	var lastErr error
	for _, url := range candidates {
		body, err := fetchRemoteJSON(url)
		if err != nil {
			if isHTTPNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("fetch template metadata from %s: %w", url, err)
		}
		entries, parseErr := parseTemplateMetadataJSON(body)
		if parseErr != nil {
			lastErr = fmt.Errorf("parse %s: %w", url, parseErr)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		if isPerFileTemplateManifest(entries) {
			return entries, nil
		}
		if bundleFallback == nil {
			bundleFallback = entries
		}
	}

	if url := cerebroTemplatesURL(deployTypeName(isNightly), version); url != "" {
		cerebroEntries, cerebroErr := loadTemplateMetadataFromCerebro(deployTypeName(isNightly), version)
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

func isPerFileTemplateManifest(entries []TemplateMetadata) bool {
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

func parseTemplateMetadataJSON(body []byte) ([]TemplateMetadata, error) {
	var entries []TemplateMetadata
	if err := json.Unmarshal(body, &entries); err == nil {
		return entries, nil
	}

	var wrapped struct {
		Files []TemplateMetadata `json:"files"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Files) > 0 {
		return wrapped.Files, nil
	}

	var legacy struct {
		Base *legacyTemplateBundle `json:"base"`
		Mono *legacyTemplateBundle `json:"mono"`
	}
	if err := json.Unmarshal(body, &legacy); err != nil {
		return nil, err
	}
	return legacyBundlesToMetadata(legacy.Base, legacy.Mono), nil
}

type legacyTemplateBundle struct {
	Filename string `json:"filename"`
	Filesize string `json:"filesize"`
	URL      string `json:"url"`
	Checksum struct {
		Sha256 string `json:"256"`
	} `json:"checksum"`
}

func legacyBundlesToMetadata(base, mono *legacyTemplateBundle) []TemplateMetadata {
	var out []TemplateMetadata
	if base != nil && base.Filename != "" {
		out = append(out, legacyBundleToMetadata(*base, false))
	}
	if mono != nil && mono.Filename != "" {
		out = append(out, legacyBundleToMetadata(*mono, true))
	}
	return out
}

func legacyBundleToMetadata(b legacyTemplateBundle, mono bool) TemplateMetadata {
	return TemplateMetadata{
		Filename:    b.Filename,
		DownloadURL: b.URL,
		Sha256:      b.Checksum.Sha256,
		Mono:        mono,
	}
}

func loadTemplateMetadataFromCerebro(deployType, version string) ([]TemplateMetadata, error) {
	url := cerebroTemplatesURL(deployType, version)
	body, err := fetchRemoteJSON(url)
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

func filterTemplateMetadata(all []TemplateMetadata, opts templateFilterOptions) []TemplateMetadata {
	var out []TemplateMetadata
	wantFiles := make(map[string]bool)
	for _, f := range opts.files {
		wantFiles[strings.ToLower(strings.TrimSpace(f))] = true
	}
	platform := strings.ToLower(strings.TrimSpace(opts.platform))

	for _, m := range all {
		if opts.monoOnly && !m.Mono {
			continue
		}
		if opts.skipMono && m.Mono {
			continue
		}
		if platform != "" && strings.ToLower(m.Platform) != platform {
			continue
		}
		if len(wantFiles) > 0 && !wantFiles[strings.ToLower(m.Filename)] {
			continue
		}
		out = append(out, m)
	}
	return out
}

type templateFilterOptions struct {
	files    []string
	platform string
	monoOnly bool
	skipMono bool
}

func templateInstallVersion(entries []TemplateMetadata, fallback string) string {
	for _, e := range entries {
		if v := strings.TrimSpace(e.Version); v != "" {
			return v
		}
	}
	return fallback
}

func templateAlreadyInstalled(templatesDest, engineVersion, filename string) bool {
	if templatesDest == "" {
		return false
	}
	for _, dir := range templateVersionDirs(templatesDest, engineVersion) {
		if _, err := os.Stat(filepath.Join(dir, filename)); err == nil {
			return true
		}
	}
	return false
}

func templateVersionDirs(templatesDest, engineVersion string) []string {
	dirs := []string{filepath.Join(templatesDest, engineVersion)}
	if short := templateShortVersion(engineVersion); short != engineVersion {
		dirs = append(dirs, filepath.Join(templatesDest, short))
	}
	return dirs
}

func downloadTemplateFile(url, dest string, expectedSHA256 string) error {
	if err := downloadFile(url, dest); err != nil {
		return err
	}
	if strings.TrimSpace(expectedSHA256) == "" {
		return nil
	}
	sum, err := fileSHA256(dest)
	if err != nil {
		return err
	}
	want := strings.ToLower(strings.TrimSpace(expectedSHA256))
	if sum != want {
		_ = os.Remove(dest)
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", filepath.Base(dest), sum, want)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func downloadTemplateEntries(dest string, entries []TemplateMetadata, templatesDest string, engineVersion string, onlyMissing bool) ([]string, error) {
	var paths []string
	for _, e := range entries {
		if e.DownloadURL == "" || e.Filename == "" {
			continue
		}
		if onlyMissing && templateAlreadyInstalled(templatesDest, engineVersion, e.Filename) {
			logMessage("Skipping %s (already installed)", e.Filename)
			continue
		}
		outPath := filepath.Join(dest, e.Filename)
		logMessage("Downloading template %s from %s", e.Filename, e.DownloadURL)
		if err := downloadTemplateFile(e.DownloadURL, outPath, e.Sha256); err != nil {
			return paths, fmt.Errorf("download %s: %w", e.Filename, err)
		}
		paths = append(paths, outPath)
		logMessage("Template file downloaded to %s", outPath)
	}
	return paths, nil
}

// selectRequiredRuntimeTemplates picks web, linux, and windows templates for the active variant.
// Windows requires both normal and console executables.
func selectRequiredRuntimeTemplates(all []TemplateMetadata, variant TemplateVariant) []TemplateMetadata {
	var out []TemplateMetadata

	if m := pickTemplateByFilename(all, webTemplateName(variant)); m != nil {
		out = append(out, *m)
	} else if m := pickPlatformFallback(all, "web", variant); m != nil {
		logMessage("Warning: %s web template not found; using %s", webTemplateName(variant), m.Filename)
		out = append(out, *m)
	}

	if m := pickTemplateByFilename(all, linuxTemplateName(variant)); m != nil {
		out = append(out, *m)
	} else if m := pickPlatformFallback(all, "linux", variant); m != nil {
		logMessage("Warning: %s linux template not found; using %s", linuxTemplateName(variant), m.Filename)
		out = append(out, *m)
	}

	normal := windowsNormalTemplateName(variant)
	console := windowsConsoleTemplateName(variant)
	if m := pickTemplateByFilename(all, normal); m != nil {
		out = append(out, *m)
	} else if m := pickWindowsNormalFallback(all, variant); m != nil {
		logMessage("Warning: %s not found; using %s", normal, m.Filename)
		out = append(out, *m)
	}
	if m := pickTemplateByFilename(all, console); m != nil {
		out = append(out, *m)
	} else if m := pickWindowsConsoleFallback(all, variant); m != nil {
		logMessage("Warning: %s not found; using %s", console, m.Filename)
		out = append(out, *m)
	}

	return out
}

func pickTemplateByFilename(all []TemplateMetadata, filename string) *TemplateMetadata {
	want := strings.ToLower(filename)
	for i := range all {
		if strings.ToLower(all[i].Filename) == want {
			return &all[i]
		}
	}
	return nil
}

func pickPlatformFallback(all []TemplateMetadata, platform string, variant TemplateVariant) *TemplateMetadata {
	for i := range all {
		p := strings.ToLower(all[i].Platform)
		if p != platform {
			continue
		}
		if matchesTemplateVariant(all[i].Filename, variant) {
			return &all[i]
		}
	}
	for i := range all {
		if strings.ToLower(all[i].Platform) == platform {
			return &all[i]
		}
	}
	return nil
}

func pickWindowsNormalFallback(all []TemplateMetadata, variant TemplateVariant) *TemplateMetadata {
	for i := range all {
		if strings.ToLower(all[i].Platform) != "windows" {
			continue
		}
		if !isWindowsNormalTemplate(all[i].Filename) {
			continue
		}
		if matchesTemplateVariant(all[i].Filename, variant) {
			return &all[i]
		}
	}
	for i := range all {
		if strings.ToLower(all[i].Platform) == "windows" && isWindowsNormalTemplate(all[i].Filename) {
			return &all[i]
		}
	}
	return nil
}

func pickWindowsConsoleFallback(all []TemplateMetadata, variant TemplateVariant) *TemplateMetadata {
	for i := range all {
		if strings.ToLower(all[i].Platform) != "windows" {
			continue
		}
		if !isWindowsConsoleTemplate(all[i].Filename) {
			continue
		}
		if matchesTemplateVariant(all[i].Filename, variant) {
			return &all[i]
		}
	}
	for i := range all {
		if strings.ToLower(all[i].Platform) == "windows" && isWindowsConsoleTemplate(all[i].Filename) {
			return &all[i]
		}
	}
	return nil
}

func tryDownloadTPZ(url, dest string) error {
	return downloadFile(url, dest)
}
