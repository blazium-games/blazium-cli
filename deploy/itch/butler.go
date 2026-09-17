package itch

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/blazium-games/blazium-cli/deploy/config"
	"github.com/itchio/butler/cmd/login"
	"github.com/itchio/butler/cmd/push"
	"github.com/itchio/butler/comm"
	"github.com/itchio/butler/mansion"
	butlersteam "github.com/itchio/butler/steam"
	itchio "github.com/itchio/go-itchio"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

func butlerContext(jsonOut bool) *mansion.Context {
	app := kingpin.New("blazium-cli", "itch.io deploy via butler libraries")
	ctx := mansion.NewContext(app)
	ctx.Identity = IdentityFile()
	ctx.ContextTimeout = 300
	ctx.JSON = jsonOut
	ctx.SetAddress("https://itch.io")
	comm.Configure(false, false, false, jsonOut, false, true, false)
	return ctx
}

// Login runs butler's itch.io OAuth / saved-credentials flow.
func Login(jsonOut bool) error {
	return login.Do(butlerContext(jsonOut))
}

// Logout deletes the local itch.io identity file without prompting.
func Logout() error {
	path := IdentityFile()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type PushInput struct {
	Src         string
	Target      string
	UserVersion string
	DryRun      bool
	IfChanged   bool
	Hidden      bool
	JSON        bool
}

// Push uploads a directory to itch.io in-process.
func Push(in PushInput) error {
	if strings.TrimSpace(in.Src) == "" {
		return fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(in.Target) == "" {
		return fmt.Errorf("itch target is required (user/game[:channel])")
	}
	ctx := butlerContext(in.JSON)
	if in.DryRun {
		// push.Do reads package-level args.dryRun; call with a walk-only path
		// by invoking Do after setting the unexported flag via a dry-run wrapper.
		return pushDryRun(ctx, in)
	}
	return push.Do(ctx, in.Src, in.Target, in.UserVersion, true, false, in.IfChanged, true, true, in.Hidden, nil)
}

func pushDryRun(ctx *mansion.Context, in PushInput) error {
	// butler's dry-run is gated on unexported args; listing via Do with a
	// dummy client is unsafe. Walk the source and report the plan instead.
	st, err := os.Stat(in.Src)
	if err != nil {
		return err
	}
	if !st.IsDir() && !st.Mode().IsRegular() {
		return fmt.Errorf("src is not a file or directory: %s", in.Src)
	}
	comm.Opf("Dry run: would push %s to %s", in.Src, in.Target)
	comm.Result(map[string]interface{}{
		"buildId": 0,
		"target":  in.Target,
		"src":     in.Src,
		"dryRun":  true,
	})
	_ = ctx
	return nil
}

type SteamSyncInput struct {
	AppID    uint32
	Target   string
	Branch   string
	CacheDir string
	Force    bool
	DryRun   bool
	NoPush   bool
	Hidden   bool
	JSON     bool
	Entries  []butlersteam.SyncEntry
}

// SteamSync copies Steam depots to itch.io via butler/fresh-steamer.
func SteamSync(goCtx context.Context, in SteamSyncInput) error {
	ctx := butlerContext(in.JSON)
	store := butlersteam.StoreFor(ctx.Identity)
	entries := in.Entries
	if len(entries) == 0 {
		if in.AppID == 0 || strings.TrimSpace(in.Target) == "" {
			return fmt.Errorf("give an app id and target, or steam_sync entries in blazium-deploy.yml")
		}
		e := butlersteam.SyncEntry{App: in.AppID, Target: in.Target, Branch: in.Branch, CacheDir: in.CacheDir, Hidden: in.Hidden}
		if err := e.Validate(); err != nil {
			return err
		}
		entries = []butlersteam.SyncEntry{e}
	}
	password := os.Getenv("BUTLER_STEAM_BRANCH_PASSWORD")
	if password == "" {
		password = os.Getenv("BLAZIUM_STEAM_BRANCH_PASSWORD")
	}
	for _, e := range entries {
		if in.CacheDir != "" && e.CacheDir == "" {
			e.CacheDir = in.CacheDir
		}
		if in.Hidden {
			e.Hidden = true
		}
		planOpts, err := e.PlanOptions(password)
		if err != nil {
			return err
		}
		if in.DryRun {
			plan, err := butlersteam.Plan(goCtx, store, planOpts)
			if err != nil {
				return err
			}
			comm.Result(plan)
			continue
		}
		opts := butlersteam.SyncOptions{
			PlanOptions: planOpts,
			CacheDir:    e.CacheDir,
			Force:       in.Force,
			Hidden:      e.Hidden,
			Logf:        comm.Debugf,
		}
		if in.NoPush && e.CacheDir == "" {
			return fmt.Errorf("--no-push needs a cache directory")
		}
		if !in.NoPush {
			client, err := ctx.AuthenticateViaOauth()
			if err != nil {
				return fmt.Errorf("authenticating with itch.io: %w", err)
			}
			opts.Client = client
			opts.Push = func(goCtx context.Context, dir, target, userVersion string, hidden bool, metadata itchio.BuildMetadata) error {
				return push.Do(ctx, dir, target, userVersion, true, false, false, true, false, hidden, metadata)
			}
		}
		result, err := butlersteam.Sync(goCtx, store, opts)
		if err != nil {
			return err
		}
		if result != nil && result.Plan != nil {
			for _, c := range result.Channels {
				if files := butlersteam.SteamworksFiles(c.Dir); len(files) > 0 {
					comm.Notice("Steamworks SDK detected", append([]string{
						fmt.Sprintf("The %s build ships the Steamworks SDK.", c.Name),
						"If the game initializes Steam at startup it may not run for itch.io players.",
					}, files...))
				}
			}
			comm.Object("steamSyncDone", comm.JsonMessage{
				"appId":    result.Plan.AppID,
				"buildId":  result.Plan.BuildID,
				"target":   result.Plan.Target,
				"channels": result.Channels,
			})
		}
	}
	return nil
}

// SteamLoginPassword persists a Steam refresh token next to butler identity.
func SteamLoginPassword(goCtx context.Context, account, password string, persist bool) error {
	store := butlersteam.StoreFor(IdentityFile())
	_, err := butlersteam.LoginPassword(goCtx, store, account, password, nil, butlersteam.LoginOptions{Persist: persist})
	return err
}

// SteamLogout deletes local Steam-sync credentials (full account token).
func SteamLogout() error {
	return butlersteam.StoreFor(IdentityFile()).Logout()
}

// ParseAppID converts a decimal app id string.
func ParseAppID(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty app id")
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("app id: %w", err)
	}
	return uint32(n), nil
}

// ApplyConfigEnv maps BLAZIUM_* onto butler env names.
func ApplyConfigEnv(cfg *config.Config) {
	apiKey := ""
	if cfg != nil {
		apiKey = cfg.Itch.APIKey
	}
	config.ApplyItchEnv(apiKey)
	config.ApplySteamSyncEnv(cfg)
}
