package deploy

import (
	"fmt"
	"os"
	"strings"

	"github.com/blazium-games/blazium-cli/deploy/config"
	"github.com/blazium-games/blazium-cli/deploy/itch"
	"github.com/blazium-games/blazium-cli/deploy/steam"
	"github.com/blazium-games/blazium-cli/deploy/steam/guard"
	"github.com/blazium-games/blazium-cli/deploy/tools"
	"github.com/blazium-games/blazium-cli/output"
	"github.com/spf13/cobra"
)

// Options configures deploy Cobra commands.
type Options struct {
	Format *string
}

func formatOf(opts Options) string {
	if opts.Format != nil {
		return *opts.Format
	}
	return "human"
}

func jsonOut(opts Options) bool {
	return strings.EqualFold(formatOf(opts), "json")
}

func loadConfig(path string) (*config.Config, error) {
	if strings.TrimSpace(path) != "" {
		return config.LoadFile(path)
	}
	return config.LoadOptional("")
}

// NewCommand returns `blazium-cli deploy`.
func NewCommand(opts Options) *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Steam, itch.io, and tool setup for store deploys",
		Long: `Store deploy commands. Secrets stay in BLAZIUM_* env or ${NAME} references in blazium-deploy.yml.

steam-sync login tokens are full Steam account access; use deploy itch steam-logout to delete local creds.
Builder password/shared_secret are never written unless you opt into a local maFile.
CI should set BLAZIUM_STEAM_* / BLAZIUM_BUTLER_API_KEY rather than running guard setup on the runner.`,
		SilenceUsage: true,
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to blazium-deploy.yml (default: walk up from cwd)")

	cmd.AddCommand(toolsCommands(opts))
	cmd.AddCommand(itchCommands(opts, &configPath))
	cmd.AddCommand(steamCommands(opts, &configPath))
	return cmd
}

func toolsCommands(opts Options) *cobra.Command {
	toolsCmd := &cobra.Command{Use: "tools", Short: "steamcmd via blazium-toolchain"}
	toolsCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show steamcmd path from env, toolchain cache, or PATH",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := tools.LocateSteamcmd()
			return output.Write(formatOf(opts), map[string]any{
				"steamcmd":  st.Steamcmd,
				"toolchain": st.Toolchain,
				"source":    st.Source,
				"ready":     st.Ready,
				"error":     st.Error,
			})
		},
	})
	toolsCmd.AddCommand(&cobra.Command{
		Use:   "ensure",
		Short: "Install steamcmd with blazium-toolchain steam setup if missing",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := tools.EnsureSteamcmd(cmd.Context())
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{
				"steamcmd":  st.Steamcmd,
				"toolchain": st.Toolchain,
				"source":    st.Source,
				"ready":     st.Ready,
			})
		},
	})
	return toolsCmd
}

func itchCommands(opts Options, configPath *string) *cobra.Command {
	itchCmd := &cobra.Command{Use: "itch", Short: "itch.io push and Steam→itch steam-sync"}
	itchCmd.AddCommand(&cobra.Command{
		Use:   "login",
		Short: "Connect to itch.io (BUTLER_API_KEY / BLAZIUM_BUTLER_API_KEY or OAuth)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig(*configPath)
			itch.ApplyConfigEnv(cfg)
			if cfg != nil {
				config.ApplyItchEnv(cfg.Itch.APIKey)
			} else {
				config.ApplyItchEnv("")
			}
			if err := itch.Login(jsonOut(opts)); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok", "identity": itch.IdentityFile()})
		},
	})
	itchCmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Delete local itch.io butler credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := itch.Logout(); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok"})
		},
	})
	var pushTarget, pushVersion string
	var pushDry, pushIfChanged, pushHidden bool
	pushCmd := &cobra.Command{
		Use:   "push <dir>",
		Short: "Upload a directory to itch.io (in-process butler)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*configPath)
			if err != nil {
				return err
			}
			itch.ApplyConfigEnv(cfg)
			target := pushTarget
			if cfg != nil {
				config.ApplyItchEnv(cfg.Itch.APIKey)
				if target == "" {
					target = cfg.Itch.Target
				}
			} else {
				config.ApplyItchEnv("")
			}
			if pushDry {
				plan := map[string]any{
					"src":       args[0],
					"target":    target,
					"dry_run":   true,
					"env_names": config.EnvNamesUsed(cfg),
				}
				if cfg != nil {
					plan["config"] = cfg.Redacted()
				}
				if err := itch.Push(itch.PushInput{
					Src:         args[0],
					Target:      target,
					UserVersion: pushVersion,
					DryRun:      true,
					IfChanged:   pushIfChanged,
					Hidden:      pushHidden,
					JSON:        jsonOut(opts),
				}); err != nil {
					return err
				}
				return output.Write(formatOf(opts), plan)
			}
			return itch.Push(itch.PushInput{
				Src:         args[0],
				Target:      target,
				UserVersion: pushVersion,
				DryRun:      pushDry,
				IfChanged:   pushIfChanged,
				Hidden:      pushHidden,
				JSON:        jsonOut(opts),
			})
		},
	}
	pushCmd.Flags().StringVar(&pushTarget, "target", "", "user/game[:channel]")
	pushCmd.Flags().StringVar(&pushVersion, "userversion", "", "User-facing version string")
	pushCmd.Flags().BoolVar(&pushDry, "dry-run", false, "Show what would be pushed")
	pushCmd.Flags().BoolVar(&pushIfChanged, "if-changed", false, "Skip empty patches")
	pushCmd.Flags().BoolVar(&pushHidden, "hidden", false, "Mark new channels hidden")
	itchCmd.AddCommand(pushCmd)

	var syncApp, syncTarget, syncBranch, syncCache string
	var syncDry, syncForce, syncNoPush, syncHidden bool
	syncCmd := &cobra.Command{
		Use:   "steam-sync",
		Short: "Copy a Steam app's depots to itch.io (in-process butler/fresh-steamer)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*configPath)
			if err != nil {
				return err
			}
			itch.ApplyConfigEnv(cfg)
			in := itch.SteamSyncInput{
				Target:   syncTarget,
				Branch:   syncBranch,
				CacheDir: syncCache,
				Force:    syncForce,
				DryRun:   syncDry,
				NoPush:   syncNoPush,
				Hidden:   syncHidden,
				JSON:     jsonOut(opts),
			}
			if syncApp != "" {
				id, err := itch.ParseAppID(syncApp)
				if err != nil {
					return err
				}
				in.AppID = id
			}
			if cfg != nil {
				if in.Target == "" {
					in.Target = cfg.Itch.Target
				}
				if in.CacheDir == "" {
					in.CacheDir = cfg.Itch.CacheDir
				}
				if in.AppID == 0 && len(cfg.Itch.SteamSync) > 0 {
					entries, err := itch.EntriesFromConfig(cfg)
					if err != nil {
						return err
					}
					in.Entries = entries
				}
			}
			if syncDry {
				if err := itch.SteamSync(cmd.Context(), in); err != nil {
					return err
				}
				out := map[string]any{
					"dry_run":   true,
					"env_names": config.EnvNamesUsed(cfg),
				}
				if cfg != nil {
					out["config"] = cfg.Redacted()
				}
				return output.Write(formatOf(opts), out)
			}
			return itch.SteamSync(cmd.Context(), in)
		},
	}
	syncCmd.Flags().StringVar(&syncApp, "app", "", "Steam app id")
	syncCmd.Flags().StringVar(&syncTarget, "target", "", "itch.io project user/game")
	syncCmd.Flags().StringVar(&syncBranch, "branch", "", "Steam branch (default public)")
	syncCmd.Flags().StringVar(&syncCache, "cache-dir", "", "Keep downloaded depots between syncs")
	syncCmd.Flags().BoolVar(&syncDry, "dry-run", false, "Print the plan only")
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Push even if the channel already has this Steam build")
	syncCmd.Flags().BoolVar(&syncNoPush, "no-push", false, "Download and assemble only (requires --cache-dir)")
	syncCmd.Flags().BoolVar(&syncHidden, "hidden", false, "Mark new itch channels hidden")
	itchCmd.AddCommand(syncCmd)

	itchCmd.AddCommand(&cobra.Command{
		Use:   "steam-login",
		Short: "Log in to Steam for steam-sync (stores refresh token locally)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := guard.StdioPrompter{}
			user, err := p.Username()
			if err != nil {
				return err
			}
			pw, err := p.Password()
			if err != nil {
				return err
			}
			if err := itch.SteamLoginPassword(cmd.Context(), user, pw, true); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok"})
		},
	})
	itchCmd.AddCommand(&cobra.Command{
		Use:   "steam-logout",
		Short: "Delete local Steam-sync credentials (full account token)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := itch.SteamLogout(); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok"})
		},
	})
	return itchCmd
}

func steamCommands(opts Options, configPath *string) *cobra.Command {
	steamCmd := &cobra.Command{Use: "steam", Short: "SteamCMD upload, Guard, and SetAppBuildLive"}
	steamCmd.AddCommand(guardCommands(opts, configPath))

	var loginUser, loginPass, loginSecret, loginMa, loginVDF string
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Test steamcmd login with builder credentials and internal TOTP",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig(*configPath)
			st, err := tools.EnsureSteamcmd(cmd.Context())
			if err != nil {
				return err
			}
			user, pass, secret, vdf := steamCreds(cfg, loginUser, loginPass, loginSecret, loginMa, loginVDF)
			res, err := steam.Login(cmd.Context(), steam.LoginInput{
				Steamcmd:     st.Steamcmd,
				Username:     user,
				Password:     pass,
				SharedSecret: secret,
				ConfigVDF:    vdf,
			})
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"ok": res})
		},
	}
	loginCmd.Flags().StringVar(&loginUser, "username", "", "Steam builder username")
	loginCmd.Flags().StringVar(&loginPass, "password", "", "Steam builder password")
	loginCmd.Flags().StringVar(&loginSecret, "secret", "", "shared_secret")
	loginCmd.Flags().StringVar(&loginMa, "mafile", "", "Path to SDA/steamguard-cli maFile")
	loginCmd.Flags().StringVar(&loginVDF, "config-vdf", "", "Base64 config.vdf")
	steamCmd.AddCommand(loginCmd)

	var (
		upApp, upDesc, upUser, upPass, upSecret, upMa, upVDF, upRoot string
		upDepots                                                     []string
		upDry                                                        bool
	)
	uploadCmd := &cobra.Command{
		Use:   "upload",
		Short: "Generate VDFs and run steamcmd +run_app_build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*configPath)
			if err != nil {
				return err
			}
			in, err := uploadInput(cfg, upApp, upDesc, upUser, upPass, upSecret, upMa, upVDF, upRoot, upDepots, upDry)
			if err != nil {
				return err
			}
			if !upDry {
				st, err := tools.EnsureSteamcmd(cmd.Context())
				if err != nil {
					return err
				}
				in.Steamcmd = st.Steamcmd
			}
			res, err := steam.Upload(cmd.Context(), in)
			if err != nil {
				return err
			}
			out := map[string]any{
				"app_vdf":   res.AppVDF,
				"build_id":  res.BuildID,
				"dry_run":   res.DryRun,
				"env_names": config.EnvNamesUsed(cfg),
			}
			if upDry && cfg != nil {
				out["config"] = cfg.Redacted()
			}
			return output.Write(formatOf(opts), out)
		},
	}
	uploadCmd.Flags().StringVar(&upApp, "app-id", "", "Steam app id")
	uploadCmd.Flags().StringVar(&upDesc, "description", "", "Build description")
	uploadCmd.Flags().StringVar(&upUser, "username", "", "Steam builder username")
	uploadCmd.Flags().StringVar(&upPass, "password", "", "Steam builder password")
	uploadCmd.Flags().StringVar(&upSecret, "secret", "", "shared_secret")
	uploadCmd.Flags().StringVar(&upMa, "mafile", "", "Path to maFile")
	uploadCmd.Flags().StringVar(&upVDF, "config-vdf", "", "Base64 config.vdf")
	uploadCmd.Flags().StringVar(&upRoot, "content-root", "", "Content root for depots")
	uploadCmd.Flags().StringArrayVar(&upDepots, "depot", nil, "Depot as id=path (repeatable)")
	uploadCmd.Flags().BoolVar(&upDry, "dry-run", false, "Write VDFs without logging in")
	steamCmd.AddCommand(uploadCmd)

	steamCmd.AddCommand(&cobra.Command{
		Use:   "dry-run",
		Short: "Print a VDF plan without steamcmd login",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*configPath)
			if err != nil {
				return err
			}
			in, err := uploadInput(cfg, "", "", "", "", "", "", "", "", nil, true)
			if err != nil {
				return err
			}
			res, err := steam.Upload(cmd.Context(), in)
			if err != nil {
				return err
			}
			body, err := os.ReadFile(res.AppVDF)
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{
				"app_vdf":   res.AppVDF,
				"vdf":       string(body),
				"dry_run":   true,
				"env_names": config.EnvNamesUsed(cfg),
				"config":    redactedConfig(cfg),
			})
		},
	})

	var liveKey, liveApp, liveBuild, liveBeta, liveSteam, liveDesc string
	liveCmd := &cobra.Command{
		Use:   "set-live",
		Short: "POST ISteamApps/SetAppBuildLive/v2/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig(*configPath)
			in := steam.SetLiveInput{
				APIKey:      liveKey,
				AppID:       liveApp,
				BuildID:     liveBuild,
				BetaKey:     liveBeta,
				SteamID:     liveSteam,
				Description: liveDesc,
			}
			if cfg != nil {
				in.APIKey = config.FirstNonEmpty(in.APIKey, cfg.Steam.APIKey)
				in.AppID = config.FirstNonEmpty(in.AppID, cfg.Steam.AppID)
				in.SteamID = config.FirstNonEmpty(in.SteamID, cfg.Steam.SteamID)
			}
			in.APIKey = config.FirstNonEmpty(in.APIKey, os.Getenv("BLAZIUM_STEAM_API_KEY"))
			in.AppID = config.FirstNonEmpty(in.AppID, os.Getenv("BLAZIUM_STEAM_APP_ID"))
			in.SteamID = config.FirstNonEmpty(in.SteamID, os.Getenv("BLAZIUM_STEAM_ID"))
			code, body, err := steam.SetLive(in)
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status_code": code, "body": body})
		},
	}
	liveCmd.Flags().StringVar(&liveKey, "api-key", "", "Steamworks Web API publisher key")
	liveCmd.Flags().StringVar(&liveApp, "app-id", "", "Steam app id")
	liveCmd.Flags().StringVar(&liveBuild, "build-id", "", "Steam build id")
	liveCmd.Flags().StringVar(&liveBeta, "beta-key", "", "Branch key (public, beta, …)")
	liveCmd.Flags().StringVar(&liveSteam, "steam-id", "", "Required when beta-key is public")
	liveCmd.Flags().StringVar(&liveDesc, "description", "", "Internal description")
	steamCmd.AddCommand(liveCmd)
	return steamCmd
}

func guardCommands(opts Options, configPath *string) *cobra.Command {
	g := &cobra.Command{Use: "guard", Short: "In-process Steam Guard (TOTP, setup, maFile)"}
	var totpSecret, totpMa string
	var totpOffset int64
	totpCmd := &cobra.Command{
		Use:   "totp",
		Short: "Print a 5-character Steam Guard code",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig(*configPath)
			secret, err := resolveSecret(totpSecret, totpMa, cfg)
			if err != nil {
				return err
			}
			code, err := guard.GenerateAuthCode(secret, totpOffset)
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"code": code})
		},
	}
	totpCmd.Flags().StringVar(&totpSecret, "secret", "", "shared_secret (base64 or 40-char hex)")
	totpCmd.Flags().StringVar(&totpMa, "mafile", "", "Path to maFile")
	totpCmd.Flags().Int64Var(&totpOffset, "offset", 0, "Seconds added to local time")
	g.AddCommand(totpCmd)

	g.AddCommand(&cobra.Command{
		Use:   "time",
		Short: "Query Steam server time offset vs local clock",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			off, err := guard.QueryTimeOffset()
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"offset_seconds": off})
		},
	})

	var importPath string
	imp := &cobra.Command{
		Use:   "import",
		Short: "Import an SDA / steamguard-cli maFile into the CLI steamguard dir",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if importPath == "" {
				return fmt.Errorf("--mafile is required")
			}
			acc, err := guard.ImportFile(importPath)
			if err != nil {
				return err
			}
			path, err := guard.Save(acc)
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"path": path, "account": acc.AccountName})
		},
	}
	imp.Flags().StringVar(&importPath, "mafile", "", "Path to maFile JSON")
	g.AddCommand(imp)

	var setupUser string
	var printSecret bool
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive AddAuthenticator (account must already have a phone on Steam)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := guard.SetupAuthenticator(guard.StdioPrompter{}, setupUser, "")
			if err != nil {
				return err
			}
			out := map[string]any{
				"path":    res.Path,
				"account": res.Account.AccountName,
				"steamid": string(res.Account.SteamID),
			}
			if printSecret {
				out["shared_secret"] = res.Account.SharedSecret
				fmt.Fprintln(os.Stderr, "warning: --print-shared-secret writes a secret to stdout (shell history risk)")
			}
			return output.Write(formatOf(opts), out)
		},
	}
	setupCmd.Flags().StringVar(&setupUser, "user", "", "Steam username")
	setupCmd.Flags().BoolVar(&printSecret, "print-shared-secret", false, "Include shared_secret in output (CI; avoid shell history)")
	g.AddCommand(setupCmd)

	g.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "QueryAuthenticator status (requires interactive login)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, err := guard.LoginWithPassword(guard.StdioPrompter{}, "", "")
			if err != nil {
				return err
			}
			st, err := guard.QueryStatus(tok.AccessToken, tok.SteamID)
			if err != nil {
				return err
			}
			return output.Write(formatOf(opts), st)
		},
	})
	g.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "RemoveAuthenticator using the revocation code",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := guard.StdioPrompter{}
			tok, err := guard.LoginWithPassword(p, "", "")
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stderr, "Revocation code: ")
			var rev string
			if _, err := fmt.Scanln(&rev); err != nil {
				return err
			}
			if err := guard.RemoveAuthenticator(tok.AccessToken, strings.TrimSpace(rev)); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok"})
		},
	})
	return g
}

func resolveSecret(secret, maFile string, cfg *config.Config) (string, error) {
	if strings.TrimSpace(secret) != "" {
		return strings.TrimSpace(secret), nil
	}
	if cfg != nil && strings.TrimSpace(cfg.Steam.SharedSecret) != "" {
		return strings.TrimSpace(cfg.Steam.SharedSecret), nil
	}
	return guard.SharedSecretFromFlags("", maFile)
}

func redactedConfig(cfg *config.Config) any {
	if cfg == nil {
		return map[string]any{}
	}
	return cfg.Redacted()
}

func steamCreds(cfg *config.Config, user, pass, secret, ma, vdf string) (string, string, string, string) {
	if cfg != nil {
		user = config.FirstNonEmpty(user, cfg.Steam.Username)
		pass = config.FirstNonEmpty(pass, cfg.Steam.Password)
		secret = config.FirstNonEmpty(secret, cfg.Steam.SharedSecret)
		vdf = config.FirstNonEmpty(vdf, cfg.Steam.ConfigVDF)
	}
	user = config.FirstNonEmpty(user, os.Getenv("BLAZIUM_STEAM_USERNAME"))
	pass = config.FirstNonEmpty(pass, os.Getenv("BLAZIUM_STEAM_PASSWORD"))
	secret = config.FirstNonEmpty(secret, os.Getenv("BLAZIUM_STEAM_SHARED_SECRET"))
	vdf = config.FirstNonEmpty(vdf, os.Getenv("BLAZIUM_STEAM_CONFIG_VDF"))
	if secret == "" {
		if s, err := guard.SharedSecretFromFlags("", ma); err == nil {
			secret = s
		}
	}
	return user, pass, secret, vdf
}

func uploadInput(cfg *config.Config, app, desc, user, pass, secret, ma, vdf, root string, depotFlags []string, dry bool) (steam.UploadInput, error) {
	if cfg != nil {
		app = config.FirstNonEmpty(app, cfg.Steam.AppID)
		desc = config.FirstNonEmpty(desc, cfg.Steam.Description)
		root = config.FirstNonEmpty(root, "")
	}
	user, pass, secret, vdf = steamCreds(cfg, user, pass, secret, ma, vdf)
	app = config.FirstNonEmpty(app, os.Getenv("BLAZIUM_STEAM_APP_ID"))
	b := steam.AppBuild{AppID: app, Description: desc, ContentRoot: root, SetLive: ""}
	if len(depotFlags) > 0 {
		for _, d := range depotFlags {
			id, path, ok := strings.Cut(d, "=")
			if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(path) == "" {
				return steam.UploadInput{}, fmt.Errorf("invalid --depot %q (want id=path)", d)
			}
			b.Depots = append(b.Depots, steam.Depot{ID: strings.TrimSpace(id), Path: strings.TrimSpace(path)})
		}
	} else if cfg != nil {
		for _, d := range cfg.Steam.Depots {
			b.Depots = append(b.Depots, steam.Depot{ID: d.ID, Path: d.Path})
		}
	}
	return steam.UploadInput{
		Username:     user,
		Password:     pass,
		SharedSecret: secret,
		ConfigVDF:    vdf,
		App:          b,
		DryRun:       dry,
	}, nil
}
