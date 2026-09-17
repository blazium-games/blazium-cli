package games

import (
	"fmt"
	"os"
	"strings"

	"github.com/blazium-games/blazium-cli/deploy/config"
	"github.com/blazium-games/blazium-cli/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const defaultAPIURL = "https://api.blazium.games/api/v1"
const defaultUploadURL = "https://upload.blazium.games/api/v1"

// Options configures games Cobra commands.
type Options struct {
	Format *string
}

func formatOf(opts Options) string {
	if opts.Format != nil {
		return *opts.Format
	}
	return "human"
}

// NewCommand returns `blazium-cli games` (chauffeur successor).
func NewCommand(opts Options) *cobra.Command {
	var access, secret, apiURL, uploadURL, deployCfg string
	cmd := &cobra.Command{
		Use:     "games",
		Aliases: []string{"chauffeur"},
		Short:   "Upload builds and files to api.blazium.games",
		Long: `Blazium Games service uploads (formerly chauffeur).

Auth: --access/--secret, blazium-deploy.yml games:, or BLAZIUM_ACCESS_TOKEN / BLAZIUM_SECRET_KEY.
The DigitalOcean Spaces plumber (games_cli/cmd/upload) is not part of this command.`,
		SilenceUsage: true,
	}
	cmd.PersistentFlags().StringVar(&access, "access", "", "Blazium access token (overrides BLAZIUM_ACCESS_TOKEN)")
	cmd.PersistentFlags().StringVar(&secret, "secret", "", "Blazium secret key (overrides BLAZIUM_SECRET_KEY)")
	cmd.PersistentFlags().StringVar(&apiURL, "url", "", "API base URL (overrides BLAZIUM_API_URL)")
	cmd.PersistentFlags().StringVar(&uploadURL, "upload", "", "Upload service base URL (overrides BLAZIUM_UPLOAD_URL)")
	cmd.PersistentFlags().StringVar(&deployCfg, "config", "", "Path to blazium-deploy.yml")

	creds := func() (token, key, url, upload string, err error) {
		var cfg *config.Config
		if strings.TrimSpace(deployCfg) != "" {
			cfg, err = config.LoadFile(deployCfg)
			if err != nil {
				return "", "", "", "", err
			}
		} else {
			cfg, _ = config.LoadOptional("")
		}
		token = access
		key = secret
		url = apiURL
		upload = uploadURL
		if cfg != nil {
			token = config.FirstNonEmpty(token, cfg.Games.AccessToken)
			key = config.FirstNonEmpty(key, cfg.Games.SecretKey)
			url = config.FirstNonEmpty(url, cfg.Games.APIURL)
			upload = config.FirstNonEmpty(upload, cfg.Games.UploadURL)
		}
		token = config.FirstNonEmpty(token, os.Getenv("BLAZIUM_ACCESS_TOKEN"))
		key = config.FirstNonEmpty(key, os.Getenv("BLAZIUM_SECRET_KEY"))
		url = config.FirstNonEmpty(url, os.Getenv("BLAZIUM_API_URL"), defaultAPIURL)
		upload = config.FirstNonEmpty(upload, os.Getenv("BLAZIUM_UPLOAD_URL"), defaultUploadURL)
		return token, key, url, upload, nil
	}

	var asset string
	var osFlag, archFlag, channelFlag string
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Upload a build using a YAML configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, key, url, upload, err := creds()
			if err != nil {
				return err
			}
			if asset == "" {
				return fmt.Errorf("--asset is required")
			}
			if token == "" || key == "" {
				return fmt.Errorf("access token and secret key are required")
			}
			client := NewClient(url, token, key)
			client.SetUploadURL(upload)
			parsed, err := ParseYAML(asset)
			if err != nil {
				return err
			}
			if parsed.Spec != "build" {
				return fmt.Errorf("invalid spec type %q, expected build", parsed.Spec)
			}
			if osFlag != "" || archFlag != "" || channelFlag != "" {
				parsed.BuildAsset.Platforms = nil
				if osFlag != "" {
					parsed.BuildAsset.OS = osFlag
				}
				if archFlag != "" {
					parsed.BuildAsset.Arch = archFlag
				}
				if channelFlag != "" {
					parsed.BuildAsset.Channel = channelFlag
				}
			}
			if err := ProcessBuild(client, parsed); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok"})
		},
	}
	buildCmd.Flags().StringVar(&asset, "asset", "", "Path to YAML asset file")
	buildCmd.Flags().StringVar(&osFlag, "os", "", "OS override")
	buildCmd.Flags().StringVar(&archFlag, "arch", "", "Arch override")
	buildCmd.Flags().StringVar(&channelFlag, "channel", "", "Channel override")
	_ = buildCmd.MarkFlagRequired("asset")

	addfilesCmd := &cobra.Command{
		Use:   "addfiles",
		Short: "Upload files using a YAML configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, key, url, upload, err := creds()
			if err != nil {
				return err
			}
			if asset == "" {
				return fmt.Errorf("--asset is required")
			}
			if token == "" || key == "" {
				return fmt.Errorf("access token and secret key are required")
			}
			client := NewClient(url, token, key)
			client.SetUploadURL(upload)
			parsed, err := ParseYAML(asset)
			if err != nil {
				return err
			}
			if parsed.Spec != "addfiles" {
				return fmt.Errorf("invalid spec type %q, expected addfiles", parsed.Spec)
			}
			if osFlag != "" {
				parsed.FilesAsset.OS = osFlag
			}
			if archFlag != "" {
				parsed.FilesAsset.Arch = archFlag
			}
			if channelFlag != "" {
				parsed.FilesAsset.Channel = channelFlag
			}
			if err := ProcessFiles(client, parsed); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok"})
		},
	}
	addfilesCmd.Flags().StringVar(&asset, "asset", "", "Path to YAML asset file")
	addfilesCmd.Flags().StringVar(&osFlag, "os", "", "OS override")
	addfilesCmd.Flags().StringVar(&archFlag, "arch", "", "Arch override")
	addfilesCmd.Flags().StringVar(&channelFlag, "channel", "", "Channel override")
	_ = addfilesCmd.MarkFlagRequired("asset")

	var imagesDir, version string
	genbuildCmd := &cobra.Command{
		Use:   "genbuild",
		Short: "Generate a new build.yml file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "" {
				version = "0.0.1"
			}
			var imagePaths []string
			if imagesDir != "" {
				images, err := ScanImageDirectory(imagesDir)
				if err != nil {
					return err
				}
				imagePaths = images
			}
			cfg := &Config{
				Version: "v1",
				Spec:    "build",
				Asset: BuildAsset{
					Title:       "Untitled Build",
					Type:        "game",
					Description: "No description provided",
					Version:     version,
					Images:      imagePaths,
					Changelog:   []ChangelogEntry{},
				},
			}
			if err := WriteYAMLFile("build.yml", cfg); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok", "path": "build.yml", "images": len(imagePaths)})
		},
	}
	genbuildCmd.Flags().StringVar(&imagesDir, "images", "", "Directory of images to scan")
	genbuildCmd.Flags().StringVar(&version, "version", "", "Version (default 0.0.1)")

	var clTitle, clDesc string
	addchangelogCmd := &cobra.Command{
		Use:   "addchangelog",
		Short: "Add a changelog entry to build.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ReadYAMLFile("build.yml")
			if err != nil {
				return err
			}
			if cfg.Spec != "build" {
				return fmt.Errorf("invalid spec type %q, expected build", cfg.Spec)
			}
			assetData, err := yaml.Marshal(cfg.Asset)
			if err != nil {
				return err
			}
			var buildAsset BuildAsset
			if err := yaml.Unmarshal(assetData, &buildAsset); err != nil {
				return err
			}
			buildAsset.Changelog = append(buildAsset.Changelog, ChangelogEntry{Title: clTitle, Description: clDesc})
			cfg.Asset = buildAsset
			if err := WriteYAMLFile("build.yml", cfg); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok"})
		},
	}
	addchangelogCmd.Flags().StringVar(&clTitle, "title", "", "Changelog title")
	addchangelogCmd.Flags().StringVar(&clDesc, "description", "", "Changelog description")
	_ = addchangelogCmd.MarkFlagRequired("title")
	_ = addchangelogCmd.MarkFlagRequired("description")

	var setVersion, setChannel, setOS, setType, setArch, filesDir string
	setfilesCmd := &cobra.Command{
		Use:   "setfiles",
		Short: "Generate a new addfiles.yml file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setVersion == "" {
				setVersion = "0.0.1"
			}
			if setChannel == "" {
				setChannel = "stable"
			}
			if setOS == "" {
				setOS = "windows"
			}
			if setType == "" {
				setType = "game"
			}
			if setArch == "" {
				setArch = "x86_64"
			}
			var fileEntries []FileEntry
			if filesDir != "" {
				files, err := ScanFileDirectory(filesDir)
				if err != nil {
					return err
				}
				for _, f := range files {
					fileEntries = append(fileEntries, FileEntry{File: f})
				}
			}
			cfg := &Config{
				Version: "v1",
				Spec:    "addfiles",
				Asset: FilesAsset{
					Type:    setType,
					Version: setVersion,
					Channel: setChannel,
					OS:      setOS,
					Arch:    setArch,
					Files:   fileEntries,
				},
			}
			if err := WriteYAMLFile("addfiles.yml", cfg); err != nil {
				return err
			}
			return output.Write(formatOf(opts), map[string]any{"status": "ok", "path": "addfiles.yml", "files": len(fileEntries)})
		},
	}
	setfilesCmd.Flags().StringVar(&setVersion, "version", "", "Version (default 0.0.1)")
	setfilesCmd.Flags().StringVar(&setChannel, "channel", "", "Channel (default stable)")
	setfilesCmd.Flags().StringVar(&setOS, "os", "", "OS (default windows)")
	setfilesCmd.Flags().StringVar(&setType, "type", "", "Type (default game)")
	setfilesCmd.Flags().StringVar(&setArch, "arch", "", "Arch (default x86_64)")
	setfilesCmd.Flags().StringVar(&filesDir, "files", "", "Directory of files to scan")

	cmd.AddCommand(buildCmd, addfilesCmd, genbuildCmd, addchangelogCmd, setfilesCmd)
	return cmd
}
