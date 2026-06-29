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

// TemplateMetadata matches cdn.blazium.app/{channel}/{version}/templates.json entries.
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

func templatesJSONURL(version string, isNightly bool) string {
	ch := channelName(isNightly)
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/templates.json", ch, version)
}

func cerebroTemplatesURL(deployType, version string) string {
	if base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("BLAZIUM_CEREBRO_URL")), "/"); base != "" {
		return fmt.Sprintf("%s/api/v1/templates/%s/%s", base, deployType, version)
	}
	return fmt.Sprintf("https://blazium.app/api/templates/%s/%s", deployType, version)
}

func loadTemplateMetadata(version string, isNightly bool) ([]TemplateMetadata, error) {
	url := templatesJSONURL(version, isNightly)
	body, err := fetchRemoteJSON(url)
	if err == nil {
		var entries []TemplateMetadata
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, fmt.Errorf("parse templates.json: %w", err)
		}
		return entries, nil
	}
	if !isHTTPNotFound(err) {
		return nil, fmt.Errorf("fetch templates.json from %s: %w", url, err)
	}
	return loadTemplateMetadataFromCerebro(deployTypeName(isNightly), version)
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
	files     []string
	platform  string
	monoOnly  bool
	skipMono  bool
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

// selectRequiredRuntimeTemplates picks one web, linux, and windows template when possible.
func selectRequiredRuntimeTemplates(all []TemplateMetadata) []TemplateMetadata {
	need := map[string]bool{"web": true, "linux": true, "windows": true}
	var out []TemplateMetadata
	for _, m := range all {
		p := strings.ToLower(m.Platform)
		if !need[p] {
			continue
		}
		name := strings.ToLower(m.Filename)
		if strings.Contains(name, "debug") {
			continue
		}
		out = append(out, m)
		need[p] = false
	}
	for p, missing := range need {
		if !missing {
			continue
		}
		for _, m := range all {
			if strings.ToLower(m.Platform) == p {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

func tryDownloadTPZ(url, dest string) error {
	return downloadFile(url, dest)
}
