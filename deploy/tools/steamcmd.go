package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/update"
)

const envSteamcmd = "BLAZIUM_STEAMCMD"

// Status describes steamcmd discovery.
type Status struct {
	Steamcmd  string `json:"steamcmd"`
	Toolchain string `json:"toolchain"`
	Source    string `json:"source"`
	Ready     bool   `json:"ready"`
	Error     string `json:"error,omitempty"`
}

// LocateSteamcmd finds steamcmd without installing.
func LocateSteamcmd() Status {
	st := Status{}
	if p := strings.TrimSpace(os.Getenv(envSteamcmd)); p != "" {
		if fileExists(p) {
			st.Steamcmd = p
			st.Source = envSteamcmd
			st.Ready = true
			return st
		}
		st.Error = envSteamcmd + " is set but not a file: " + p
	}
	if tc := toolchainBin(); tc != "" {
		st.Toolchain = tc
		if p, err := steamcmdFromToolchain(tc, false); err == nil && p != "" {
			st.Steamcmd = p
			st.Source = "toolchain"
			st.Ready = true
			return st
		}
	}
	if p, err := exec.LookPath(steamcmdName()); err == nil {
		st.Steamcmd = p
		st.Source = "PATH"
		st.Ready = true
		return st
	}
	if st.Error == "" {
		st.Error = "steamcmd not found; run blazium-cli deploy tools ensure"
	}
	return st
}

// EnsureSteamcmd locates steamcmd, running `blazium-toolchain steam setup` if needed.
func EnsureSteamcmd(ctx context.Context) (Status, error) {
	st := LocateSteamcmd()
	if st.Ready {
		return st, nil
	}
	tc := toolchainBin()
	if tc == "" {
		return st, fmt.Errorf("blazium-toolchain not found (install with blazium-cli update apply --product toolchain)")
	}
	st.Toolchain = tc
	if err := runToolchain(ctx, tc, "steam", "setup"); err != nil {
		st.Error = err.Error()
		return st, err
	}
	p, err := steamcmdFromToolchain(tc, true)
	if err != nil {
		st.Error = err.Error()
		return st, err
	}
	st.Steamcmd = p
	st.Source = "toolchain"
	st.Ready = p != ""
	if !st.Ready {
		return st, fmt.Errorf("steamcmd still missing after toolchain steam setup")
	}
	return st, nil
}

func steamcmdFromToolchain(tc string, afterSetup bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tc, "--json", "steam", "env")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if afterSetup {
			return "", fmt.Errorf("blazium-toolchain steam env: %w (%s)", err, strings.TrimSpace(errb.String()))
		}
		return "", err
	}
	var env map[string]string
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		return "", err
	}
	return strings.TrimSpace(env["STEAMCMD"]), nil
}

func runToolchain(ctx context.Context, tc string, args ...string) error {
	cmd := exec.CommandContext(ctx, tc, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func toolchainBin() string {
	dest := update.ToolchainDest("")
	if fileExists(dest) {
		return dest
	}
	if p, err := exec.LookPath("blazium-toolchain"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("blazium-toolchain.exe"); err == nil {
			return p
		}
	}
	return ""
}

func steamcmdName() string {
	if runtime.GOOS == "windows" {
		return "steamcmd.exe"
	}
	return "steamcmd"
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// PrefixHint is unused helper for tests.
func PrefixHint() string {
	return filepath.Dir(update.ToolchainDest(""))
}
