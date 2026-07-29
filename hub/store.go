package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// File is the persisted hub.json document (editors + projects registry).
type File struct {
	InstallPath   string    `json:"install_path"`
	DefaultEditor string    `json:"default_editor"`
	Editors       []Editor  `json:"editors"`
	Projects      []Project `json:"projects"`
}

// Editor is a registered Blazium editor install.
type Editor struct {
	Version  string `json:"version"`
	Path     string `json:"path"`
	Dir      string `json:"dir"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Mono     bool   `json:"mono"`
}

// Project is a registered local project.
type Project struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	LastOpened string `json:"last_opened,omitempty"`
}

// ConfigPath returns the platform path for hub.json.
func ConfigPath() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "blazium", "hub.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "blazium", "hub.json"), nil
}

// DefaultInstallPath returns the default editors install root.
func DefaultInstallPath() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "Blazium", "Editors"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "blazium", "editors"), nil
}

// Load reads hub.json; missing file yields defaults.
func Load() (File, error) {
	path, err := ConfigPath()
	if err != nil {
		return File{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			ip, ipErr := DefaultInstallPath()
			if ipErr != nil {
				return File{}, ipErr
			}
			return File{InstallPath: ip, Editors: []Editor{}, Projects: []Project{}}, nil
		}
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Editors == nil {
		f.Editors = []Editor{}
	}
	if f.Projects == nil {
		f.Projects = []Project{}
	}
	if strings.TrimSpace(f.InstallPath) == "" {
		ip, ipErr := DefaultInstallPath()
		if ipErr != nil {
			return File{}, ipErr
		}
		f.InstallPath = ip
	}
	return f, nil
}

// Save writes hub.json, creating parent directories as needed.
func Save(f File) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// EffectiveInstallPath returns the configured or default install path.
func (f File) EffectiveInstallPath() (string, error) {
	if strings.TrimSpace(f.InstallPath) != "" {
		return f.InstallPath, nil
	}
	return DefaultInstallPath()
}

// NowRFC3339 returns the current UTC time for last_opened stamps.
func NowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
