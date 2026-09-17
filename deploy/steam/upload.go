package steam

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blazium-games/blazium-cli/deploy/steam/guard"
)

var buildIDRe = regexp.MustCompile(`BuildID\s+(\d+)`)

type UploadInput struct {
	Steamcmd     string
	Username     string
	Password     string
	SharedSecret string
	ConfigVDF    string // base64 optional
	App          AppBuild
	WorkDir      string
	DryRun       bool
}

type UploadResult struct {
	AppVDF  string `json:"app_vdf"`
	BuildID string `json:"build_id,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

func Upload(ctx context.Context, in UploadInput) (*UploadResult, error) {
	if in.App.AppID == "" {
		return nil, fmt.Errorf("steam app_id is required")
	}
	if len(in.App.Depots) == 0 {
		return nil, fmt.Errorf("at least one steam depot is required")
	}
	dir := in.WorkDir
	if dir == "" {
		var err error
		dir, err = os.MkdirTemp("", "blazium-steam-upload-*")
		if err != nil {
			return nil, err
		}
	}
	appVDF, err := WriteVDFs(dir, in.App)
	if err != nil {
		return nil, err
	}
	res := &UploadResult{AppVDF: appVDF, DryRun: in.DryRun}
	if in.DryRun {
		return res, nil
	}
	if in.Steamcmd == "" {
		return nil, fmt.Errorf("steamcmd path is empty; run blazium-cli deploy tools ensure")
	}
	if in.Username == "" || in.Password == "" {
		return nil, fmt.Errorf("steam username and password are required")
	}
	if in.SharedSecret == "" && in.ConfigVDF != "" {
		if err := writeConfigVDF(in.ConfigVDF); err != nil {
			return nil, err
		}
	}
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		args := []string{"+login", in.Username, in.Password}
		if in.SharedSecret != "" {
			code, err := guard.WindowCode(in.SharedSecret)
			if err != nil {
				return nil, err
			}
			args = append(args, code)
		}
		args = append(args, "+run_app_build", appVDF, "+quit")
		cmd := exec.CommandContext(ctx, in.Steamcmd, args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		body := out.String()
		if err == nil {
			if m := buildIDRe.FindStringSubmatch(body); len(m) == 2 {
				res.BuildID = m[1]
			}
			return res, nil
		}
		lastErr = fmt.Errorf("steamcmd attempt %d: %w\n%s", attempt, err, body)
	}
	return res, lastErr
}

func writeConfigVDF(b64 string) error {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return fmt.Errorf("config_vdf base64: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Steam", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.vdf"), raw, 0o600)
}
