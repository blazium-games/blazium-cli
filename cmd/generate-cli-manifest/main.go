package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blazium-cli/manifest"
)

func main() {
	version := flag.String("version", "", "CLI semver to publish")
	released := flag.String("released-on", "", "RFC3339 release timestamp")
	base := flag.String("base-manifest", "", "Existing cli.json to merge")
	cerebroURL := flag.String("cerebro-manifest", "", "Optional Cerebro manifest URL")
	out := flag.String("out", "cli.json", "Output manifest path")
	artifactRoot := flag.String("artifacts", "artifacts", "Directory containing platform subdirs")
	flag.Parse()

	if *version == "" {
		fmt.Fprintln(os.Stderr, "--version is required")
		os.Exit(1)
	}

	releasedOn := time.Now().UTC()
	if strings.TrimSpace(*released) != "" {
		parsed, err := time.Parse(time.RFC3339, *released)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --released-on: %v\n", err)
			os.Exit(1)
		}
		releasedOn = parsed.UTC()
	}

	doc := manifest.Document{Versions: map[string]manifest.Version{}}
	if data, err := os.ReadFile(*base); err == nil {
		doc, err = manifest.ParseDocument(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse base manifest: %v\n", err)
			os.Exit(1)
		}
	}
	if *cerebroURL != "" {
		if remote, err := fetchManifest(*cerebroURL); err == nil {
			doc = manifest.MergeDocuments(doc, remote)
		}
	}

	builds := []manifest.BuildInput{
		{
			Platform: "linux",
			Arch:     "x86_64",
			Filename: "blazium-cli",
			Path:     filepath.Join(*artifactRoot, "linux", "blazium-cli"),
			BaseURL:  fmt.Sprintf("https://cdn.blazium.app/cli/linux/%s", *version),
			SigURL:   fmt.Sprintf("https://cdn.blazium.app/cli/linux/%s/blazium-cli.sig", *version),
			Signing:  "gpg",
		},
		{
			Platform: "windows",
			Arch:     "x86_64",
			Filename: "blazium-cli.exe",
			Path:     filepath.Join(*artifactRoot, "windows", "blazium-cli.exe"),
			BaseURL:  fmt.Sprintf("https://cdn.blazium.app/cli/windows/%s", *version),
			Signing:  "sslcom",
		},
		{
			Platform: "darwin",
			Arch:     "x86_64",
			Filename: "blazium-cli",
			Path:     filepath.Join(*artifactRoot, "darwin", "blazium-cli"),
			BaseURL:  fmt.Sprintf("https://cdn.blazium.app/cli/darwin/%s", *version),
			SigURL:   fmt.Sprintf("https://cdn.blazium.app/cli/darwin/%s/blazium-cli.sig", *version),
			Signing:  "gpg",
		},
	}
	var present []manifest.BuildInput
	for _, b := range builds {
		if _, err := os.Stat(b.Path); err == nil {
			present = append(present, b)
		}
	}
	if len(present) == 0 {
		fmt.Fprintln(os.Stderr, "no build artifacts found")
		os.Exit(1)
	}
	if err := manifest.MergeVersion(&doc, *version, releasedOn, present); err != nil {
		fmt.Fprintf(os.Stderr, "merge version: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
}

func fetchManifest(url string) (manifest.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return manifest.Document{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return manifest.Document{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return manifest.Document{}, err
	}
	return manifest.ParseDocument(body)
}
