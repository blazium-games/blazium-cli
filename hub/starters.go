package hub

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StartersCatalogURL is the CDN catalog for project starters.
// Export templates stay on the templates commands.
const StartersCatalogURL = "https://cdn.blazium.app/starters/starters.json"

const maxStarterZipBytes = 64 << 20

// Starter is one entry in starters.json.
type Starter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	File        string `json:"file"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	Repo        string `json:"repo"`
	Commit      string `json:"commit"`
	GitHub      string `json:"github"`
	Git         string `json:"git"`
}

// StartersCatalog is the document at starters.json.
type StartersCatalog struct {
	Latest   string    `json:"latest"`
	Starters []Starter `json:"starters"`
}

// LoadStartersCatalog fetches and parses a starters.json document.
func LoadStartersCatalog(url string) (StartersCatalog, error) {
	resp, err := http.Get(url)
	if err != nil {
		return StartersCatalog{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return StartersCatalog{}, fmt.Errorf("starters catalog %s: %s", url, resp.Status)
	}
	var catalog StartersCatalog
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&catalog); err != nil {
		return StartersCatalog{}, fmt.Errorf("starters catalog: %w", err)
	}
	return catalog, nil
}

// FindStarter returns the catalog entry with that name.
func FindStarter(catalog StartersCatalog, name string) (Starter, error) {
	name = strings.TrimSpace(name)
	for _, starter := range catalog.Starters {
		if starter.Name == name {
			return starter, nil
		}
	}
	return Starter{}, fmt.Errorf("starter %q is not in the catalog", name)
}

// DownloadStarter fetches the zip, checks sha256, and unpacks it into dest.
// dest is created when missing. A directory that already has files is refused.
func DownloadStarter(starter Starter, dest string) error {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return fmt.Errorf("destination directory is required")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("destination %s is not empty", dest)
	}
	if strings.TrimSpace(starter.File) == "" || strings.TrimSpace(starter.SHA256) == "" {
		return fmt.Errorf("starter %q is missing file or sha256", starter.Name)
	}

	tmp, err := os.CreateTemp("", "blazium-starter-*.zip")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	resp, err := http.Get(starter.File)
	if err != nil {
		tmp.Close()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmp.Close()
		return fmt.Errorf("download %s: %s", starter.File, resp.Status)
	}
	hasher := sha256.New()
	written, err := io.Copy(tmp, io.TeeReader(io.LimitReader(resp.Body, maxStarterZipBytes+1), hasher))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if written > maxStarterZipBytes {
		return fmt.Errorf("starter zip exceeds %d bytes", maxStarterZipBytes)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sum, strings.TrimSpace(starter.SHA256)) {
		return fmt.Errorf("sha256 mismatch: got %s want %s", sum, starter.SHA256)
	}
	return unpackStarterZip(tmpName, dest)
}

func unpackStarterZip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		if err := unpackStarterEntry(destAbs, file); err != nil {
			return err
		}
	}
	return nil
}

func unpackStarterEntry(destAbs string, file *zip.File) error {
	name := filepath.Clean(file.Name)
	if name == "." {
		return nil
	}
	if filepath.IsAbs(file.Name) || strings.HasPrefix(name, "..") {
		return fmt.Errorf("zip path %q escapes the destination", file.Name)
	}
	target := filepath.Join(destAbs, name)
	rel, err := filepath.Rel(destAbs, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("zip path %q escapes the destination", file.Name)
	}
	info := file.FileInfo()
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("zip entry %q is a symlink", file.Name)
	}
	if info.IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, io.LimitReader(src, maxStarterZipBytes))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
