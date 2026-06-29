package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const nightlyVersionsURL = "https://blazium.app/api/versions/data/nightly"

type nightlyEntry struct {
	DeployType string `json:"deploy_type"`
	Version    string `json:"version"`
}

var httpGet = func(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// ResolveLatestNightly returns the highest semver from blazium.app nightly API.
func ResolveLatestNightly() (string, error) {
	body, err := httpGet(nightlyVersionsURL)
	if err != nil {
		return "", fmt.Errorf("fetch nightly versions: %w", err)
	}
	var entries []nightlyEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return "", fmt.Errorf("parse nightly versions: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no nightly versions returned")
	}
	best := entries[0].Version
	for _, e := range entries[1:] {
		if compareSemver(e.Version, best) > 0 {
			best = e.Version
		}
	}
	if strings.TrimSpace(best) == "" {
		return "", fmt.Errorf("empty nightly version")
	}
	return best, nil
}

func compareSemver(a, b string) int {
	ap := parseVersionParts(a)
	bp := parseVersionParts(b)
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
		i++
	}
	return 0
}

func parseVersionParts(v string) []int {
	v = strings.TrimSpace(strings.Split(v, "-")[0])
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

func templateShortVersion(version string) string {
	if idx := strings.Index(version, " "); idx >= 0 {
		return version[:idx]
	}
	return version
}

func channelName(isNightly bool) string {
	if isNightly {
		return "nightly"
	}
	return "release"
}
