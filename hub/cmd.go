package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/editorinstall"
	"github.com/blazium-games/blazium-cli/exporttemplates"
	"github.com/blazium-games/blazium-cli/output"

	"github.com/spf13/cobra"
)

// Options configures hub Cobra commands.
type Options struct {
	DefaultRelease string
	Format         *string
	Quiet          *bool
}

// AddCommands registers install, uninstall, editors, install-path, open, load, handle-uri, projects on root.
func AddCommands(root *cobra.Command, opts Options) {
	format := func() string {
		if opts.Format != nil {
			return *opts.Format
		}
		return "human"
	}
	quiet := func() bool {
		return opts.Quiet != nil && *opts.Quiet
	}

	var (
		channel         string
		platform        string
		arch            string
		mono            bool
		withTpl         bool
		yes             bool
		crashReporter   string
		analytics       string
		analyticsMode   string
		noCrashReporter bool
	)

	const crashReporterFlagHelp = "Editor sidecar crash reporter executable (omitted: Hub sidecar under BLAZIUM if present)"
	launchFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&crashReporter, "crash-reporter", "", crashReporterFlagHelp)
		cmd.Flags().BoolVar(&noCrashReporter, "no-crash-reporter", false, "Do not attach a crash reporter sidecar (overrides --crash-reporter and the Hub default)")
		cmd.Flags().StringVar(&analytics, "analytics", "", "Editor analytics consent: accepted|declined (omitted: editor keeps its own setting)")
		cmd.Flags().StringVar(&analyticsMode, "analytics-mode", "", "Editor analytics mode: anonymous|identified")
	}

	resolveLaunchPrivacy := func() (consent, mode string, err error) {
		consent, err = NormalizeAnalyticsConsent(analytics)
		if err != nil {
			return "", "", err
		}
		mode, err = NormalizeAnalyticsMode(analyticsMode)
		if err != nil {
			return "", "", err
		}
		return consent, mode, nil
	}

	installCmd := &cobra.Command{
		Use:     "install [version]",
		Aliases: []string{"i"},
		Short:   "Download and install a Blazium editor",
		Long: `Install a Blazium editor from the CDN into the configured install-path.

Version may be a concrete version (e.g. 0.6.714), "nightly", "latest", or "lts".
Use --templates to also download and install export templates.`,
		Example: `  blazium-cli install 0.6.714
  blazium-cli install nightly --templates
  blazium-cli install lts --platform windows --arch x86_64`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			verArg := ""
			if len(args) > 0 {
				verArg = args[0]
			}
			baseVersion, isNightly, err := cdn.ResolveInstallVersion(verArg, channel, opts.DefaultRelease)
			if err != nil {
				return err
			}
			resolvedChannel := InferChannel(baseVersion, isNightly)
			if strings.TrimSpace(channel) != "" {
				resolvedChannel, err = NormalizeChannel(channel)
				if err != nil {
					return err
				}
			}
			if platform == "" || arch == "" {
				dp, da := cdn.DefaultPlatformArch()
				if platform == "" {
					platform = dp
				}
				if arch == "" {
					arch = da
				}
			}
			list, err := cdn.LoadEditorMetadata(baseVersion, resolvedChannel)
			if err != nil {
				return err
			}
			meta, err := cdn.FindEditorMetadata(list, baseVersion, platform, arch, mono)
			if err != nil {
				return err
			}

			f, err := Load()
			if err != nil {
				return err
			}
			installRoot, err := f.EffectiveInstallPath()
			if err != nil {
				return err
			}
			destDir := EditorInstallDir(installRoot, resolvedChannel, baseVersion)
			tmpZip := filepath.Join(os.TempDir(), meta.Filename)
			fmt.Fprintf(os.Stderr, "Downloading %s\n", meta.DownloadURL)
			if err := cdn.DownloadFile(meta.DownloadURL, tmpZip); err != nil {
				return err
			}
			defer os.Remove(tmpZip)

			fmt.Fprintf(os.Stderr, "Installing to %s\n", destDir)
			bin, err := editorinstall.InstallEditorTree(tmpZip, destDir)
			if err != nil {
				return err
			}
			ed := Editor{
				Version:  baseVersion,
				Path:     bin,
				Dir:      destDir,
				Platform: platform,
				Arch:     arch,
				Mono:     mono,
				Channel:  resolvedChannel,
			}
			f.UpsertEditor(ed)
			if err := Save(f); err != nil {
				return err
			}

			if withTpl {
				tplDest := editorinstall.DefaultTemplatesDest()
				installed, err := exporttemplates.InstallTPZ(baseVersion, mono, resolvedChannel == ChannelNightly, tplDest)
				if err != nil {
					return fmt.Errorf("templates: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Templates installed to %s/%s\n", tplDest, installed)
			}

			resolved, _ := f.ResolveDefault()
			resolvedVer := ""
			if resolved != nil {
				resolvedVer = resolved.Version
			}
			return output.Write(format(), map[string]any{
				"ok":       true,
				"version":  ed.Version,
				"channel":  ed.Channel,
				"path":     ed.Path,
				"dir":      ed.Dir,
				"default":  resolvedVer,
				"platform": ed.Platform,
				"arch":     ed.Arch,
			})
		},
	}
	installCmd.Flags().StringVar(&channel, "channel", "", "release|prerelease|nightly")
	installCmd.Flags().StringVar(&platform, "platform", "", "Platform (linux, windows, macos); default: host")
	installCmd.Flags().StringVar(&arch, "arch", "", "Architecture (x86_64, arm64); default: host")
	installCmd.Flags().BoolVar(&mono, "mono", false, "Install mono editor variant")
	installCmd.Flags().BoolVar(&withTpl, "templates", false, "Also install export templates (.tpz)")
	installCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Accept prompts (reserved)")

	var uninstallChannel string
	uninstallCmd := &cobra.Command{
		Use:     "uninstall <version>",
		Aliases: []string{"u"},
		Short:   "Uninstall a registered Blazium editor",
		Long:    "Removes the editor from hub.json and deletes its install directory.\nWhen the same version exists on multiple channels, pass --channel.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			ed, err := f.RemoveEditor(args[0], uninstallChannel)
			if err != nil {
				return err
			}
			if ed.Dir != "" {
				if err := os.RemoveAll(ed.Dir); err != nil {
					return fmt.Errorf("remove %s: %w", ed.Dir, err)
				}
			}
			if err := Save(f); err != nil {
				return err
			}
			return output.Write(format(), map[string]any{
				"ok":      true,
				"removed": ed.Version,
				"channel": ed.EditorChannel(),
				"dir":     ed.Dir,
			})
		},
	}
	uninstallCmd.Flags().StringVar(&uninstallChannel, "channel", "", "release|prerelease|nightly (required if version exists on multiple channels)")
	uninstallCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation (always non-interactive)")

	installPathCmd := &cobra.Command{
		Use:     "install-path [path]",
		Aliases: []string{"ip"},
		Short:   "Get or set the editors install directory",
		Long:    "With no argument, prints the current install-path. With a path, persists it in hub.json (does not move existing installs).",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				p, err := f.EffectiveInstallPath()
				if err != nil {
					return err
				}
				return output.Write(format(), map[string]any{"install_path": p})
			}
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if err := os.MkdirAll(abs, 0o755); err != nil {
				return err
			}
			f.InstallPath = abs
			if err := Save(f); err != nil {
				return err
			}
			return output.Write(format(), map[string]any{"install_path": abs, "ok": true})
		},
	}

	var addVersion, addPlatform, addArch string
	var addMono bool

	editorsCmd := &cobra.Command{
		Use:     "editors",
		Aliases: []string{"e"},
		Short:   "List and manage installed editors",
		Long:    "List registered editors, add a local binary, set the default, or print an editor path.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			rows := make([]any, 0, len(f.Editors))
			resolved, _ := f.ResolveDefault()
			resolvedVer := ""
			if resolved != nil {
				resolvedVer = resolved.Version
			}
			for _, ed := range f.Editors {
				isDefault := resolved != nil &&
					ed.Version == resolved.Version &&
					ed.EditorChannel() == resolved.EditorChannel()
				rows = append(rows, map[string]any{
					"version":  ed.Version,
					"channel":  ed.EditorChannel(),
					"path":     ed.Path,
					"dir":      ed.Dir,
					"platform": ed.Platform,
					"arch":     ed.Arch,
					"mono":     ed.Mono,
					"default":  isDefault,
				})
			}
			return output.Write(format(), map[string]any{
				"default_editor":         f.DefaultEditor,
				"default_editor_channel": f.EffectiveEditorChannel(),
				"default_editor_version": f.EffectiveEditorVersionPolicy(),
				"resolved_default":       resolvedVer,
				"install_path":           f.InstallPath,
				"editors":                rows,
			})
		},
	}

	editorsAdd := &cobra.Command{
		Use:   "add <path>",
		Short: "Register a local editor binary or directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			if addPlatform == "" || addArch == "" {
				dp, da := cdn.DefaultPlatformArch()
				if addPlatform == "" {
					addPlatform = dp
				}
				if addArch == "" {
					addArch = da
				}
			}
			ed, err := f.AddEditorFromPath(args[0], addVersion, addPlatform, addArch, "", addMono)
			if err != nil {
				return err
			}
			if err := Save(f); err != nil {
				return err
			}
			return output.Write(format(), map[string]any{"ok": true, "editor": ed, "default": f.DefaultEditor})
		},
	}
	editorsAdd.Flags().StringVar(&addVersion, "version", "", "Editor version label")
	editorsAdd.Flags().StringVar(&addPlatform, "platform", "", "Platform label")
	editorsAdd.Flags().StringVar(&addArch, "arch", "", "Architecture label")
	editorsAdd.Flags().BoolVar(&addMono, "mono", false, "Mono variant")

	var defChannel, defVersion string
	editorsDefault := &cobra.Command{
		Use:   "default [version]",
		Short: "Get or set the default editor (channel + latest/concrete version)",
		Long: `Without flags, prints the resolved default editor.

Set policy with --channel release|prerelease|nightly and/or --version latest|<ver>.
Passing a positional version hard-pins that installed editor (overrides channel policy).
Unset policy defaults to latest release.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			if len(args) == 0 && defChannel == "" && defVersion == "" {
				ed, err := f.ResolveDefault()
				if err != nil {
					return err
				}
				return output.Write(format(), map[string]any{
					"default_editor_pin":     f.DefaultEditor,
					"default_editor_channel": f.EffectiveEditorChannel(),
					"default_editor_version": f.EffectiveEditorVersionPolicy(),
					"resolved":               ed.Version,
					"path":                   ed.Path,
					"channel":                ed.EditorChannel(),
				})
			}
			if defChannel != "" || defVersion != "" {
				if err := f.SetDefaultEditorPolicy(defChannel, defVersion); err != nil {
					return err
				}
			}
			if len(args) == 1 {
				if err := f.SetDefaultEditor(args[0]); err != nil {
					return err
				}
			}
			if err := Save(f); err != nil {
				return err
			}
			ed, err := f.ResolveDefault()
			if err != nil {
				return err
			}
			return output.Write(format(), map[string]any{
				"ok":                     true,
				"default_editor_pin":     f.DefaultEditor,
				"default_editor_channel": f.EffectiveEditorChannel(),
				"default_editor_version": f.EffectiveEditorVersionPolicy(),
				"resolved":               ed.Version,
				"path":                   ed.Path,
			})
		},
	}
	editorsDefault.Flags().StringVar(&defChannel, "channel", "", "Default channel: release, prerelease, or nightly")
	editorsDefault.Flags().StringVar(&defVersion, "version", "", "Version policy: latest or a concrete version")

	editorsPath := &cobra.Command{
		Use:   "path <version>",
		Short: "Print the binary path for an installed editor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			ed := f.FindEditor(args[0])
			if ed == nil {
				return fmt.Errorf("editor version %q is not registered", args[0])
			}
			if format() == "human" {
				fmt.Println(ed.Path)
				return nil
			}
			return output.Write(format(), map[string]any{"version": ed.Version, "path": ed.Path})
		},
	}
	editorsCmd.AddCommand(editorsAdd, editorsDefault, editorsPath)

	projectsCmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"p"},
		Short:   "List and manage the local project registry",
		Long:    "Local-only project list stored in hub.json (no cloud sync).",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			rows := make([]any, 0, len(f.Projects))
			for _, p := range f.Projects {
				rows = append(rows, map[string]any{
					"name":        p.Name,
					"path":        p.Path,
					"last_opened": p.LastOpened,
				})
			}
			return output.Write(format(), map[string]any{"projects": rows})
		},
	}
	projectsAdd := &cobra.Command{
		Use:   "add <path>",
		Short: "Register a project directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			p, err := f.AddProject(args[0])
			if err != nil {
				return err
			}
			if err := Save(f); err != nil {
				return err
			}
			return output.Write(format(), map[string]any{"ok": true, "project": p})
		},
	}
	projectsRemove := &cobra.Command{
		Use:   "remove <path-or-name>",
		Short: "Unregister a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			if err := f.RemoveProject(args[0]); err != nil {
				return err
			}
			if err := Save(f); err != nil {
				return err
			}
			return output.Write(format(), map[string]any{"ok": true, "removed": args[0]})
		},
	}
	projectsCmd.AddCommand(projectsAdd, projectsRemove)

	runLaunch := func(projectArg string, fullProfile bool) error {
		f, err := Load()
		if err != nil {
			return err
		}
		projectPath, err := f.ResolveProjectPath(projectArg)
		if err != nil {
			return err
		}
		profile, err := LoadProjectProfile(projectPath)
		if err != nil {
			return err
		}
		ed, reason, err := f.ResolveEditorForProject(projectPath)
		if err != nil {
			return err
		}
		if err := f.TouchProjectLastOpened(projectPath); err != nil {
			return err
		}
		if err := Save(f); err != nil {
			return err
		}
		consent, mode, err := resolveLaunchPrivacy()
		if err != nil {
			return err
		}
		result, err := LaunchEditor(LaunchOptions{
			EditorPath:        ed.Path,
			EditorVersion:     ed.Version,
			ProjectPath:       projectPath,
			Profile:           profile,
			Quiet:             quiet(),
			CrashReporterPath: crashReporter,
			SkipCrashReporter: noCrashReporter,
			AnalyticsConsent:  consent,
			AnalyticsMode:     mode,
		})
		if err != nil {
			return err
		}
		out := map[string]any{
			"ok":             true,
			"project":        projectPath,
			"editor":         ed.Version,
			"editor_path":    ed.Path,
			"resolve_reason": reason,
			"pid":            result.Instance.PID,
		}
		if result.RemoteEnabled {
			out["instance_id"] = result.Instance.ID
			out["remote_control"] = map[string]any{
				"host":  result.Instance.Host,
				"port":  result.Instance.RemotePort,
				"token": result.Instance.RemoteToken,
				"bound": result.Instance.Bound,
			}
		}
		if result.MCPEnabled {
			out["justamcp"] = map[string]any{
				"enabled": true,
				"port":    result.Instance.MCPPort,
			}
		}
		if fullProfile {
			out["profile"] = map[string]any{
				"name":           profile.Name,
				"features":       profile.Features,
				"editor_version": profile.EditorVersion,
				"justamcp":       profile.JustAMCP,
				"remote_control": profile.RemoteControl,
			}
		}
		return output.Write(format(), out)
	}

	openCmd := &cobra.Command{
		Use:   "open <project-path-or-name>",
		Short: "Open a project in the resolved Blazium editor",
		Long: `Launches the editor with remote_control enabled by default (unique port/token/instance id).

Editor resolution order:
  1. blazium/editor_version in project.blazium or project.godot
  2. Matching installed editor for config/features
  3. Default editor policy from hub.json (latest release unless configured)`,
		Example: `  blazium-cli open ./MyProject
  blazium-cli open MyGame`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLaunch(args[0], false)
		},
	}

	launchFlags(openCmd)

	loadCmd := &cobra.Command{
		Use:   "load <project-path-or-name>",
		Short: "Profile a project (JustAMCP/remote_control/etc.) and launch the editor",
		Long: `Reads project.blazium or project.godot for JustAMCP, remote_control, and editor settings, then launches
the same way as open — allocating unique ports/tokens and binding a short instance id
after remote_control is ready.`,
		Example: `  blazium-cli load ./MyProject
  blazium-cli load MyGame --quiet`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLaunch(args[0], true)
		},
	}
	launchFlags(loadCmd)

	handleURICmd := &cobra.Command{
		Use:   "handle-uri <uri>",
		Short: "Handle a blazium:// deep link (open, load, install, register, hub)",
		Long: `Parses blazium:// URIs from the Hub or OS handlers and performs the matching action.

Supported forms:
  blazium://open?path=<path-or-file-url>
  blazium://load?path=<path-or-file-url>
  blazium://project/<url-encoded-path>
  blazium://install?version=<ver>&channel=<optional>&platform=&arch=&mono=
  blazium://register?path=<path>&version=&channel=&platform=&arch=&mono=
  blazium://hub  (or blazium:// with empty host)`,
		Example: `  blazium-cli handle-uri "blazium://open?path=C%3A%5CGames%5CFoo"
  blazium-cli handle-uri "blazium://install?version=0.6.714"
  blazium-cli handle-uri "blazium://register?path=C%3A%5CEditors%5Cblazium.exe&version=0.6.714&channel=release"
  blazium-cli handle-uri blazium://hub`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			consent, mode, err := resolveLaunchPrivacy()
			if err != nil {
				return err
			}
			out, err := HandleURI(HandleURIOptions{
				URI:               args[0],
				DefaultRelease:    opts.DefaultRelease,
				Quiet:             quiet(),
				CrashReporterPath: crashReporter,
				SkipCrashReporter: noCrashReporter,
				AnalyticsConsent:  consent,
				AnalyticsMode:     mode,
			})
			if err != nil {
				return err
			}
			return output.Write(format(), out)
		},
	}
	launchFlags(handleURICmd)

	hubRemotePath := ""
	hubRemoteCmd := &cobra.Command{
		Use:   "hub-remote",
		Short: "Manage Hub remote_control secret (hub_remote.json)",
	}
	hubRemoteEnsure := &cobra.Command{
		Use:   "ensure",
		Short: "Load or create hub_remote.json (never rotates a valid token)",
		Long: `Ensures an authenticated Hub remote_control secret exists.

Without --path: uses user config, then machine config; creates the user file if both missing.
With --path: ensures only that file (installer/machine use).`,
		Example: `  blazium-cli hub-remote ensure
  blazium-cli hub-remote ensure --path "C:\\ProgramData\\blazium\\hub_remote.json"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				f   HubRemoteFile
				err error
			)
			path := strings.TrimSpace(hubRemotePath)
			if path == "" {
				f, err = EnsureHubRemoteSecret()
			} else {
				f, err = EnsureHubRemoteSecretAt(path)
			}
			if err != nil {
				return err
			}
			outPath := path
			if outPath == "" {
				outPath, _ = HubRemoteConfigPath()
			}
			return output.Write(format(), map[string]any{
				"ok":     true,
				"action": "hub-remote-ensure",
				"path":   outPath,
				"host":   f.Host,
				"port":   f.Port,
				// Token intentionally omitted from default human output surface;
				// still present in JSON for automation that already has file access.
				"token": f.Token,
			})
		},
	}
	hubRemoteEnsure.Flags().StringVar(&hubRemotePath, "path", "", "Explicit hub_remote.json path")
	hubRemoteCmd.AddCommand(hubRemoteEnsure)

	addTemplatesCommands(root, opts, format)
	root.AddCommand(installCmd, uninstallCmd, installPathCmd, editorsCmd, projectsCmd, openCmd, loadCmd, handleURICmd, hubRemoteCmd)
}
