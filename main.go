package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed data/cliBuild.txt
var CLIBUILD string

//go:embed data/defaultEngineBuild.txt
var DEFAULTBUILD string

func main() {
	rootCmd := &cobra.Command{
		Use:     "blazium-cli [flags] .",
		Short:   "Blazium CLI tool",
		Long:    "Blazium CLI help\n\nCommand-line interface for managing Blazium resources, including templates and editors.",
		Version: CLIBUILD,
	}

	rootCmd.AddCommand(NewDownloadCommand())
	rootCmd.AddCommand(NewLobbyCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
