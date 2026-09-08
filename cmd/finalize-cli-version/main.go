package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/manifest"
)

func main() {
	computed := flag.String("computed", "", "Semver from game-semver-action")
	baseline := flag.String("baseline", "0.1.0", "Minimum public CLI version")
	cdnLatest := flag.String("cdn-latest", "", "cli.json latest; fetched from --manifest-url when empty")
	manifestURL := flag.String("manifest-url", manifest.CDNBaseURL+"/cli.json", "Public cli.json URL")
	skipProbe := flag.Bool("skip-probe", false, "Do not HEAD versioned CDN objects")
	flag.Parse()

	latest := strings.TrimSpace(*cdnLatest)
	if latest == "" && strings.TrimSpace(*manifestURL) != "" {
		if fetched, err := fetchLatest(*manifestURL); err != nil {
			fmt.Fprintf(os.Stderr, "warn: fetch cli.json: %v\n", err)
		} else {
			latest = fetched
		}
	}

	occupied := func(string) bool { return false }
	if !*skipProbe {
		client := &http.Client{Timeout: 15 * time.Second}
		occupied = func(version string) bool {
			return versionOccupied(client, version)
		}
	}

	version := manifest.NextPublishVersion(*computed, *baseline, latest, occupied)
	if version == "" {
		fmt.Fprintln(os.Stderr, "could not resolve a CLI publish version")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "computed=%s baseline=%s cdn_latest=%s => %s\n", *computed, *baseline, orNone(latest), version)
	fmt.Println(version)
}

func fetchLatest(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var doc struct {
		Latest string `json:"latest"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.Latest == "null" {
		return "", nil
	}
	return strings.TrimSpace(doc.Latest), nil
}

func versionOccupied(client *http.Client, version string) bool {
	for _, raw := range manifest.VersionedObjectURLs(version) {
		if urlExists(client, raw) {
			fmt.Fprintf(os.Stderr, "cdn object exists for %s: %s\n", version, raw)
			return true
		}
	}
	return false
}

func urlExists(client *http.Client, raw string) bool {
	req, err := http.NewRequest(http.MethodHead, raw, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func orNone(v string) string {
	if strings.TrimSpace(v) == "" {
		return "none"
	}
	return v
}
