package steam

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/blazium-games/blazium-cli/deploy/steam/guard"
)

type LoginInput struct {
	Steamcmd     string
	Username     string
	Password     string
	SharedSecret string
	ConfigVDF    string
}

// Login runs steamcmd +login +quit with an internal Guard code when shared_secret is set.
func Login(ctx context.Context, in LoginInput) (bool, error) {
	if in.Steamcmd == "" {
		return false, fmt.Errorf("steamcmd path is empty; run blazium-cli deploy tools ensure")
	}
	if in.Username == "" || in.Password == "" {
		return false, fmt.Errorf("steam username and password are required")
	}
	if in.SharedSecret == "" && in.ConfigVDF != "" {
		if err := writeConfigVDF(in.ConfigVDF); err != nil {
			return false, err
		}
	}
	args := []string{"+login", in.Username, in.Password}
	if in.SharedSecret != "" {
		code, err := guard.WindowCode(in.SharedSecret)
		if err != nil {
			return false, err
		}
		args = append(args, code)
	}
	args = append(args, "+quit")
	cmd := exec.CommandContext(ctx, in.Steamcmd, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("steamcmd login: %w\n%s", err, out.String())
	}
	return true, nil
}
