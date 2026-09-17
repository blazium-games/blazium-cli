package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileNames searched when walking up from cwd.
var FileNames = []string{"blazium-deploy.yml", "blazium-deploy.yaml"}

// Config is the project deploy YAML after expansion.
type Config struct {
	Steam SteamConfig `yaml:"steam"`
	Itch  ItchConfig  `yaml:"itch"`
	Games GamesConfig `yaml:"games"`
	Path  string      `yaml:"-"`
}

type SteamConfig struct {
	AppID          string       `yaml:"app_id"`
	Description    string       `yaml:"description"`
	Username       string       `yaml:"username"`
	Password       string       `yaml:"password"`
	SharedSecret   string       `yaml:"shared_secret"`
	APIKey         string       `yaml:"api_key"`
	SteamID        string       `yaml:"steam_id"`
	ConfigVDF      string       `yaml:"config_vdf"`
	RefreshToken   string       `yaml:"refresh_token"`
	AccountName    string       `yaml:"account_name"`
	PublisherKey   string       `yaml:"publisher_key"`
	Branch         string       `yaml:"branch"`
	BranchPassword string       `yaml:"branch_password"`
	Depots         []SteamDepot `yaml:"depots"`
}

type SteamDepot struct {
	ID   string `yaml:"id"`
	Path string `yaml:"path"`
}

type ItchConfig struct {
	Target    string          `yaml:"target"`
	APIKey    string          `yaml:"api_key"`
	CacheDir  string          `yaml:"cache_dir"`
	SteamSync []SteamSyncItem `yaml:"steam_sync"`
}

type SteamSyncItem struct {
	App    string            `yaml:"app"`
	Target string            `yaml:"target"`
	Branch string            `yaml:"branch"`
	Skip   []string          `yaml:"skip"`
	Map    map[string]string `yaml:"map"`
}

type GamesConfig struct {
	AccessToken string `yaml:"access_token"`
	SecretKey   string `yaml:"secret_key"`
	APIURL      string `yaml:"api_url"`
	UploadURL   string `yaml:"upload_url"`
}

// Find walks from start (or cwd) toward root for a deploy YAML.
func Find(start string) (string, error) {
	dir := start
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		for _, name := range FileNames {
			p := filepath.Join(dir, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no blazium-deploy.yml found (walked from %s)", start)
		}
		dir = parent
	}
}

// LoadFile parses path, expands ${ENV}, then fills empty fields from BLAZIUM_* env.
func LoadFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var node any
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	expanded, err := ExpandAny(node)
	if err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(expanded)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		return nil, err
	}
	cfg.Path = path
	applyEnvDefaults(&cfg)
	return &cfg, nil
}

// LoadOptional returns nil,nil when no file exists.
func LoadOptional(start string) (*Config, error) {
	p, err := Find(start)
	if err != nil {
		return nil, nil
	}
	return LoadFile(p)
}

func applyEnvDefaults(cfg *Config) {
	set := func(dst *string, env string) {
		if strings.TrimSpace(*dst) != "" {
			return
		}
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			*dst = v
		}
	}
	set(&cfg.Steam.Username, "BLAZIUM_STEAM_USERNAME")
	set(&cfg.Steam.Password, "BLAZIUM_STEAM_PASSWORD")
	set(&cfg.Steam.SharedSecret, "BLAZIUM_STEAM_SHARED_SECRET")
	set(&cfg.Steam.APIKey, "BLAZIUM_STEAM_API_KEY")
	set(&cfg.Steam.SteamID, "BLAZIUM_STEAM_ID")
	set(&cfg.Steam.ConfigVDF, "BLAZIUM_STEAM_CONFIG_VDF")
	set(&cfg.Steam.RefreshToken, "BLAZIUM_STEAM_REFRESH_TOKEN")
	set(&cfg.Steam.AccountName, "BLAZIUM_STEAM_ACCOUNT_NAME")
	set(&cfg.Steam.PublisherKey, "BLAZIUM_STEAM_PUBLISHER_KEY")
	set(&cfg.Steam.BranchPassword, "BLAZIUM_STEAM_BRANCH_PASSWORD")
	set(&cfg.Steam.Branch, "BLAZIUM_STEAM_BRANCH")
	if cfg.Steam.Description == "" {
		set(&cfg.Steam.Description, "BLAZIUM_GAME_VERSION")
	}
	set(&cfg.Itch.APIKey, "BLAZIUM_BUTLER_API_KEY")
	set(&cfg.Games.AccessToken, "BLAZIUM_ACCESS_TOKEN")
	set(&cfg.Games.SecretKey, "BLAZIUM_SECRET_KEY")
	set(&cfg.Games.APIURL, "BLAZIUM_API_URL")
	set(&cfg.Games.UploadURL, "BLAZIUM_UPLOAD_URL")
}

// FirstNonEmpty returns the first non-empty string.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ApplyItchEnv copies BLAZIUM_BUTLER_API_KEY onto BUTLER_API_KEY when unset.
func ApplyItchEnv(apiKey string) {
	if os.Getenv("BUTLER_API_KEY") == "" && apiKey != "" {
		_ = os.Setenv("BUTLER_API_KEY", apiKey)
	}
	if os.Getenv("BUTLER_API_KEY") == "" && os.Getenv("BLAZIUM_BUTLER_API_KEY") != "" {
		_ = os.Setenv("BUTLER_API_KEY", os.Getenv("BLAZIUM_BUTLER_API_KEY"))
	}
}

// ApplySteamSyncEnv maps BLAZIUM_STEAM_* onto butler steam-sync names.
func ApplySteamSyncEnv(cfg *Config) {
	pairs := [][2]string{
		{"BUTLER_STEAM_REFRESH_TOKEN", FirstNonEmpty(cfgSteam(cfg).RefreshToken, os.Getenv("BLAZIUM_STEAM_REFRESH_TOKEN"))},
		{"BUTLER_STEAM_ACCOUNT_NAME", FirstNonEmpty(cfgSteam(cfg).AccountName, os.Getenv("BLAZIUM_STEAM_ACCOUNT_NAME"))},
		{"BUTLER_STEAM_PUBLISHER_KEY", FirstNonEmpty(cfgSteam(cfg).PublisherKey, os.Getenv("BLAZIUM_STEAM_PUBLISHER_KEY"))},
		{"BUTLER_STEAM_BRANCH_PASSWORD", FirstNonEmpty(cfgSteam(cfg).BranchPassword, os.Getenv("BLAZIUM_STEAM_BRANCH_PASSWORD"))},
	}
	for _, p := range pairs {
		if os.Getenv(p[0]) == "" && p[1] != "" {
			_ = os.Setenv(p[0], p[1])
		}
	}
}

func cfgSteam(cfg *Config) SteamConfig {
	if cfg == nil {
		return SteamConfig{}
	}
	return cfg.Steam
}

// Redacted returns a copy safe for logs/JSON.
func (c *Config) Redacted() map[string]any {
	if c == nil {
		return map[string]any{}
	}
	return map[string]any{
		"path": c.Path,
		"steam": map[string]any{
			"app_id":        c.Steam.AppID,
			"description":   c.Steam.Description,
			"username":      redactIfSet(c.Steam.Username),
			"password":      redactIfSet(c.Steam.Password),
			"shared_secret": redactIfSet(c.Steam.SharedSecret),
			"api_key":       redactIfSet(c.Steam.APIKey),
			"steam_id":      c.Steam.SteamID,
			"depots":        c.Steam.Depots,
		},
		"itch": map[string]any{
			"target":     c.Itch.Target,
			"api_key":    redactIfSet(c.Itch.APIKey),
			"cache_dir":  c.Itch.CacheDir,
			"steam_sync": c.Itch.SteamSync,
		},
		"games": map[string]any{
			"access_token": redactIfSet(c.Games.AccessToken),
			"secret_key":   redactIfSet(c.Games.SecretKey),
			"api_url":      c.Games.APIURL,
			"upload_url":   c.Games.UploadURL,
		},
	}
}

func redactIfSet(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return "(set)"
}
