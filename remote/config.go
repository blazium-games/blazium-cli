package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CLIFile is the persisted blazium-cli configuration document.
type CLIFile struct {
	Remote RemoteConfig `json:"remote"`
}

// RemoteConfig holds remote_control-related CLI preferences.
type RemoteConfig struct {
	EvalDefault string `json:"eval_default"`
}

const (
	LangGDScript = "gdscript"
	LangLuau     = "luau"
)

// NormalizeEvalLang maps aliases to canonical language names.
func NormalizeEvalLang(lang string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "gdscript", "gd":
		return LangGDScript, nil
	case "luau", "lua":
		return LangLuau, nil
	default:
		return "", fmt.Errorf("unsupported eval language %q (use gdscript or luau)", lang)
	}
}

// ConfigPath returns the platform path for cli.json.
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
		return filepath.Join(base, "blazium", "cli.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "blazium", "cli.json"), nil
}

// LoadCLIFile reads cli.json; missing file yields empty defaults.
func LoadCLIFile() (CLIFile, error) {
	path, err := ConfigPath()
	if err != nil {
		return CLIFile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return CLIFile{}, nil
		}
		return CLIFile{}, err
	}
	var cfg CLIFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return CLIFile{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// SaveCLIFile writes cli.json, creating parent directories as needed.
func SaveCLIFile(cfg CLIFile) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// EvalDefault resolves bare `remote eval` language: env → file → gdscript.
func EvalDefault() (string, error) {
	if env := strings.TrimSpace(os.Getenv("BLAZIUM_REMOTE_EVAL_DEFAULT")); env != "" {
		return NormalizeEvalLang(env)
	}
	cfg, err := LoadCLIFile()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.Remote.EvalDefault) == "" {
		return LangGDScript, nil
	}
	return NormalizeEvalLang(cfg.Remote.EvalDefault)
}

// SetEvalDefault persists remote.eval_default in cli.json.
func SetEvalDefault(lang string) (string, error) {
	normalized, err := NormalizeEvalLang(lang)
	if err != nil {
		return "", err
	}
	cfg, err := LoadCLIFile()
	if err != nil {
		return "", err
	}
	cfg.Remote.EvalDefault = normalized
	if err := SaveCLIFile(cfg); err != nil {
		return "", err
	}
	return normalized, nil
}
