package main

import (
	_ "embed"
	"os"
	"strings"

	"github.com/blazium-games/blazium-cli/hub"
	"github.com/blazium-games/blazium-cli/output"
	"github.com/blazium-games/blazium-cli/remote"
	upd "github.com/blazium-games/blazium-cli/update"
	"github.com/blazium-games/blazium-cli/upgrade"

	"github.com/spf13/cobra"
)

//go:embed data/cliBuild.txt
var CLIBUILD string

//go:embed data/defaultEngineBuild.txt
var DEFAULTBUILD string

func main() {
	var format string
	var quiet bool
	var jsonFlag bool
	version := strings.TrimSpace(CLIBUILD)

	rootCmd := &cobra.Command{
		Use:     "blazium-cli <command>",
		Short:   "Blazium CLI — install editors, manage projects, remote control",
		Version: version,
		Long: `Command-line interface for Blazium editors and projects.

Hub commands:
  install, uninstall, editors, install-path, templates, open, load, projects, upgrade, update

Templates:
  templates list|download|path  (individual files, platform sets, runtime, full .tpz)

Updates:
  update check|apply  (cli, hub, editor, templates check; apply cli/hub)

Info:
  version  (print CLI build version; also --version)

Remote control (running editor):
  remote status|list|exec|eval|logs|debugger|failed-run|autowork|instances|config|enable|doctor

Configuration:
  Editors/projects: %APPDATA%\blazium\hub.json (Windows) or ~/.config/blazium/hub.json
  CLI prefs:        %APPDATA%\blazium\cli.json / ~/.config/blazium/cli.json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			output.ResetErrorWritten()
			resolved, jsonQuiet := output.ResolveFormat(format, jsonFlag)
			format = resolved
			if jsonQuiet {
				quiet = true
			}
			output.Quiet = quiet
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.PersistentFlags().StringVar(&format, "format", "human", "Output format: human, json, or tsv")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Emit JSON on stdout (shorthand for --format json; implies --quiet)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress warnings and non-fatal notices")

	hub.AddCommands(rootCmd, hub.Options{
		DefaultRelease: strings.TrimSpace(DEFAULTBUILD),
		Format:         &format,
		Quiet:          &quiet,
	})
	rootCmd.AddCommand(upgrade.NewCommand(upgrade.Options{
		CurrentVersion: version,
		Format:         &format,
	}))
	rootCmd.AddCommand(upd.NewCommand(upd.Options{
		CLIVersion: version,
		Format:     &format,
	}))
	rootCmd.AddCommand(remote.NewCommand(remote.Options{
		Format: &format,
		Quiet:  &quiet,
	}))
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print blazium-cli version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return output.Write(format, map[string]any{"version": version})
		},
	})

	if err := rootCmd.Execute(); err != nil {
		if !output.ErrorWritten() {
			output.ErrorJSON(format, err)
		}
		os.Exit(1)
	}
}
