package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

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

func NewDownloadCommand() *cobra.Command {
	var version string
	var template, editor, mono bool
	var platform, arch string
	var silentMode bool

	dlCmd := &cobra.Command{
		Use:   "download [template|editor] [--get-version <version>] [--silent] [--mono] [--platform] [--arch] <destination>",
		Short: "Download templates or editors",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			item := args[0]
			fmt.Println("Downloading", item)
			dest := args[len(args)-1]
			switch item {
			case "template":
				template = true
				editor = false
			case "editor":
				editor = true
				template = false
			default:
				fmt.Println("First argument must be 'template' or 'editor'")
				os.Exit(1)
			}
			if version == "" {
				version = DEFAULTBUILD
			}
			isNightly := strings.Contains(version, "-nightly")
			baseVersion := version
			if isNightly {
				baseVersion = strings.Split(version, "-")[0]
			}
			if (platform == "" || arch == "") && editor {
				fmt.Println("Error: --editor requires both --platform and --arch flags.")
				os.Exit(1)
			}
			if err := loadEditorMetadata(baseVersion, isNightly); err != nil {
				fmt.Printf("Error loading editor metadata: %v\n", err)
				os.Exit(1)
			}
			if template {
				templateURL := buildURL(baseVersion, mono, isNightly)
				templateDest := filepath.Join(dest, filepath.Base(templateURL))
				if !silentMode {
					fmt.Printf("Downloading template from %s\n", templateURL)
				}
				if err := downloadFile(templateURL, templateDest); err != nil {
					fmt.Printf("Error downloading template: %v\n", err)
					os.Exit(1)
				}
				if !silentMode {
					fmt.Printf("Template downloaded to %s\n", templateDest)
				}
			}
			if editor {
				// Only proceed if both platform and arch are set
				metadata, err := findEditorMetadata(baseVersion, platform, arch, mono)
				if err != nil {
					fmt.Printf("Error finding editor metadata: %v\n", err)
					os.Exit(1)
				}
				editorDest := filepath.Join(dest, metadata.Filename)
				if !silentMode {
					fmt.Printf("Downloading editor from %s\n", metadata.DownloadURL)
				}
				if err := downloadFile(metadata.DownloadURL, editorDest); err != nil {
					fmt.Printf("Error downloading editor: %v\n", err)
					os.Exit(1)
				}
				if !silentMode {
					fmt.Printf("Editor downloaded to %s\n", editorDest)
				}
			}
		},
	}
	dlCmd.Flags().StringVar(&version, "get-version", "", "Specify the version to use (default to ("+DEFAULTBUILD+") if not set).")
	dlCmd.Flags().BoolVar(&silentMode, "silent", false, "Suppress all logging output.")
	dlCmd.Flags().BoolVar(&mono, "mono", false, "Download the mono version.")
	dlCmd.Flags().StringVar(&platform, "platform", "", "Specify the platform (e.g., linux, windows, macos).")
	dlCmd.Flags().StringVar(&arch, "arch", "", "Specify the architecture (e.g., x86_64, arm64, arm32, 32bit, 64bit).")
	return dlCmd
}
