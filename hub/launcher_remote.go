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

// LauncherRemotePort is the localhost remote_control port for the games launcher.
// Hub uses 39218. The launcher does not bind Hub's port or hub_remote.json.
const LauncherRemotePort = 39220

// LauncherRemoteFile is launcher_remote.json. It has its own token and the exe path.
type LauncherRemoteFile struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Token       string `json:"token"`
	Executable  string `json:"executable,omitempty"`
	CreatedUnix int64  `json:"created_unix,omitempty"`
}

func LauncherRemoteConfigPath() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "blazium", "launcher_remote.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "blazium", "launcher_remote.json"), nil
}

func LauncherRemoteMachineConfigPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("BLAZIUM_LAUNCHER_REMOTE_MACHINE")); v != "" {
		return v, nil
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("PROGRAMDATA")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "blazium", "launcher_remote.json"), nil
	}
	return "/etc/blazium/launcher_remote.json", nil
}

func normalizeLauncherRemote(f LauncherRemoteFile) (LauncherRemoteFile, error) {
	if strings.TrimSpace(f.Host) == "" {
		f.Host = "127.0.0.1"
	}
	if f.Port <= 0 {
		f.Port = LauncherRemotePort
	}
	if strings.TrimSpace(f.Token) == "" {
		return LauncherRemoteFile{}, fmt.Errorf("launcher_remote.json missing token")
	}
	return f, nil
}

func loadLauncherRemoteAt(path string) (LauncherRemoteFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LauncherRemoteFile{}, err
	}
	var f LauncherRemoteFile
	if err := json.Unmarshal(data, &f); err != nil {
		return LauncherRemoteFile{}, fmt.Errorf("parse launcher_remote.json: %w", err)
	}
	return normalizeLauncherRemote(f)
}

func LoadLauncherRemote() (LauncherRemoteFile, error) {
	userPath, err := LauncherRemoteConfigPath()
	if err != nil {
		return LauncherRemoteFile{}, err
	}
	f, err := loadLauncherRemoteAt(userPath)
	if err == nil {
		return f, nil
	}
	userErr := err
	machinePath, merr := LauncherRemoteMachineConfigPath()
	if merr != nil {
		return LauncherRemoteFile{}, userErr
	}
	f, err = loadLauncherRemoteAt(machinePath)
	if err == nil {
		return f, nil
	}
	return LauncherRemoteFile{}, userErr
}

func SaveLauncherRemoteAt(path string, f LauncherRemoteFile) error {
	if strings.TrimSpace(f.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if strings.TrimSpace(f.Host) == "" {
		f.Host = "127.0.0.1"
	}
	if f.Port <= 0 {
		f.Port = LauncherRemotePort
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

// EnsureLauncherRemoteSecretAt loads or creates the file. A valid token is never rotated.
// executable, when set, is written without changing the token.
func EnsureLauncherRemoteSecretAt(path, executable string) (LauncherRemoteFile, error) {
	if strings.TrimSpace(path) == "" {
		return LauncherRemoteFile{}, fmt.Errorf("path is required")
	}
	f, err := loadLauncherRemoteAt(path)
	if err == nil {
		if executable != "" && f.Executable != executable {
			f.Executable = executable
			if err := SaveLauncherRemoteAt(path, f); err != nil {
				return LauncherRemoteFile{}, err
			}
		}
		return f, nil
	}
	if !os.IsNotExist(err) && !strings.Contains(err.Error(), "missing token") && !strings.Contains(err.Error(), "parse launcher_remote") {
		return LauncherRemoteFile{}, err
	}
	token, err := remote.GenerateToken()
	if err != nil {
		return LauncherRemoteFile{}, err
	}
	f = LauncherRemoteFile{
		Host:        "127.0.0.1",
		Port:        LauncherRemotePort,
		Token:       token,
		Executable:  executable,
		CreatedUnix: time.Now().Unix(),
	}
	if err := SaveLauncherRemoteAt(path, f); err != nil {
		return LauncherRemoteFile{}, err
	}
	return f, nil
}

func EnsureLauncherRemoteSecret(executable string) (LauncherRemoteFile, error) {
	f, err := LoadLauncherRemote()
	if err == nil {
		if executable != "" && f.Executable != executable {
			path, perr := LauncherRemoteConfigPath()
			if perr != nil {
				return LauncherRemoteFile{}, perr
			}
			if _, statErr := os.Stat(path); statErr != nil {
				path, perr = LauncherRemoteMachineConfigPath()
				if perr != nil {
					return LauncherRemoteFile{}, perr
				}
			}
			f.Executable = executable
			if err := SaveLauncherRemoteAt(path, f); err != nil {
				return LauncherRemoteFile{}, err
			}
		}
		return f, nil
	}
	if !os.IsNotExist(err) && !strings.Contains(err.Error(), "missing token") && !strings.Contains(err.Error(), "parse launcher_remote") {
		return LauncherRemoteFile{}, err
	}
	userPath, err := LauncherRemoteConfigPath()
	if err != nil {
		return LauncherRemoteFile{}, err
	}
	return EnsureLauncherRemoteSecretAt(userPath, executable)
}

func launcherClient(f LauncherRemoteFile, timeout time.Duration) *remote.Client {
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

func resolveLauncherExecutable(recorded string) (string, error) {
	var candidates []string
	if recorded != "" {
		candidates = append(candidates, recorded)
	}
	if runtime.GOOS == "windows" {
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			candidates = append(candidates, filepath.Join(pf, "Blazium Games", "BlaziumGames.exe"))
		}
	} else {
		candidates = append(candidates, "/opt/blazium-games/bin/blazium-games")
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("games launcher executable not found")
}

// OpenLauncherURI starts the launcher if needed and forwards the raw URI.
// The CLI does not install, buy, or chat. The launcher routes the URI after login.
func OpenLauncherURI(raw string, wait time.Duration) (map[string]any, error) {
	if wait <= 0 {
		wait = 60 * time.Second
	}
	secret, err := EnsureLauncherRemoteSecret("")
	if err != nil {
		return nil, err
	}
	client := launcherClient(secret, 2*time.Second)
	launched := false
	if _, err := client.Health(); err != nil {
		exe, rerr := resolveLauncherExecutable(secret.Executable)
		if rerr != nil {
			return nil, rerr
		}
		cmd := exec.Command(exe)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("launch games launcher: %w", err)
		}
		go func() { _ = cmd.Wait() }()
		if err := client.WaitForHealth(wait); err != nil {
			return nil, fmt.Errorf("launcher started but remote_control did not become ready: %w", err)
		}
		launched = true
	}
	if _, err := client.Exec("open_uri", map[string]any{"uri": raw}); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":       true,
		"action":   "launcher",
		"uri":      raw,
		"port":     secret.Port,
		"launched": launched,
	}, nil
}
