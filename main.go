package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"blazium-cli/hub"
	"blazium-cli/output"
	"blazium-cli/remote"
	"blazium-cli/upgrade"

	"github.com/spf13/cobra"
)

//go:embed data/cliBuild.txt
var CLIBUILD string

//go:embed data/defaultEngineBuild.txt
var DEFAULTBUILD string

func main() {
	var format string
	var quiet bool
	version := strings.TrimSpace(CLIBUILD)

	rootCmd := &cobra.Command{
		Use:     "blazium-cli <command>",
		Short:   "Blazium CLI — install editors, manage projects, remote control",
		Version: version,
		Long: `Command-line interface for Blazium editors and projects.

Hub commands:
  install, uninstall, editors, install-path, open, load, projects, upgrade

Remote control (running editor):
  remote status|list|exec|eval|instances|config|enable|doctor

Configuration:
  Editors/projects: %APPDATA%\blazium\hub.json (Windows) or ~/.config/blazium/hub.json
  CLI prefs:        %APPDATA%\blazium\cli.json / ~/.config/blazium/cli.json`,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			output.Quiet = quiet
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.PersistentFlags().StringVar(&format, "format", "human", "Output format: human, json, or tsv")
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
	rootCmd.AddCommand(remote.NewCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
