package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/output"
	"github.com/blazium-games/blazium-cli/upgrade"

	"github.com/spf13/cobra"
)

// Options configures the update command group.
type Options struct {
	CLIVersion string
	Format     *string
}

// NewCommand returns the update Cobra command with check/apply subcommands.
func NewCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for or apply Blazium product updates",
		Long: `Check CDN catalogs for updates to blazium-cli, Blazium Hub, crash reporter, toolchain, editors, and export templates.
Apply downloads and installs CLI, Hub (via the published installer), the Hub crash reporter sidecar, or the Blazium Toolchain manager.`,
		Example: `  blazium-cli update check
  blazium-cli update check --product cli --json
  blazium-cli update apply --product cli
  blazium-cli update apply --product hub --current 0.1.0 --install-root "C:\\Program Files\\Blazium" --launch
  blazium-cli update apply --product crash_reporter --install-root "C:\\Program Files\\Blazium"
  blazium-cli update apply --product toolchain --install-root "C:\\Program Files\\Blazium"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newCheckCommand(opts))
	cmd.AddCommand(newApplyCommand(opts))
	cmd.AddCommand(newReplaceBinCommand(opts))
	return cmd
}

func formatOf(opts Options) string {
	if opts.Format != nil && *opts.Format != "" {
		return *opts.Format
	}
	return "human"
}

func newCheckCommand(opts Options) *cobra.Command {
	var product string
	var channel string
	var hubCurrent string
	var installRoot string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check CDN for product updates (non-mutating)",
		RunE: func(cmd *cobra.Command, args []string) error {
			products := []string{"all"}
			if p := strings.TrimSpace(product); p != "" && p != "all" {
				products = strings.Split(p, ",")
			}
			statuses, err := Check(products, CheckOptions{
				CLIVersion:  opts.CLIVersion,
				HubCurrent:  hubCurrent,
				Channel:     channel,
				InstallRoot: installRoot,
			})
			if err != nil {
				return err
			}
			available := false
			for _, s := range statuses {
				if s.UpdateAvailable {
					available = true
					break
				}
			}
			return output.Write(formatOf(opts), map[string]any{
				"products":         statuses,
				"update_available": available,
			})
		},
	}
	cmd.Flags().StringVar(&product, "product", "all", "Product(s): all, cli, hub, crash_reporter, toolchain, editor, templates (comma-separated)")
	cmd.Flags().StringVar(&channel, "channel", "release", "Editor/templates channel: release or nightly")
	cmd.Flags().StringVar(&hubCurrent, "current", "", "Current Hub version (also BLAZIUM_HUB_VERSION)")
	cmd.Flags().StringVar(&installRoot, "install-root", "", "Hub install root for VERSION discovery / apply")
	return cmd
}

func newApplyCommand(opts Options) *cobra.Command {
	var product string
	var target string
	var hubCurrent string
	var installRoot string
	var dryRun bool
	var launch bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Download and apply an update for cli, hub, crash_reporter, or toolchain",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := strings.ToLower(strings.TrimSpace(product))
			switch p {
			case "cli":
				plan, err := upgrade.ResolvePlan(opts.CLIVersion, target)
				if err != nil {
					return err
				}
				available := cdn.CompareSemver(plan.Version, opts.CLIVersion) > 0
				if dryRun {
					return output.Write(formatOf(opts), map[string]any{
						"dry_run":          true,
						"product":          "cli",
						"current_version":  opts.CLIVersion,
						"target_version":   plan.Version,
						"update_available": available,
						"url":              plan.URL,
						"sha256":           plan.SHA256,
						"filename":         plan.Filename,
						"size":             plan.Size,
					})
				}
				if !available && strings.TrimSpace(target) == "" {
					return fmt.Errorf("cli already at %s (latest %s)", opts.CLIVersion, plan.Version)
				}
				if err := upgrade.Apply(plan); err != nil {
					return err
				}
				return output.Write(formatOf(opts), map[string]any{
					"ok":        true,
					"product":   "cli",
					"previous":  opts.CLIVersion,
					"installed": plan.Version,
					"path":      plan.DestPath,
				})
			case "hub":
				plan, err := ResolveHubPlan(hubCurrent, target, installRoot)
				if err != nil {
					return err
				}
				available := plan.Current == "" || cdn.CompareSemver(plan.Version, plan.Current) > 0
				if dryRun {
					return output.Write(formatOf(opts), map[string]any{
						"dry_run":          true,
						"product":          "hub",
						"current_version":  plan.Current,
						"target_version":   plan.Version,
						"update_available": available,
						"url":              plan.URL,
						"sha256":           plan.SHA256,
						"filename":         plan.Filename,
						"size":             plan.Size,
						"install_root":     plan.InstallRoot,
						"launch":           launch,
					})
				}
				plan, err = ApplyHub(hubCurrent, target, installRoot, launch)
				if err != nil {
					return err
				}
				return output.Write(formatOf(opts), map[string]any{
					"ok":             true,
					"product":        "hub",
					"previous":       plan.Current,
					"installed":      plan.Version,
					"installer_path": plan.InstallerPath,
					"install_root":   plan.InstallRoot,
					"launch":         plan.Launch,
				})
			case "crash_reporter":
				plan, err := ResolveCrashReporterPlan(target, installRoot)
				if err != nil {
					return err
				}
				available := plan.Current == "" || cdn.CompareSemver(plan.Version, plan.Current) > 0
				if dryRun {
					return output.Write(formatOf(opts), map[string]any{
						"dry_run":          true,
						"product":          "crash_reporter",
						"current_version":  plan.Current,
						"target_version":   plan.Version,
						"update_available": available,
						"url":              plan.URL,
						"sha256":           plan.SHA256,
						"filename":         plan.Filename,
						"size":             plan.Size,
						"install_root":     plan.InstallRoot,
						"path":             plan.DestPath,
					})
				}
				plan, err = ApplyCrashReporter(target, installRoot)
				if err != nil {
					return err
				}
				return output.Write(formatOf(opts), map[string]any{
					"ok":        true,
					"product":   "crash_reporter",
					"previous":  plan.Current,
					"installed": plan.Version,
					"path":      plan.DestPath,
				})
			case "toolchain":
				plan, err := ResolveToolchainPlan(target, installRoot)
				if err != nil {
					return err
				}
				available := plan.Current == "" || cdn.CompareSemver(plan.Version, plan.Current) > 0
				if dryRun {
					return output.Write(formatOf(opts), map[string]any{
						"dry_run":          true,
						"product":          "toolchain",
						"current_version":  plan.Current,
						"target_version":   plan.Version,
						"update_available": available,
						"url":              plan.URL,
						"sha256":           plan.SHA256,
						"filename":         plan.Filename,
						"size":             plan.Size,
						"install_root":     plan.InstallRoot,
						"path":             plan.DestPath,
					})
				}
				plan, err = ApplyToolchain(target, installRoot)
				if err != nil {
					return err
				}
				return output.Write(formatOf(opts), map[string]any{
					"ok":        true,
					"product":   "toolchain",
					"previous":  plan.Current,
					"installed": plan.Version,
					"path":      plan.DestPath,
				})
			default:
				return fmt.Errorf("apply supports --product cli|hub|crash_reporter|toolchain (got %q); editor/templates use install / templates download", product)
			}
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "Product to update: cli, hub, crash_reporter, or toolchain (required)")
	cmd.Flags().StringVar(&target, "target", "", "Specific version (default: manifest latest)")
	cmd.Flags().StringVar(&hubCurrent, "current", "", "Current Hub version")
	cmd.Flags().StringVar(&installRoot, "install-root", "", "Hub install directory for Inno /DIR=")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show plan without downloading/installing")
	cmd.Flags().BoolVar(&launch, "launch", false, "After Hub install, start Hub (Windows: Inno /LAUNCH)")
	_ = cmd.MarkFlagRequired("product")
	return cmd
}

// newReplaceBinCommand installs a pre-downloaded CLI binary (used by elevated helpers).
func newReplaceBinCommand(opts Options) *cobra.Command {
	var from string
	var to string
	var versionFile string
	var version string
	cmd := &cobra.Command{
		Use:    "replace-bin",
		Short:  "Replace a CLI binary from a local file (internal / elevated helper)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			from = strings.TrimSpace(from)
			to = strings.TrimSpace(to)
			versionFile = strings.TrimSpace(versionFile)
			if from == "" && to == "" && versionFile == "" {
				return fmt.Errorf("--from/--to or --version-file is required")
			}
			result := map[string]any{"ok": true}
			if from != "" || to != "" {
				if from == "" || to == "" {
					return fmt.Errorf("--from and --to are required together")
				}
				fromAbs, err := filepath.Abs(from)
				if err != nil {
					return err
				}
				toAbs, err := filepath.Abs(to)
				if err != nil {
					return err
				}
				if st, err := os.Stat(fromAbs); err != nil || st.IsDir() {
					return fmt.Errorf("source binary not found: %s", fromAbs)
				}
				if err := upgrade.ReplaceExecutable(toAbs, fromAbs); err != nil {
					return err
				}
				result["from"] = fromAbs
				result["to"] = toAbs
			}
			if versionFile != "" {
				verAbs, err := filepath.Abs(versionFile)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(verAbs), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(verAbs, []byte(version+"\n"), 0o644); err != nil {
					return err
				}
				result["version_file"] = verAbs
				result["version"] = version
			}
			return output.Write(formatOf(opts), result)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Downloaded CLI binary path")
	cmd.Flags().StringVar(&to, "to", "", "Destination blazium-cli path")
	cmd.Flags().StringVar(&versionFile, "version-file", "", "Optional sidecar version file to write after replace")
	cmd.Flags().StringVar(&version, "version", "", "Contents written to --version-file")
	return cmd
}
