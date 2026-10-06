package hub

import (
	"fmt"

	"github.com/blazium-games/blazium-cli/output"

	"github.com/spf13/cobra"
)

func addStartersCommands(root *cobra.Command, format func() string) {
	startersCmd := &cobra.Command{
		Use:   "starters",
		Short: "List and download Blazium project starters",
		Long: `Work with project starters from https://cdn.blazium.app/starters/starters.json.

These are game projects. Export templates stay on the templates command.
Each catalog entry includes github and git so a failed unzip can be cloned.`,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List project starters",
		Example: `  blazium-cli starters list
  blazium-cli starters list --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := LoadStartersCatalog(StartersCatalogURL)
			if err != nil {
				return err
			}
			return writeStarters(format(), catalog)
		},
	}

	var dest string
	downloadCmd := &cobra.Command{
		Use:     "download <name>",
		Short:   "Download a project starter into a directory",
		Example: `  blazium-cli starters download template_2d_empty -dir D:\games\my_game`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := LoadStartersCatalog(StartersCatalogURL)
			if err != nil {
				return err
			}
			starter, err := FindStarter(catalog, args[0])
			if err != nil {
				return err
			}
			if err := DownloadStarter(starter, dest); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Starter %s unpacked to %s\n", starter.Name, dest)
			return output.Write(format(), map[string]any{
				"ok":          true,
				"name":        starter.Name,
				"dir":         dest,
				"description": starter.Description,
				"github":      starter.GitHub,
				"git":         starter.Git,
				"commit":      starter.Commit,
				"file":        starter.File,
				"sha256":      starter.SHA256,
			})
		},
	}
	downloadCmd.Flags().StringVar(&dest, "dir", "", "Directory to unpack the starter into")
	if err := downloadCmd.MarkFlagRequired("dir"); err != nil {
		panic(err)
	}

	startersCmd.AddCommand(listCmd, downloadCmd)
	root.AddCommand(startersCmd)
}

func writeStarters(format string, catalog StartersCatalog) error {
	if format == "" || format == "human" {
		for _, starter := range catalog.Starters {
			fmt.Printf("%s\t%s\t%s\n", starter.Name, starter.Description, starter.GitHub)
		}
		return nil
	}
	rows := make([]any, 0, len(catalog.Starters))
	for _, starter := range catalog.Starters {
		rows = append(rows, map[string]any{
			"name":        starter.Name,
			"description": starter.Description,
			"github":      starter.GitHub,
			"git":         starter.Git,
			"commit":      starter.Commit,
			"file":        starter.File,
			"sha256":      starter.SHA256,
			"repo":        starter.Repo,
		})
	}
	return output.Write(format, map[string]any{
		"latest":   catalog.Latest,
		"starters": rows,
	})
}
