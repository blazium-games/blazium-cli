package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"blazium-cli/hub"
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
	version := strings.TrimSpace(CLIBUILD)

	rootCmd := &cobra.Command{
		Use:     "blazium-cli <command>",
		Short:   "Blazium CLI — install editors, manage projects, remote control",
		Version: version,
		Long: `Command-line interface for Blazium editors and projects.

Hub commands:
  install, uninstall, editors, install-path, open, projects, upgrade

Remote control (running editor):
  remote status|list|exec|eval|eval-gdscript|eval-lua|config|enable|doctor

Configuration:
  Editors/projects: %APPDATA%\blazium\hub.json (Windows) or ~/.config/blazium/hub.json
  CLI prefs:        %APPDATA%\blazium\cli.json / ~/.config/blazium/cli.json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.PersistentFlags().StringVar(&format, "format", "human", "Output format: human, json, or tsv")

	hub.AddCommands(rootCmd, hub.Options{
		DefaultRelease: strings.TrimSpace(DEFAULTBUILD),
		Format:         &format,
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
