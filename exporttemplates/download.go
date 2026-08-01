package exporttemplates

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/editorinstall"
)

// DownloadAndInstall downloads selected per-file templates and installs them.
func DownloadAndInstall(entries []Entry, destRoot, engineVersion string, onlyMissing bool) ([]string, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no template entries to download")
	}
	tmpDir, err := os.MkdirTemp("", "blazium-templates-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	installVer := InstallVersion(entries, engineVersion)
	paths, err := downloadEntries(tmpDir, entries, destRoot, installVer, onlyMissing)
	if err != nil {
		return paths, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	if err := editorinstall.InstallTemplateFiles(destRoot, installVer, paths); err != nil {
		return paths, err
	}
	var installed []string
	for _, p := range paths {
		installed = append(installed, filepath.Base(p))
	}
	return installed, nil
}

// InstallTPZ downloads and extracts the full export-templates .tpz bundle.
func InstallTPZ(version string, mono, isNightly bool, destRoot string) (string, error) {
	if destRoot == "" {
		destRoot = editorinstall.DefaultTemplatesDest()
	}
	tplURL := cdn.TemplatesTPZURL(version, mono, isNightly)
	tplPath := filepath.Join(os.TempDir(), filepath.Base(tplURL))
	logf("Downloading templates %s", tplURL)
	if err := cdn.DownloadFile(tplURL, tplPath); err != nil {
		return "", fmt.Errorf("download tpz: %w", err)
	}
	defer os.Remove(tplPath)
	installed, err := editorinstall.InstallTemplatesFromTPZ(tplPath, destRoot)
	if err != nil {
		return "", err
	}
	return installed, nil
}

func downloadEntries(dest string, entries []Entry, templatesDest, engineVersion string, onlyMissing bool) ([]string, error) {
	var paths []string
	for _, e := range entries {
		if e.DownloadURL == "" || e.Filename == "" {
			continue
		}
		if onlyMissing && alreadyInstalled(templatesDest, engineVersion, e.Filename) {
			logf("Skipping %s (already installed)", e.Filename)
			continue
		}
		outPath := filepath.Join(dest, e.Filename)
		logf("Downloading template %s from %s", e.Filename, e.DownloadURL)
		if err := downloadFile(e.DownloadURL, outPath, e.Sha256); err != nil {
			return paths, fmt.Errorf("download %s: %w", e.Filename, err)
		}
		paths = append(paths, outPath)
		logf("Template file downloaded to %s", outPath)
	}
	return paths, nil
}

func alreadyInstalled(templatesDest, engineVersion, filename string) bool {
	if templatesDest == "" {
		return false
	}
	for _, dir := range versionDirs(templatesDest, engineVersion) {
		if _, err := os.Stat(filepath.Join(dir, filename)); err == nil {
			return true
		}
	}
	return false
}

func versionDirs(templatesDest, engineVersion string) []string {
	dirs := []string{filepath.Join(templatesDest, engineVersion)}
	if short := editorinstall.TemplateShortVersion(engineVersion); short != engineVersion {
		dirs = append(dirs, filepath.Join(templatesDest, short))
	}
	return dirs
}

func downloadFile(url, dest, expectedSHA256 string) error {
	if err := cdn.DownloadFile(url, dest); err != nil {
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
