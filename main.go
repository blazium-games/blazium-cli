package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed data/cliBuild.txt
var CLIBUILD string

//go:embed data/defaultEngineBuild.txt
var DEFAULTBUILD string

// EditorMetadata represents the metadata for an editor file.
type EditorMetadata struct {
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	Sha512      string `json:"sha512"`
	Sha256      string `json:"sha256"`
	Size        int    `json:"size"`
	Timestamp   string `json:"timestamp"`
}

var (
	editorMetadata []EditorMetadata
	silentMode     bool
)

func loadEditorMetadata(version string, isNightly bool) error {
	releaseType := channelName(isNightly)
	metadataURL := fmt.Sprintf("https://cdn.blazium.app/%s/%s/editors.json", releaseType, version)
	metadataContent, err := fetchRemoteJSON(metadataURL)
	if err != nil {
		return fmt.Errorf("error fetching editor metadata from %s: %w", metadataURL, err)
	}
	if err := json.Unmarshal(metadataContent, &editorMetadata); err != nil {
		return fmt.Errorf("error parsing editor metadata JSON: %w", err)
	}
	return nil
}

func fetchRemoteJSON(url string) ([]byte, error) {
	return httpGet(url)
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to download file: " + resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func buildURL(version string, isMono, isNightly bool) string {
	releaseType := channelName(isNightly)
	suffix := "_export_templates.tpz"
	if isMono {
		suffix = "_mono_export_templates.tpz"
	}
	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/Blazium_v%s%s", releaseType, version, version, suffix)
}

func validateDestinationPath(dest string) error {
	if dest == "." {
		return nil
	}
	pathRegex := `^(\.|[a-zA-Z]:\\|/)?([^<>:"|?*]+(/|\\)?)+$`
	matched, err := regexp.MatchString(pathRegex, dest)
	if err != nil {
		return fmt.Errorf("failed to validate path: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid path format: %s", dest)
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", dest)
	}
	return nil
}

func logMessage(format string, args ...interface{}) {
	if !silentMode {
		fmt.Printf(format+"\n", args...)
	}
}

func resolveChannelVersion(version string, latestNightly bool, channel string) (baseVersion string, isNightly bool, err error) {
	isNightly = channel == "nightly" || strings.Contains(version, "-nightly")
	if latestNightly || strings.EqualFold(strings.TrimSpace(version), "latest") {
		baseVersion, err = ResolveLatestNightly()
		if err != nil {
			return "", false, err
		}
		isNightly = true
		return baseVersion, isNightly, nil
	}
	if version == "" {
		version = strings.TrimSpace(DEFAULTBUILD)
	}
	if strings.Contains(version, "-nightly") {
		baseVersion = strings.Split(version, "-")[0]
		isNightly = true
		return baseVersion, isNightly, nil
	}
	if channel == "nightly" {
		baseVersion = version
		isNightly = true
		return baseVersion, isNightly, nil
	}
	return version, isNightly, nil
}

func runTemplateFileFlow(baseVersion string, isNightly bool, dest, templatesDest string, opts templateFlowOptions) error {
	all, err := loadTemplateMetadata(baseVersion, isNightly)
	if err != nil {
		return err
	}
	if opts.listOnly {
		for _, m := range all {
			fmt.Printf("%s\tplatform=%s\tarch=%s\tmono=%v\n", m.Filename, m.Platform, m.Arch, m.Mono)
		}
		return nil
	}

	filtered := filterTemplateMetadata(all, templateFilterOptions{
		files:    opts.templateFiles,
		platform: opts.templatePlatform,
		skipMono: !opts.mono && len(opts.templateFiles) == 0 && opts.templatePlatform == "",
	})
	if len(filtered) == 0 {
		return fmt.Errorf("no template files matched the requested filters")
	}

	engineVersion := templateInstallVersion(filtered, baseVersion)
	if templatesDest == "" {
		templatesDest = defaultTemplatesDest()
	}
	downloaded, err := downloadTemplateEntries(dest, filtered, templatesDest, engineVersion, opts.onlyMissing)
	if err != nil {
		return err
	}
	if opts.installFiles && len(downloaded) > 0 {
		if err := InstallTemplateFiles(templatesDest, engineVersion, downloaded); err != nil {
			return err
		}
		logMessage("Template files installed to %s/%s", templatesDest, engineVersion)
	}
	return nil
}

type templateFlowOptions struct {
	listOnly         bool
	templateFiles    []string
	templatePlatform string
	onlyMissing      bool
	installFiles     bool
	mono             bool
}

func downloadAndInstallTPZ(baseVersion string, isNightly, mono bool, dest, templatesDest string) (string, error) {
	templateURL := buildURL(baseVersion, mono, isNightly)
	templatePath := filepath.Join(dest, filepath.Base(templateURL))
	logMessage("Downloading template from %s", templateURL)
	if err := tryDownloadTPZ(templateURL, templatePath); err != nil {
		return "", err
	}
	logMessage("Template downloaded to %s", templatePath)
	if templatesDest == "" {
		templatesDest = defaultTemplatesDest()
	}
	installedVersion, err := InstallTemplatesFromTPZ(templatePath, templatesDest)
	if err != nil {
		return "", err
	}
	logMessage("Templates installed to %s/%s", templatesDest, installedVersion)
	return installedVersion, nil
}

func setupRuntimeTemplates(baseVersion string, isNightly bool, dest, templatesDest string, variant TemplateVariant) error {
	if templatesDest == "" {
		templatesDest = defaultTemplatesDest()
	}

	// Standard export templates bundle.
	if _, err := downloadAndInstallTPZ(baseVersion, isNightly, false, dest, templatesDest); err != nil {
		logMessage("Full template bundle unavailable (%v); trying individual template files", err)
		all, loadErr := loadTemplateMetadata(baseVersion, isNightly)
		if loadErr != nil {
			return fmt.Errorf("template bundle and registry both unavailable: bundle=%v registry=%w", err, loadErr)
		}
		required := selectRequiredRuntimeTemplates(all, variant)
		if len(required) == 0 {
			return fmt.Errorf("no individual template files available for version %s", baseVersion)
		}
		engineVersion := templateInstallVersion(required, baseVersion)
		downloaded, dlErr := downloadTemplateEntries(dest, required, templatesDest, engineVersion, false)
		if dlErr != nil {
			return dlErr
		}
		if err := InstallTemplateFiles(templatesDest, engineVersion, downloaded); err != nil {
			return err
		}
		logMessage("Individual runtime templates installed to %s/%s", templatesDest, engineVersion)
	}

	// Mono bundle (best effort).
	monoURL := buildURL(baseVersion, true, isNightly)
	monoPath := filepath.Join(dest, filepath.Base(monoURL))
	if err := tryDownloadTPZ(monoURL, monoPath); err != nil {
		logMessage("Mono template bundle unavailable (%v); skipping", err)
		return nil
	}
	logMessage("Mono template downloaded to %s", monoPath)
	if _, err := InstallTemplatesFromTPZ(monoPath, templatesDest); err != nil {
		return fmt.Errorf("install mono templates: %w", err)
	}
	return nil
}

func main() {
	var version, platform, arch, engineDest, templatesDest, channel, templatePlatform string
	var download, template, editor, mono, latestNightly, installTemplates, setupRuntime bool
	var listTemplates, templateOnlyMissing, installTemplateFiles bool
	var templateFiles []string
	var templateVariant string

	rootCmd := &cobra.Command{
		Use:     "blazium-cli [flags] <destination-path>",
		Short:   "Blazium CLI tool",
		Version: strings.TrimSpace(CLIBUILD),
		Long:    "Command-line interface for managing Blazium resources, including templates and editors.",
		PreRun: func(cmd *cobra.Command, args []string) {
			if listTemplates {
				return
			}
			if len(args) == 0 {
				logMessage("Missing destination path.")
				logMessage(cmd.UsageString())
				os.Exit(1)
			}
			if err := validateDestinationPath(args[len(args)-1]); err != nil {
				logMessage("Error with destination path: %v", err)
				os.Exit(1)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			if listTemplates {
				if version == "" && !latestNightly {
					latestNightly = true
				}
				if channel == "" {
					channel = "nightly"
				}
				baseVersion, isNightly, err := resolveChannelVersion(version, latestNightly, channel)
				if err != nil {
					logMessage("Error resolving version: %v", err)
					os.Exit(1)
				}
				if err := runTemplateFileFlow(baseVersion, isNightly, "", "", templateFlowOptions{listOnly: true}); err != nil {
					logMessage("Error listing templates: %v", err)
					os.Exit(1)
				}
				return
			}

			if !download {
				logMessage(cmd.UsageString())
				return
			}

			templateFileMode := len(templateFiles) > 0 || templatePlatform != "" || installTemplateFiles

			if setupRuntime {
				template = true
				editor = true
				installTemplates = true
				if engineDest == "" {
					engineDest = "/opt/blazium/blazium"
				}
				if templatesDest == "" {
					templatesDest = defaultTemplatesDest()
				}
				if platform == "" {
					platform = "linux"
				}
				if arch == "" {
					arch = "x86_64"
				}
				if channel == "" {
					channel = "nightly"
				}
				if version == "" && !latestNightly {
					latestNightly = true
				}
			}

			baseVersion, isNightly, err := resolveChannelVersion(version, latestNightly, channel)
			if err != nil {
				logMessage("Error resolving version: %v", err)
				os.Exit(1)
			}
			logMessage("Using Blazium version %s (nightly=%v)", baseVersion, isNightly)
			variant := TemplateVariantFromEnv()
			if strings.TrimSpace(templateVariant) != "" {
				variant = TemplateVariantFromString(templateVariant)
			}
			logMessage("Using template variant %s", variant)

			dest := "."
			if len(args) > 0 {
				dest = args[len(args)-1]
			}

			if templateFileMode {
				if err := runTemplateFileFlow(baseVersion, isNightly, dest, templatesDest, templateFlowOptions{
					templateFiles:    templateFiles,
					templatePlatform: templatePlatform,
					onlyMissing:      templateOnlyMissing,
					installFiles:     installTemplateFiles,
					mono:             mono,
				}); err != nil {
					logMessage("Error with template files: %v", err)
					os.Exit(1)
				}
				if !template && !editor && !installTemplates {
					return
				}
			}

			if editor {
				if err := loadEditorMetadata(baseVersion, isNightly); err != nil {
					logMessage("Error loading editor metadata: %v", err)
					os.Exit(1)
				}
			}

			var templatePath, editorZipPath string

			if template && !setupRuntime {
				templateURL := buildURL(baseVersion, mono, isNightly)
				templatePath = filepath.Join(dest, filepath.Base(templateURL))
				logMessage("Downloading template from %s", templateURL)
				if err := tryDownloadTPZ(templateURL, templatePath); err != nil {
					logMessage("Error downloading template: %v", err)
					os.Exit(1)
				}
				logMessage("Template downloaded to %s", templatePath)
			}

			if editor {
				if platform == "" || arch == "" {
					logMessage("Error: --editor requires both --platform and --arch flags.")
					os.Exit(1)
				}
				metadata, err := findEditorMetadata(baseVersion, platform, arch, mono)
				if err != nil {
					logMessage("Error finding editor metadata: %v", err)
					os.Exit(1)
				}
				editorZipPath = filepath.Join(dest, metadata.Filename)
				logMessage("Downloading editor from %s", metadata.DownloadURL)
				if err := downloadFile(metadata.DownloadURL, editorZipPath); err != nil {
					logMessage("Error downloading editor: %v", err)
					os.Exit(1)
				}
				logMessage("Editor downloaded to %s", editorZipPath)
			}

			if installTemplates {
				if setupRuntime {
					if err := setupRuntimeTemplates(baseVersion, isNightly, dest, templatesDest, variant); err != nil {
						logMessage("Error installing runtime templates: %v", err)
						os.Exit(1)
					}
				} else {
					if templatePath == "" {
						logMessage("Error: --install-templates requires --template")
						os.Exit(1)
					}
					if templatesDest == "" {
						templatesDest = defaultTemplatesDest()
					}
					installedVersion, err := InstallTemplatesFromTPZ(templatePath, templatesDest)
					if err != nil {
						logMessage("Error installing templates: %v", err)
						os.Exit(1)
					}
					logMessage("Templates installed to %s/%s", templatesDest, installedVersion)
				}
			}

			if engineDest != "" && editorZipPath != "" {
				if err := InstallEditorFromZip(editorZipPath, engineDest); err != nil {
					logMessage("Error installing editor: %v", err)
					os.Exit(1)
				}
				logMessage("Editor installed to %s", engineDest)
			}
		},
	}

	rootCmd.Flags().StringVar(&version, "get-version", "", "Version to use (default embedded build). Suffix -nightly for nightly channel.")
	rootCmd.Flags().BoolVar(&download, "download", false, "Download templates or editors.")
	rootCmd.Flags().BoolVar(&template, "template", false, "Download export templates (.tpz).")
	rootCmd.Flags().BoolVar(&editor, "editor", false, "Download the editor zip.")
	rootCmd.Flags().BoolVar(&silentMode, "silent", false, "Suppress logging output.")
	rootCmd.Flags().BoolVar(&mono, "mono", false, "Download mono variant.")
	rootCmd.Flags().StringVar(&platform, "platform", "", "Platform for editor (linux, windows, macos).")
	rootCmd.Flags().StringVar(&arch, "arch", "", "Architecture for editor (x86_64, arm64, ...).")
	rootCmd.Flags().BoolVar(&latestNightly, "latest-nightly", false, "Resolve newest version from blazium.app nightly API.")
	rootCmd.Flags().BoolVar(&installTemplates, "install-templates", false, "Extract .tpz into export_templates directory.")
	rootCmd.Flags().StringVar(&templatesDest, "templates-dest", "", "Export templates install dir (default BLAZIUM_EXPORT_TEMPLATES_DIR or ~/.local/share/blazium/export_templates).")
	rootCmd.Flags().StringVar(&engineDest, "engine-dest", "", "Install editor binary to this path after download.")
	rootCmd.Flags().StringVar(&channel, "channel", "", "Release channel: nightly or release.")
	rootCmd.Flags().BoolVar(&setupRuntime, "setup-runtime", false, "Download and install latest nightly editor + full export templates for container runtime.")
	rootCmd.Flags().BoolVar(&listTemplates, "list-templates", false, "List available per-file export templates for the resolved version.")
	rootCmd.Flags().StringArrayVar(&templateFiles, "template-file", nil, "Download specific template zip(s) by filename (repeatable).")
	rootCmd.Flags().StringVar(&templatePlatform, "template-platform", "", "Filter template downloads by platform (web, linux, windows, android, ios, macos).")
	rootCmd.Flags().BoolVar(&templateOnlyMissing, "template-only-missing", false, "Skip template files already present under --templates-dest.")
	rootCmd.Flags().BoolVar(&installTemplateFiles, "install-template-files", false, "Install downloaded template zips into export_templates (no .tpz required).")
	rootCmd.Flags().StringVar(&templateVariant, "template-variant", "", "Export template variant: debug or release (default debug, or BLAZIUM_TEMPLATE_VARIANT).")

	if err := rootCmd.Execute(); err != nil {
		logMessage("%v", err)
		os.Exit(1)
	}
}
