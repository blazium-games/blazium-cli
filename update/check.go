package update

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/editorinstall"
	"github.com/blazium-games/blazium-cli/hub"
	"github.com/blazium-games/blazium-cli/manifest"
	"github.com/blazium-games/blazium-cli/upgrade"
)

// ProductStatus is one row from update check.
type ProductStatus struct {
	Product         string `json:"product"`
	CurrentVersion  string `json:"current_version,omitempty"`
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	URL             string `json:"url,omitempty"`
	Filename        string `json:"filename,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	Size            int64  `json:"size,omitempty"`
	Suggestion      string `json:"suggestion,omitempty"`
	Error           string `json:"error,omitempty"`
}

// CheckOptions configures update check.
type CheckOptions struct {
	CLIVersion  string
	HubCurrent  string
	Channel     string // release|nightly for editor/templates
	InstallRoot string
}

// HTTPGet is overridable for tests.
var HTTPGet = cdn.HTTPGet

func cacheBust(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw + "?nocache=" + fmt.Sprintf("%d", time.Now().Unix())
	}
	q := u.Query()
	q.Set("nocache", fmt.Sprintf("%d", time.Now().Unix()))
	u.RawQuery = q.Encode()
	return u.String()
}

// Check runs product update checks.
func Check(products []string, opts CheckOptions) ([]ProductStatus, error) {
	if len(products) == 0 || (len(products) == 1 && products[0] == "all") {
		products = []string{"hub", "cli", "crash_reporter", "editor", "templates"}
	}
	channel := strings.TrimSpace(opts.Channel)
	if channel == "" {
		channel = "release"
	}
	out := make([]ProductStatus, 0, len(products))
	for _, p := range products {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cli":
			out = append(out, checkCLI(opts.CLIVersion))
		case "hub":
			out = append(out, checkHub(opts.HubCurrent, opts.InstallRoot))
		case "crash_reporter":
			out = append(out, checkCrashReporter(opts.InstallRoot))
		case "editor":
			out = append(out, checkEditor(channel))
		case "templates":
			out = append(out, checkTemplates(channel))
		default:
			out = append(out, ProductStatus{
				Product: p,
				Error:   fmt.Sprintf("unknown product %q", p),
			})
		}
	}
	return out, nil
}

func checkCLI(current string) ProductStatus {
	st := ProductStatus{Product: "cli", CurrentVersion: strings.TrimSpace(current)}
	plan, err := upgrade.ResolvePlan(current, "")
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.LatestVersion = plan.Version
	st.URL = plan.URL
	st.Filename = plan.Filename
	st.SHA256 = plan.SHA256
	st.Size = plan.Size
	st.UpdateAvailable = cdn.CompareSemver(plan.Version, current) > 0
	if st.UpdateAvailable {
		st.Suggestion = "blazium-cli update apply --product cli"
	}
	return st
}

func checkHub(current, installRoot string) ProductStatus {
	st := ProductStatus{Product: "hub"}
	cur := resolveHubCurrent(current, installRoot)
	st.CurrentVersion = cur
	doc, err := fetchToolManifest(hubManifestURLs())
	if err != nil {
		st.Error = err.Error()
		return st
	}
	latest := strings.TrimSpace(doc.Latest)
	st.LatestVersion = latest
	if latest == "" {
		st.Error = "hub manifest has no latest version"
		return st
	}
	dl, err := pickDownload(doc, latest, runtime.GOOS, runtimeArch())
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.URL = dl.DownloadURL
	st.Filename = dl.Filename
	st.SHA256 = dl.Sha256
	st.Size = dl.Size
	if cur == "" {
		st.UpdateAvailable = true
	} else {
		st.UpdateAvailable = cdn.CompareSemver(latest, cur) > 0
	}
	if st.UpdateAvailable {
		st.Suggestion = "blazium-cli update apply --product hub"
	}
	return st
}

func checkEditor(channel string) ProductStatus {
	st := ProductStatus{Product: "editor"}
	latest, err := cdn.ResolveLatestChannel(channel)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.LatestVersion = latest
	f, err := hub.Load()
	if err != nil {
		st.Error = err.Error()
		return st
	}
	best := ""
	for _, e := range f.Editors {
		ch := strings.ToLower(strings.TrimSpace(e.Channel))
		if ch == "" {
			ch = "release"
		}
		wantNightly := channel == "nightly"
		isNightly := ch == "nightly"
		if wantNightly != isNightly && channel != "all" {
			// Compare within requested channel when set.
			if channel == "release" && isNightly {
				continue
			}
			if channel == "nightly" && !isNightly {
				continue
			}
		}
		if best == "" || cdn.CompareSemver(e.Version, best) > 0 {
			best = e.Version
		}
	}
	st.CurrentVersion = best
	if best == "" {
		st.UpdateAvailable = true
		st.Suggestion = fmt.Sprintf("blazium-cli install %s --channel %s", latest, channel)
		return st
	}
	st.UpdateAvailable = cdn.CompareSemver(latest, best) > 0
	if st.UpdateAvailable {
		st.Suggestion = fmt.Sprintf("blazium-cli install %s --channel %s", latest, channel)
	}
	return st
}

func checkTemplates(channel string) ProductStatus {
	st := ProductStatus{Product: "templates"}
	latest, err := cdn.ResolveLatestChannel(channel)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.LatestVersion = latest
	best := newestInstalledTemplateVersion(editorinstall.DefaultTemplatesDest())
	st.CurrentVersion = best
	if best == "" {
		st.UpdateAvailable = true
		st.Suggestion = fmt.Sprintf("blazium-cli templates download %s --tpz", latest)
		return st
	}
	st.UpdateAvailable = cdn.CompareSemver(latest, best) > 0
	if st.UpdateAvailable {
		st.Suggestion = fmt.Sprintf("blazium-cli templates download %s --tpz", latest)
	}
	return st
}

func newestInstalledTemplateVersion(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	best := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// version.txt marker folders or version-like names
		if _, err := os.Stat(filepath.Join(root, name, "version.txt")); err != nil && !looksLikeVersion(name) {
			continue
		}
		ver := name
		if data, err := os.ReadFile(filepath.Join(root, name, "version.txt")); err == nil {
			if v := strings.TrimSpace(string(data)); v != "" {
				ver = v
			}
		}
		if best == "" || cdn.CompareSemver(ver, best) > 0 {
			best = ver
		}
	}
	return best
}

func looksLikeVersion(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
			continue
		}
		return false
	}
	return strings.ContainsAny(s, "0123456789")
}

func resolveHubCurrent(explicit, installRoot string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("BLAZIUM_HUB_VERSION")); v != "" {
		return v
	}
	roots := []string{}
	if r := strings.TrimSpace(installRoot); r != "" {
		roots = append(roots, r)
	}
	if r := strings.TrimSpace(os.Getenv("BLAZIUM")); r != "" {
		roots = append(roots, r)
	}
	for _, root := range roots {
		for _, rel := range []string{"VERSION", "Hub/VERSION", "bin/VERSION"} {
			p := filepath.Join(root, filepath.FromSlash(rel))
			if data, err := os.ReadFile(p); err == nil {
				if v := strings.TrimSpace(string(data)); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func runtimeArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "386":
		return "x86_32"
	default:
		return runtime.GOARCH
	}
}

func hubManifestURLs() []string {
	return []string{
		"https://cdn.blazium.app/hub/hub.json",
		"https://cdn.blazium.app/catalog/tools/hub/manifest.json",
	}
}

func fetchToolManifest(urls []string) (manifest.Document, error) {
	var last error
	for _, raw := range urls {
		body, err := HTTPGet(cacheBust(raw))
		if err != nil {
			last = err
			continue
		}
		doc, err := manifest.ParseDocument(body)
		if err != nil {
			last = err
			continue
		}
		if strings.TrimSpace(doc.Latest) == "" && len(doc.Versions) == 0 {
			last = fmt.Errorf("empty manifest at %s", raw)
			continue
		}
		return doc, nil
	}
	if last == nil {
		last = fmt.Errorf("hub manifest not found")
	}
	return manifest.Document{}, last
}

func pickDownload(doc manifest.Document, version, platform, arch string) (*manifest.Download, error) {
	entry, ok := doc.Versions[version]
	if !ok {
		return nil, fmt.Errorf("version %q not in hub manifest", version)
	}
	for i := range entry.Downloads {
		d := &entry.Downloads[i]
		if strings.EqualFold(d.Platform, platform) && archMatch(d.Arch, arch) {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no hub download for %s/%s in %s", platform, arch, version)
}

func archMatch(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == b {
		return true
	}
	if (a == "amd64" && b == "x86_64") || (a == "x86_64" && b == "amd64") {
		return true
	}
	if (a == "386" || a == "i386" || a == "x86") && (b == "x86_32" || b == "386") {
		return true
	}
	if (b == "386" || b == "i386" || b == "x86") && (a == "x86_32" || a == "386") {
		return true
	}
	return false
}
