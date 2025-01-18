package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	silentMode bool = false
)

func init() {}

func loadEditorMetadata(version string, isNightly bool) error {
	releaseType := "release"
	if isNightly {
		releaseType = "nightly"
	}

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
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch JSON: " + resp.Status)
	}

	return io.ReadAll(resp.Body)
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

	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func buildURL(version string, isMono, isNightly bool) string {
	releaseType := "release"
	if isNightly {
		releaseType = "nightly"
	}

	suffix := "_export_templates.tpz"
	if isMono {
		suffix = "_mono_export_templates.tpz"
	}

	return fmt.Sprintf("https://cdn.blazium.app/%s/%s/Blazium_v%s%s", releaseType, version, version, suffix)
}

func findEditorMetadata(version, platform, arch string, isMono bool) (*EditorMetadata, error) {
	for _, metadata := range editorMetadata {
		if strings.Contains(metadata.Filename, version) &&
			strings.Contains(metadata.Filename, platform) &&
			strings.Contains(metadata.Filename, arch) &&
			(isMono == strings.Contains(metadata.Filename, ".mono")) {
			return &metadata, nil
		}
	}
	return nil, errors.New("no matching editor metadata found")
}


func validateDestinationPath(dest string) error {
	// Check if the destination is "." (current directory)
	if dest == "." {
		return nil
	}
	log.Print(dest)

	// Validate the destination path using regex for Windows and Linux paths
	pathRegex := `^(\.|[a-zA-Z]:\\|/)?([^<>:"|?*]+(/|\\)?)+$`
	matched, err := regexp.MatchString(pathRegex, dest)
	if err != nil {
		return fmt.Errorf("failed to validate path: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid path format: %s", dest)
	}

	// Check if the path exists
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

func main() {
	var version string
	var download, template, editor, mono bool
	var platform, arch string

	rootCmd := &cobra.Command{
		Use:   "blazium-cli [flags] .",
		Short: "Blazium CLI tool",
		Version: CLIBUILD,
		Long:  "Command-line interface for managing Blazium resources, including templates and editors.",
		PreRun: func(cmd *cobra.Command, args []string) {
		  if len(args) > 0 {
			  dest := args[len(args)-1]
			  if err := validateDestinationPath(dest); err != nil {
				  logMessage("Error with destination path: %v", err)
				  os.Exit(1)
			  }
		  } else {
			logMessage("Missing Destination path.")
			logMessage(cmd.UsageString())
			os.Exit(1)
		  }
		},
		Run: func(cmd *cobra.Command, args []string) {
			if !download {
				logMessage(cmd.UsageString())
				return
			}

			if version == "" {
				version = DEFAULTBUILD
			}

			isNightly := strings.Contains(version, "-nightly")
			baseVersion := version
			if isNightly {
				baseVersion = strings.Split(version, "-")[0]
			}
			if err := loadEditorMetadata(baseVersion, isNightly); err != nil {
				logMessage("Error loading editor metadata: %v", err)
				os.Exit(1)
			}

			dest := "."
			if len(args) > 0 {
				dest = args[len(args)-1]
				if err := validateDestinationPath(dest); err != nil {
					logMessage("Error with destination path: %v", err)
					os.Exit(1)
				}
			} else {
				logMessage("Missing Destination path.")
				logMessage(cmd.UsageString())
				os.Exit(1)
			}

			if template {
				templateURL := buildURL(baseVersion, mono, isNightly)
				templateDest := filepath.Join(dest, filepath.Base(templateURL))
				logMessage("Downloading template from %s", templateURL)
				if err := downloadFile(templateURL, templateDest); err != nil {
					logMessage("Error downloading template: %v", err)
					os.Exit(1)
				}
				logMessage("Template downloaded to %s", templateDest)
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

				editorDest := filepath.Join(dest, metadata.Filename)
				logMessage("Downloading editor from %s", metadata.DownloadURL)
				if err := downloadFile(metadata.DownloadURL, editorDest); err != nil {
					logMessage("Error downloading editor: %v", err)
					os.Exit(1)
				}
				logMessage("Editor downloaded to %s", editorDest)
			}
		},
	}

	rootCmd.Flags().StringVar(&version, "get-version", "", "Specify the version to use (default to (" + DEFAULTBUILD+ ") if not set).")
	rootCmd.Flags().BoolVar(&download, "download", false, "Download templates or editors.")
	rootCmd.Flags().BoolVar(&template, "template", false, "Download the export template.")
	rootCmd.Flags().BoolVar(&editor, "editor", false, "Download the editor.")
	rootCmd.Flags().BoolVar(&silentMode, "silent", false, "Suppress all logging output.")
	rootCmd.Flags().BoolVar(&mono, "mono", false, "Download the mono version.")
	rootCmd.Flags().StringVar(&platform, "platform", "", "Specify the platform (e.g., linux, windows, macos).")
	rootCmd.Flags().StringVar(&arch, "arch", "", "Specify the architecture (e.g., x86_64, arm64, arm32, 32bit, 64bit).")

	if err := rootCmd.Execute(); err != nil {
		logMessage("%v", err)
		os.Exit(1)
	}
}
