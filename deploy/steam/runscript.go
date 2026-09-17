package steam

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func steamcmdQuote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func redactSteamcmdLog(s string, secrets ...string) string {
	out := s
	for _, sec := range secrets {
		if strings.TrimSpace(sec) == "" {
			continue
		}
		out = strings.ReplaceAll(out, sec, "***")
	}
	return out
}

func runSteamcmdScript(ctx context.Context, steamcmd string, secrets []string, lines ...string) (string, error) {
	dir, err := os.MkdirTemp("", "blazium-steamcmd-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "script.txt")
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, steamcmd, "+runscript", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	log := redactSteamcmdLog(out.String(), secrets...)
	if err != nil {
		return log, fmt.Errorf("steamcmd: %w\n%s", err, log)
	}
	return log, nil
}

func loginScriptLines(username, password, totp string, extra ...string) []string {
	login := "login " + steamcmdQuote(username) + " " + steamcmdQuote(password)
	if totp != "" {
		login += " " + steamcmdQuote(totp)
	}
	lines := []string{
		"@ShutdownOnFailedCommand 1",
		"@NoPromptForPassword 1",
		login,
	}
	lines = append(lines, extra...)
	lines = append(lines, "quit")
	return lines
}
