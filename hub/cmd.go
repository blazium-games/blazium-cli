package hub

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"blazium-cli/cdn"
	"blazium-cli/editorinstall"
	"blazium-cli/output"

	"github.com/spf13/cobra"
)

// Options configures hub Cobra commands.
type Options struct {
	DefaultRelease string
	Format         *string
}

// AddCommands registers install, uninstall, editors, install-path, open, projects on root.
func AddCommands(root *cobra.Command, opts Options) {
	format := func() string {
		if opts.Format != nil {
			return *opts.Format
		}
		return "human"
	}

	var (
		channel  string
		platform string
		arch     string
		mono     bool
		withTpl  bool
		yes      bool
	)

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
			if platform == "" || arch == "" {
				dp, da := cdn.DefaultPlatformArch()
				if platform == "" {
					platform = dp
				}
				if arch == "" {
					arch = da
				}
			}
			list, err := cdn.LoadEditorMetadata(baseVersion, isNightly)
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
			destDir := filepath.Join(installRoot, baseVersion)
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
			}
			f.UpsertEditor(ed)
			if f.DefaultEditor == "" || len(f.Editors) == 1 {
				f.DefaultEditor = baseVersion
			}
			if err := Save(f); err != nil {
				return err
			}

			if withTpl {
				tplURL := cdn.TemplatesTPZURL(baseVersion, mono, isNightly)
				tplPath := filepath.Join(os.TempDir(), filepath.Base(tplURL))
				fmt.Fprintf(os.Stderr, "Downloading templates %s\n", tplURL)
				if err := cdn.DownloadFile(tplURL, tplPath); err != nil {
					return fmt.Errorf("templates: %w", err)
				}
				defer os.Remove(tplPath)
				tplDest := editorinstall.DefaultTemplatesDest()
				installed, err := editorinstall.InstallTemplatesFromTPZ(tplPath, tplDest)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Templates installed to %s/%s\n", tplDest, installed)
			}

			return output.Write(format(), map[string]any{
				"ok":       true,
				"version":  ed.Version,
				"path":     ed.Path,
				"dir":      ed.Dir,
				"default":  f.DefaultEditor,
				"platform": ed.Platform,
				"arch":     ed.Arch,
			})
		},
	}
	installCmd.Flags().StringVar(&channel, "channel", "", "Release channel: nightly or release")
	installCmd.Flags().StringVar(&platform, "platform", "", "Platform (linux, windows, macos); default: host")
	installCmd.Flags().StringVar(&arch, "arch", "", "Architecture (x86_64, arm64); default: host")
	installCmd.Flags().BoolVar(&mono, "mono", false, "Install mono editor variant")
	installCmd.Flags().BoolVar(&withTpl, "templates", false, "Also install export templates (.tpz)")
	installCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Accept prompts (reserved)")

	uninstallCmd := &cobra.Command{
		Use:     "uninstall <version>",
		Aliases: []string{"u"},
		Short:   "Uninstall a registered Blazium editor",
		Long:    "Removes the editor from hub.json and deletes its install directory.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			ed, err := f.RemoveEditor(args[0])
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
				"dir":     ed.Dir,
			})
		},
	}
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
			for _, ed := range f.Editors {
				rows = append(rows, map[string]any{
					"version":  ed.Version,
					"path":     ed.Path,
					"dir":      ed.Dir,
					"platform": ed.Platform,
					"arch":     ed.Arch,
					"mono":     ed.Mono,
					"default":  ed.Version == f.DefaultEditor,
				})
			}
			return output.Write(format(), map[string]any{
				"default_editor": f.DefaultEditor,
				"install_path":   f.InstallPath,
				"editors":        rows,
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
			ed, err := f.AddEditorFromPath(args[0], addVersion, addPlatform, addArch, addMono)
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

	editorsDefault := &cobra.Command{
		Use:   "default [version]",
		Short: "Get or set the default editor version",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				ed, err := f.ResolveDefault()
				if err != nil {
					return err
				}
				return output.Write(format(), map[string]any{"default_editor": ed.Version, "path": ed.Path})
			}
			if err := f.SetDefaultEditor(args[0]); err != nil {
				return err
			}
			if err := Save(f); err != nil {
				return err
			}
			return output.Write(format(), map[string]any{"ok": true, "default_editor": f.DefaultEditor})
		},
	}

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

	openCmd := &cobra.Command{
		Use:   "open <project-path-or-name>",
		Short: "Open a project in the resolved Blazium editor",
		Long: `Launches the editor with --path <project>.

Editor resolution order:
  1. blazium/editor_version in project.godot
  2. Matching installed editor for config/features
  3. Default editor from hub.json`,
		Example: `  blazium-cli open ./MyProject
  blazium-cli open MyGame`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := Load()
			if err != nil {
				return err
			}
			projectPath, err := f.ResolveProjectPath(args[0])
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
			if err := launchEditor(ed.Path, projectPath); err != nil {
				return err
			}
			return output.Write(format(), map[string]any{
				"ok":            true,
				"project":       projectPath,
				"editor":        ed.Version,
				"editor_path":   ed.Path,
				"resolve_reason": reason,
			})
		},
	}

	root.AddCommand(installCmd, uninstallCmd, installPathCmd, editorsCmd, projectsCmd, openCmd)
}

func launchEditor(editorPath, projectPath string) error {
	var cmd *exec.Cmd
	if strings.HasSuffix(strings.ToLower(editorPath), ".app") {
		cmd = exec.Command("open", "-a", editorPath, "--args", "--path", projectPath)
	} else {
		cmd = exec.Command(editorPath, "--path", projectPath)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch editor: %w", err)
	}
	// Detach: do not wait for the editor process.
	go func() { _ = cmd.Wait() }()
	return nil
}
