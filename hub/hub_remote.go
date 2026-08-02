package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/remote"
)

// HubRemotePort is the fixed localhost port for Hub remote_control.
const HubRemotePort = 39218

// HubRemoteFile is the shared CLI↔Hub secret file (hub_remote.json).
type HubRemoteFile struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Token       string `json:"token"`
	CreatedUnix int64  `json:"created_unix,omitempty"`
}

// HubRemoteConfigPath returns the per-user path for hub_remote.json.
func HubRemoteConfigPath() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "blazium", "hub_remote.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "blazium", "hub_remote.json"), nil
}

// HubRemoteMachineConfigPath returns the machine-wide hub_remote.json path.
// BLAZIUM_HUB_REMOTE_MACHINE overrides the path (tests / advanced installers).
func HubRemoteMachineConfigPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("BLAZIUM_HUB_REMOTE_MACHINE")); v != "" {
		return v, nil
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("PROGRAMDATA")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "blazium", "hub_remote.json"), nil
	}
	return "/etc/blazium/hub_remote.json", nil
}

func normalizeHubRemote(f HubRemoteFile) (HubRemoteFile, error) {
	if strings.TrimSpace(f.Host) == "" {
		f.Host = "127.0.0.1"
	}
	if f.Port <= 0 {
		f.Port = HubRemotePort
	}
	if strings.TrimSpace(f.Token) == "" {
		return HubRemoteFile{}, fmt.Errorf("hub_remote.json missing token")
	}
	return f, nil
}

func loadHubRemoteAt(path string) (HubRemoteFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HubRemoteFile{}, err
	}
	var f HubRemoteFile
	if err := json.Unmarshal(data, &f); err != nil {
		return HubRemoteFile{}, fmt.Errorf("parse hub_remote.json: %w", err)
	}
	return normalizeHubRemote(f)
}

// LoadHubRemote reads hub_remote.json from user path, then machine path.
func LoadHubRemote() (HubRemoteFile, error) {
	userPath, err := HubRemoteConfigPath()
	if err != nil {
		return HubRemoteFile{}, err
	}
	f, err := loadHubRemoteAt(userPath)
	if err == nil {
		return f, nil
	}
	userErr := err
	machinePath, merr := HubRemoteMachineConfigPath()
	if merr != nil {
		return HubRemoteFile{}, userErr
	}
	f, err = loadHubRemoteAt(machinePath)
	if err == nil {
		return f, nil
	}
	if os.IsNotExist(userErr) {
		return HubRemoteFile{}, userErr
	}
	return HubRemoteFile{}, userErr
}

// SaveHubRemote writes hub_remote.json to the user path.
func SaveHubRemote(f HubRemoteFile) error {
	path, err := HubRemoteConfigPath()
	if err != nil {
		return err
	}
	return SaveHubRemoteAt(path, f)
}

// SaveHubRemoteAt writes hub_remote.json to an explicit path.
func SaveHubRemoteAt(path string, f HubRemoteFile) error {
	if strings.TrimSpace(f.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if strings.TrimSpace(f.Host) == "" {
		f.Host = "127.0.0.1"
	}
	if f.Port <= 0 {
		f.Port = HubRemotePort
	}
	if f.CreatedUnix == 0 {
		f.CreatedUnix = time.Now().Unix()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "\t")
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	return os.WriteFile(path, data, mode)
}

// EnsureHubRemoteSecretAt loads or creates hub_remote.json at path (never rotates a valid token).
func EnsureHubRemoteSecretAt(path string) (HubRemoteFile, error) {
	if strings.TrimSpace(path) == "" {
		return HubRemoteFile{}, fmt.Errorf("path is required")
	}
	f, err := loadHubRemoteAt(path)
	if err == nil {
		return f, nil
	}
	if !os.IsNotExist(err) && !strings.Contains(err.Error(), "missing token") && !strings.Contains(err.Error(), "parse hub_remote") {
		return HubRemoteFile{}, err
	}
	token, err := remote.GenerateToken()
	if err != nil {
		return HubRemoteFile{}, err
	}
	f = HubRemoteFile{
		Host:        "127.0.0.1",
		Port:        HubRemotePort,
		Token:       token,
		CreatedUnix: time.Now().Unix(),
	}
	if err := SaveHubRemoteAt(path, f); err != nil {
		return HubRemoteFile{}, err
	}
	return f, nil
}

// EnsureHubRemoteSecret loads user/machine secret, or creates a new user file.
func EnsureHubRemoteSecret() (HubRemoteFile, error) {
	f, err := LoadHubRemote()
	if err == nil {
		return f, nil
	}
	if !os.IsNotExist(err) {
		if !strings.Contains(err.Error(), "missing token") && !strings.Contains(err.Error(), "parse hub_remote") {
			return HubRemoteFile{}, err
		}
	}
	userPath, err := HubRemoteConfigPath()
	if err != nil {
		return HubRemoteFile{}, err
	}
	return EnsureHubRemoteSecretAt(userPath)
}

// HubClient builds an authenticated remote_control client for Hub.
func HubClient(f HubRemoteFile, timeout time.Duration) *remote.Client {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return remote.NewClient(remote.Config{
		Host:    f.Host,
		Port:    f.Port,
		Token:   f.Token,
		Timeout: timeout,
	})
}

// ResolveHubExecutable finds BlaziumHub / blazium-hub on disk.
func ResolveHubExecutable() (string, error) {
	var candidates []string
	if runtime.GOOS == "windows" {
		if root := strings.TrimSpace(os.Getenv("BLAZIUM")); root != "" {
			candidates = append(candidates,
				filepath.Join(root, "Hub", "BlaziumHub.exe"),
				filepath.Join(root, "BlaziumHub.exe"),
			)
		}
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			candidates = append(candidates, filepath.Join(pf, "Blazium", "Hub", "BlaziumHub.exe"))
		}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			candidates = append(candidates, filepath.Join(local, "Blazium", "Hub", "BlaziumHub.exe"))
		}
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			candidates = append(candidates,
				filepath.Join(dir, "Hub", "BlaziumHub.exe"),
				filepath.Join(dir, "BlaziumHub.exe"),
			)
		}
	} else {
		if root := strings.TrimSpace(os.Getenv("BLAZIUM")); root != "" {
			candidates = append(candidates,
				filepath.Join(root, "bin", "blazium-hub"),
				filepath.Join(root, "blazium-hub"),
			)
		}
		candidates = append(candidates,
			"/opt/blazium/bin/blazium-hub",
			"/usr/local/bin/blazium-hub",
			"/usr/bin/blazium-hub",
		)
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			candidates = append(candidates, filepath.Join(dir, "blazium-hub"))
		}
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("Blazium Hub executable not found (set BLAZIUM or install Hub)")
}

// EnsureHubResult describes EnsureHub outcome.
type EnsureHubResult struct {
	Client   *remote.Client
	Remote   HubRemoteFile
	Launched bool
}

// EnsureHub returns a healthy authenticated Hub remote_control client,
// launching Hub when necessary and waiting until /v1/health succeeds.
func EnsureHub(wait time.Duration) (EnsureHubResult, error) {
	if wait <= 0 {
		wait = 60 * time.Second
	}
	secret, err := EnsureHubRemoteSecret()
	if err != nil {
		return EnsureHubResult{}, err
	}
	client := HubClient(secret, 2*time.Second)
	if _, err := client.Health(); err == nil {
		return EnsureHubResult{Client: client, Remote: secret, Launched: false}, nil
	}

	hubExe, err := ResolveHubExecutable()
	if err != nil {
		return EnsureHubResult{}, err
	}
	args := []string{
		"--enable-remote-control",
		fmt.Sprintf("--remote-control-port=%d", secret.Port),
		fmt.Sprintf("--remote-control-token=%s", secret.Token),
	}
	cmd := exec.Command(hubExe, args...)
	if err := cmd.Start(); err != nil {
		return EnsureHubResult{}, fmt.Errorf("launch Hub: %w", err)
	}
	go func() { _ = cmd.Wait() }()

	if err := client.WaitForHealth(wait); err != nil {
		return EnsureHubResult{}, fmt.Errorf("Hub launched but remote_control did not become ready: %w", err)
	}
	return EnsureHubResult{Client: client, Remote: secret, Launched: true}, nil
}

// FocusHub ensures Hub is up and brings it to the front via remote_control.
func FocusHub(wait time.Duration) (map[string]any, error) {
	res, err := EnsureHub(wait)
	if err != nil {
		return nil, err
	}
	focused := false
	for _, cmdName := range []string{"show_hub", "focus_window", "bring_to_front", "focus"} {
		if _, err := res.Client.Exec(cmdName, nil); err == nil {
			focused = true
			break
		}
	}
	return map[string]any{
		"ok":       true,
		"action":   "hub",
		"focused":  focused,
		"launched": res.Launched,
		"host":     res.Remote.Host,
		"port":     res.Remote.Port,
	}, nil
}
