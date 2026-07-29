package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CLIFile is the persisted blazium-cli configuration document.
type CLIFile struct {
	Remote RemoteConfig `json:"remote"`
}

// RemoteConfig holds remote_control-related CLI preferences and instance registry.
type RemoteConfig struct {
	EvalDefault     string           `json:"eval_default,omitempty"`
	EnableOnOpen    *bool            `json:"enable_on_open,omitempty"`     // nil => true
	EnableMCPOnLoad *bool            `json:"enable_mcp_on_load,omitempty"` // nil => true when project has justamcp keys
	Instances       []RemoteInstance `json:"instances,omitempty"`
	Closed          []ClosedInstance `json:"closed,omitempty"`
}

// RemoteInstance is an active editor with remote_control endpoints.
type RemoteInstance struct {
	ID            string    `json:"id"`
	ProjectPath   string    `json:"project_path"`
	ProjectName   string    `json:"project_name,omitempty"`
	Host          string    `json:"host"`
	RemotePort    int       `json:"remote_port"`
	RemoteToken   string    `json:"remote_token"`
	MCPPort       int       `json:"mcp_port,omitempty"`
	MCPEnabled    bool      `json:"mcp_enabled"`
	PID           int       `json:"pid"`
	Bound         bool      `json:"bound"`
	EditorPath    string    `json:"editor_path,omitempty"`
	EditorVersion string    `json:"editor_version,omitempty"`
	StartedAt     time.Time `json:"started_at"`
}

// ClosedInstance is a retired instance history record (not matchable).
type ClosedInstance struct {
	ProjectPath string    `json:"project_path"`
	PID         int       `json:"pid"`
	RemotePort  int       `json:"remote_port,omitempty"`
	RetiredID   string    `json:"retired_id,omitempty"`
	Reason      string    `json:"reason"`
	EndedAt     time.Time `json:"ended_at"`
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

// EnableOnOpen reports whether open/load should enable remote_control (default true).
func EnableOnOpen(cfg CLIFile) bool {
	if cfg.Remote.EnableOnOpen == nil {
		return true
	}
	return *cfg.Remote.EnableOnOpen
}

// SetEnableOnOpen persists remote.enable_on_open.
func SetEnableOnOpen(enabled bool) error {
	cfg, err := LoadCLIFile()
	if err != nil {
		return err
	}
	cfg.Remote.EnableOnOpen = &enabled
	return SaveCLIFile(cfg)
}

// EnableMCPOnLoad reports whether load should enable JustAMCP when the project has justamcp keys.
// Default true when unset.
func EnableMCPOnLoad(cfg CLIFile) bool {
	if cfg.Remote.EnableMCPOnLoad == nil {
		return true
	}
	return *cfg.Remote.EnableMCPOnLoad
}

// SetEnableMCPOnLoad persists remote.enable_mcp_on_load.
func SetEnableMCPOnLoad(enabled bool) error {
	cfg, err := LoadCLIFile()
	if err != nil {
		return err
	}
	cfg.Remote.EnableMCPOnLoad = &enabled
	return SaveCLIFile(cfg)
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

// PruneDeadInstances retires dead/unresponsive instances and saves if changed.
// healthCheck, when non-nil, is called for PID-alive instances; false means unresponsive.
func PruneDeadInstances(healthCheck func(RemoteInstance) bool) (CLIFile, int, error) {
	cfg, err := LoadCLIFile()
	if err != nil {
		return cfg, 0, err
	}
	retired := 0
	var closed []ClosedInstance
	alive := make([]RemoteInstance, 0, len(cfg.Remote.Instances))
	for _, inst := range cfg.Remote.Instances {
		reason := ""
		if !IsProcessAlive(inst.PID) {
			reason = "exited"
		} else if healthCheck != nil && !healthCheck(inst) {
			reason = "crashed_or_unresponsive"
		}
		if reason != "" {
			closed = append(closed, ClosedInstance{
				ProjectPath: inst.ProjectPath,
				PID:         inst.PID,
				RemotePort:  inst.RemotePort,
				RetiredID:   inst.ID,
				Reason:      reason,
				EndedAt:     time.Now().UTC(),
			})
			retired++
			continue
		}
		alive = append(alive, inst)
	}
	if retired == 0 {
		return cfg, 0, nil
	}
	cfg.Remote.Instances = alive
	cfg.Remote.Closed = append(closed, cfg.Remote.Closed...)
	if len(cfg.Remote.Closed) > maxClosedHistory {
		cfg.Remote.Closed = cfg.Remote.Closed[:maxClosedHistory]
	}
	if err := SaveCLIFile(cfg); err != nil {
		return cfg, retired, err
	}
	return cfg, retired, nil
}
