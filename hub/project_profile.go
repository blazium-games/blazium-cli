package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reSection       = regexp.MustCompile(`(?m)^\[([^\]]+)\]\s*$`)
	reSettingBool   = regexp.MustCompile(`^([a-zA-Z0-9_/]+)\s*=\s*(true|false)\s*$`)
	reSettingInt    = regexp.MustCompile(`^([a-zA-Z0-9_/]+)\s*=\s*(-?\d+)\s*$`)
	reSettingString = regexp.MustCompile(`^([a-zA-Z0-9_/]+)\s*=\s*"([^"]*)"\s*$`)
)

// ProjectProfile summarizes project.godot identity and Blazium integrations.
type ProjectProfile struct {
	Path          string               `json:"path"`
	Name          string               `json:"name,omitempty"`
	Features      []string             `json:"features,omitempty"`
	EditorVersion string               `json:"editor_version,omitempty"`
	JustAMCP      JustAMCPProfile      `json:"justamcp"`
	RemoteControl RemoteControlProfile `json:"remote_control"`
	RawSettings   map[string]string    `json:"-"`
}

// JustAMCPProfile holds detected JustAMCP project settings.
type JustAMCPProfile struct {
	Present             bool `json:"present"`
	ServerEnabled       bool `json:"server_enabled"`
	ServerPort          int  `json:"server_port,omitempty"`
	OAuthEnabled        bool `json:"oauth_enabled"`
	HasClientID         bool `json:"has_client_id"`
	GameControlEnabled  bool `json:"game_control_enabled"`
	BindToLocalhostOnly bool `json:"bind_to_localhost_only"`
}

// RemoteControlProfile holds detected remote_control project settings.
type RemoteControlProfile struct {
	Present       bool   `json:"present"`
	ServerEnabled bool   `json:"server_enabled"`
	ServerPort    int    `json:"server_port,omitempty"`
	AllowEval     bool   `json:"allow_eval"`
	AllowRuntime  bool   `json:"allow_runtime"`
	BindAddress   string `json:"bind_address,omitempty"`
	HasToken      bool   `json:"has_token"`
}

// LoadProjectProfile parses project.godot at projectPath.
func LoadProjectProfile(projectPath string) (ProjectProfile, error) {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return ProjectProfile{}, err
	}
	if err := ValidateProjectDir(abs); err != nil {
		return ProjectProfile{}, err
	}
	data, err := os.ReadFile(filepath.Join(abs, "project.godot"))
	if err != nil {
		return ProjectProfile{}, fmt.Errorf("read project.godot: %w", err)
	}
	text := string(data)
	raw := parseSectionSettings(text)

	p := ProjectProfile{
		Path:        abs,
		Name:        firstString(raw, "application/config/name", "config/name"),
		Features:    parseFeatures(text),
		RawSettings: raw,
	}
	if v := firstString(raw, "blazium/editor_version"); v != "" {
		p.EditorVersion = v
	} else if m := reEditorVersion.FindStringSubmatch(text); len(m) == 2 {
		p.EditorVersion = strings.TrimSpace(m[1])
	}

	p.JustAMCP = JustAMCPProfile{
		Present:             hasPrefixKeys(raw, "blazium/justamcp/"),
		ServerEnabled:       boolSetting(raw, "blazium/justamcp/server_enabled", false),
		ServerPort:          intSetting(raw, "blazium/justamcp/server_port", 6506),
		OAuthEnabled:        boolSetting(raw, "blazium/justamcp/oauth_enabled", false),
		HasClientID:         strings.TrimSpace(raw["blazium/justamcp/client_id"]) != "",
		GameControlEnabled:  boolSetting(raw, "blazium/justamcp/game_control_enabled", false),
		BindToLocalhostOnly: boolSetting(raw, "blazium/justamcp/bind_to_localhost_only", true),
	}
	p.RemoteControl = RemoteControlProfile{
		Present:       hasPrefixKeys(raw, "blazium/remote_control/"),
		ServerEnabled: boolSetting(raw, "blazium/remote_control/server_enabled", false),
		ServerPort:    intSetting(raw, "blazium/remote_control/server_port", 6507),
		AllowEval:     boolSetting(raw, "blazium/remote_control/allow_eval", false),
		AllowRuntime:  boolSetting(raw, "blazium/remote_control/allow_runtime", false),
		BindAddress:   stringSetting(raw, "blazium/remote_control/bind_address", "127.0.0.1"),
		HasToken:      strings.TrimSpace(raw["blazium/remote_control/token"]) != "",
	}
	return p, nil
}

// parseSectionSettings expands section-relative keys to fully-qualified ProjectSettings paths.
func parseSectionSettings(text string) map[string]string {
	out := map[string]string{}
	section := ""
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, ";") {
			continue
		}
		if m := reSection.FindStringSubmatch(trim); len(m) == 2 {
			section = m[1]
			continue
		}
		var key, val string
		if m := reSettingString.FindStringSubmatch(trim); len(m) == 3 {
			key, val = m[1], m[2]
		} else if m := reSettingBool.FindStringSubmatch(trim); len(m) == 3 {
			key, val = m[1], m[2]
		} else if m := reSettingInt.FindStringSubmatch(trim); len(m) == 3 {
			key, val = m[1], m[2]
		} else {
			continue
		}
		full := key
		if section != "" && !strings.Contains(key, "/") {
			full = section + "/" + key
		} else if section != "" && !strings.HasPrefix(key, section+"/") {
			// Keys like justamcp/server_enabled under [blazium] → blazium/justamcp/server_enabled
			if section == "blazium" || !strings.HasPrefix(key, "blazium/") {
				full = section + "/" + key
			}
		}
		out[full] = val
		// Also keep the raw key for convenience.
		out[key] = val
	}
	return out
}

func firstString(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(m[k]); v != "" {
			return v
		}
	}
	return ""
}

func hasPrefixKeys(m map[string]string, prefix string) bool {
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func boolSetting(m map[string]string, key string, def bool) bool {
	v, ok := m[key]
	if !ok {
		return def
	}
	return strings.EqualFold(v, "true")
}

func intSetting(m map[string]string, key string, def int) int {
	v, ok := m[key]
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func stringSetting(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}
