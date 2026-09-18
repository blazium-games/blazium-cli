package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Account is an SDA / steamguard-cli maFile subset.
type Account struct {
	AccountName    string          `json:"account_name"`
	SteamID        json.Number     `json:"steamid"`
	SharedSecret   string          `json:"shared_secret"`
	IdentitySecret string          `json:"identity_secret"`
	Secret1        string          `json:"secret_1"`
	SerialNumber   string          `json:"serial_number"`
	RevocationCode string          `json:"revocation_code"`
	URI            string          `json:"uri"`
	TokenGID       string          `json:"token_gid"`
	DeviceID       string          `json:"device_id"`
	Session        json.RawMessage `json:"Session,omitempty"`
}

// Dir is %APPDATA%\blazium\steamguard or ~/.config/blazium/steamguard.
func Dir() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "blazium", "steamguard"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "blazium", "steamguard"), nil
}

// ImportFile reads a maFile JSON.
func ImportFile(path string) (*Account, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var acc Account
	if err := json.Unmarshal(b, &acc); err != nil {
		return nil, fmt.Errorf("maFile: %w", err)
	}
	if strings.TrimSpace(acc.SharedSecret) == "" {
		return nil, fmt.Errorf("maFile missing shared_secret")
	}
	return &acc, nil
}

// Save writes a maFile-compatible JSON into Dir().
func Save(acc *Account) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := strings.TrimSpace(acc.AccountName)
	if name == "" {
		name = string(acc.SteamID)
	}
	if name == "" {
		name = "account"
	}
	path := filepath.Join(dir, name+".maFile")
	b, err := json.MarshalIndent(acc, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// SharedSecretFromFlags returns secret from --secret, --mafile, env, or saved maFile.
func SharedSecretFromFlags(secret, maFile string) (string, error) {
	if strings.TrimSpace(secret) != "" {
		return strings.TrimSpace(secret), nil
	}
	if strings.TrimSpace(os.Getenv("BLAZIUM_STEAM_SHARED_SECRET")) != "" {
		return os.Getenv("BLAZIUM_STEAM_SHARED_SECRET"), nil
	}
	if maFile != "" {
		acc, err := ImportFile(maFile)
		if err != nil {
			return "", err
		}
		return acc.SharedSecret, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("no shared_secret: set BLAZIUM_STEAM_SHARED_SECRET, --secret, or --mafile")
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".mafile") {
			continue
		}
		acc, err := ImportFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if acc.SharedSecret != "" {
			return acc.SharedSecret, nil
		}
	}
	return "", fmt.Errorf("no shared_secret: set BLAZIUM_STEAM_SHARED_SECRET, --secret, or --mafile")
}
