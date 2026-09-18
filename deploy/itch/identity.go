package itch

import (
	"os"
	"path/filepath"
	"runtime"
)

// IdentityDir is %APPDATA%\blazium or ~/.config/blazium.
func IdentityDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				base = filepath.Join(home, "AppData", "Roaming")
			}
		}
		return filepath.Join(base, "blazium")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "blazium")
	}
	return filepath.Join(home, ".config", "blazium")
}

// IdentityFile is the itch.io butler credentials path.
func IdentityFile() string {
	return filepath.Join(IdentityDir(), "butler_creds")
}
