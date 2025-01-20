package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type VersionInfo struct {
	DeployType   string `json:"deploy_type"`
	Version      string `json:"version"`
	ChangelogURL string `json:"changelog_url"`
	VersionURL   string `json:"version_url"`
}

func getLatestVersion(url string) (string, error) {
	// Fetch the JSON data from the given URL
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the JSON data
	var versions []VersionInfo
	if err := json.Unmarshal(body, &versions); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Find the latest version
	latestVersion := ""
	for _, v := range versions {
		if v.Version > latestVersion {
			latestVersion = v.Version
		}
	}

	if latestVersion == "" {
		return "", fmt.Errorf("no versions found")
	}

	return latestVersion, nil
}

func main() {
	url := "https://blazium.app/api/versions/data/nightly"
	latestVersion, err := getLatestVersion(url)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Latest version:", latestVersion)
}
