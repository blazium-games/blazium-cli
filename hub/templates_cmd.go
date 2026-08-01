package hub

import (
	"fmt"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/editorinstall"
	"github.com/blazium-games/blazium-cli/exporttemplates"
	"github.com/blazium-games/blazium-cli/output"

	"github.com/spf13/cobra"
)

func addTemplatesCommands(root *cobra.Command, opts Options, format func() string) {
	templatesCmd := &cobra.Command{
		Use:     "templates",
		Aliases: []string{"t"},
		Short:   "List and download Blazium export templates",
		Long: `Work with export templates from cdn.blazium.app.

Use list to browse the per-file catalog, download to fetch individual files,
platform sets, a runtime set (web+linux+windows), all per-file artifacts, or
the full .tpz bundle.`,
	}

	var listChannel, listPlatform, listVariant string
	var listMono, listNoMono bool
	listCmd := &cobra.Command{
		Use:   "list [version]",
		Short: "List available export templates for a version",
		Example: `  blazium-cli templates list nightly
  blazium-cli templates list 0.6.748 --platform web
  blazium-cli templates list nightly --format json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			verArg := ""
			if len(args) > 0 {
				verArg = args[0]
			}
			baseVersion, isNightly, err := cdn.ResolveInstallVersion(verArg, listChannel, opts.DefaultRelease)
			if err != nil {
				return err
			}
			entries, err := exporttemplates.LoadMetadata(baseVersion, isNightly)
			if err != nil {
				return err
			}
			filter := exporttemplates.FilterOpts{Platform: listPlatform}
			if listMono {
				filter.MonoOnly = true
			}
			if listNoMono {
				filter.SkipMono = true
			}
			entries = exporttemplates.Filter(entries, filter)
			if listVariant != "" {
				variant := exporttemplates.VariantFromString(listVariant)
				var filtered []exporttemplates.Entry
				for _, e := range entries {
					if matchesVariantHint(e.Filename, variant) {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			rows := make([]any, 0, len(entries))
			for _, e := range entries {
				sha := e.Sha256
				if len(sha) > 12 {
					sha = sha[:12]
				}
				rows = append(rows, map[string]any{
					"filename":     e.Filename,
					"platform":     e.Platform,
					"arch":         e.Arch,
					"mono":         e.Mono,
					"size":         e.Size,
					"sha256":       sha,
					"download_url": e.DownloadURL,
					"version":      e.Version,
				})
			}
			return output.Write(format(), map[string]any{
				"version":   baseVersion,
				"channel":   cdn.ChannelName(isNightly),
				"count":     len(rows),
				"templates": rows,
			})
		},
	}
	listCmd.Flags().StringVar(&listChannel, "channel", "", "Release channel: nightly or release")
	listCmd.Flags().StringVar(&listPlatform, "platform", "", "Filter by platform (web, linux, windows, android, …)")
	listCmd.Flags().StringVar(&listVariant, "variant", "", "Optional filename hint filter: debug or release")
	listCmd.Flags().BoolVar(&listMono, "mono", false, "Show only mono templates")
	listCmd.Flags().BoolVar(&listNoMono, "no-mono", false, "Hide mono templates")

	var (
		dlChannel  string
		dlPlatform string
		dlVariant  string
		dlDest     string
		dlFiles    []string
		dlRuntime  bool
		dlAll      bool
		dlTPZ      bool
		dlMono     bool
		dlOnlyMiss bool
	)
	downloadCmd := &cobra.Command{
		Use:   "download [version]",
		Short: "Download and install export templates",
		Long: `Download templates for a version. Exactly one mode is required:

  --file NAME       one or more individual catalog files (repeatable)
  --platform P      all per-file templates for a platform
  --runtime         web + linux + windows for --variant
  --all             every per-file catalog entry
  --tpz             full export-templates .tpz bundle`,
		Example: `  blazium-cli templates download nightly --file web_nothreads_release.zip
  blazium-cli templates download 0.6.748 --platform android
  blazium-cli templates download nightly --runtime --variant release
  blazium-cli templates download nightly --all
  blazium-cli templates download nightly --tpz`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := exporttemplates.ResolveDownloadMode(dlFiles, dlPlatform, dlRuntime, dlAll, dlTPZ)
			if err != nil {
				return err
			}
			verArg := ""
			if len(args) > 0 {
				verArg = args[0]
			}
			baseVersion, isNightly, err := cdn.ResolveInstallVersion(verArg, dlChannel, opts.DefaultRelease)
			if err != nil {
				return err
			}
			dest := strings.TrimSpace(dlDest)
			if dest == "" {
				dest = editorinstall.DefaultTemplatesDest()
			}

			if mode == exporttemplates.ModeTPZ {
				installed, err := exporttemplates.InstallTPZ(baseVersion, dlMono, isNightly, dest)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Templates installed to %s/%s\n", dest, installed)
				return output.Write(format(), map[string]any{
					"ok":        true,
					"mode":      string(mode),
					"version":   baseVersion,
					"channel":   cdn.ChannelName(isNightly),
					"dest":      dest,
					"installed": installed,
					"mono":      dlMono,
				})
			}

			entries, err := exporttemplates.LoadMetadata(baseVersion, isNightly)
			if err != nil {
				return err
			}
			variant := exporttemplates.VariantFromEnv()
			if dlVariant != "" {
				variant = exporttemplates.VariantFromString(dlVariant)
			}

			var selected []exporttemplates.Entry
			switch mode {
			case exporttemplates.ModeFile:
				selected = exporttemplates.Filter(entries, exporttemplates.FilterOpts{
					Files:    dlFiles,
					MonoOnly: dlMono,
				})
				if len(selected) == 0 {
					return fmt.Errorf("no catalog entries matched --file %v", dlFiles)
				}
			case exporttemplates.ModePlatform:
				selected = exporttemplates.Filter(entries, exporttemplates.FilterOpts{
					Platform: dlPlatform,
					MonoOnly: dlMono,
				})
				if len(selected) == 0 {
					return fmt.Errorf("no catalog entries for platform %q", dlPlatform)
				}
			case exporttemplates.ModeRuntime:
				selected = exporttemplates.SelectRuntime(entries, variant)
				if len(selected) == 0 {
					return fmt.Errorf("no runtime templates found for variant %s", variant)
				}
			case exporttemplates.ModeAll:
				selected = exporttemplates.Filter(entries, exporttemplates.FilterOpts{MonoOnly: dlMono})
				if len(selected) == 0 {
					return fmt.Errorf("no per-file catalog entries for %s", baseVersion)
				}
			default:
				return fmt.Errorf("unsupported mode %q", mode)
			}

			installed, err := exporttemplates.DownloadAndInstall(selected, dest, baseVersion, dlOnlyMiss)
			if err != nil {
				return err
			}
			names := installed
			if names == nil {
				names = []string{}
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Installed %d template file(s) under %s\n", len(names), dest)
			return output.Write(format(), map[string]any{
				"ok":        true,
				"mode":      string(mode),
				"version":   baseVersion,
				"channel":   cdn.ChannelName(isNightly),
				"dest":      dest,
				"variant":   variant.String(),
				"installed": names,
				"count":     len(names),
			})
		},
	}
	downloadCmd.Flags().StringVar(&dlChannel, "channel", "", "Release channel: nightly or release")
	downloadCmd.Flags().StringArrayVar(&dlFiles, "file", nil, "Download individual template filename (repeatable)")
	downloadCmd.Flags().StringVar(&dlPlatform, "platform", "", "Download all per-file templates for a platform")
	downloadCmd.Flags().BoolVar(&dlRuntime, "runtime", false, "Download web+linux+windows for --variant")
	downloadCmd.Flags().BoolVar(&dlAll, "all", false, "Download every per-file catalog entry")
	downloadCmd.Flags().BoolVar(&dlTPZ, "tpz", false, "Download the full export-templates .tpz bundle")
	downloadCmd.Flags().StringVar(&dlVariant, "variant", "", "debug or release (default: BLAZIUM_TEMPLATE_VARIANT or release)")
	downloadCmd.Flags().BoolVar(&dlMono, "mono", false, "Mono TPZ / prefer mono catalog entries")
	downloadCmd.Flags().BoolVar(&dlOnlyMiss, "only-missing", false, "Skip files already present in the templates dir")
	downloadCmd.Flags().StringVar(&dlDest, "dest", "", "Templates root (default: OS export_templates path)")

	pathCmd := &cobra.Command{
		Use:   "path",
		Short: "Print the export templates install root",
		RunE: func(cmd *cobra.Command, args []string) error {
			dest := editorinstall.DefaultTemplatesDest()
			return output.Write(format(), map[string]any{
				"path": dest,
			})
		},
	}

	templatesCmd.AddCommand(listCmd, downloadCmd, pathCmd)
	root.AddCommand(templatesCmd)
}

func matchesVariantHint(filename string, variant exporttemplates.Variant) bool {
	lower := strings.ToLower(filename)
	want := variant.String()
	other := "debug"
	if want == "debug" {
		other = "release"
	}
	if strings.Contains(lower, want) {
		return true
	}
	return !strings.Contains(lower, other)
}
