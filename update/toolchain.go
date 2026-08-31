package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/manifest"
	"github.com/blazium-games/blazium-cli/upgrade"
)

// ToolchainPlan is a resolved Blazium Toolchain download.
type ToolchainPlan struct {
	Version     string
	URL         string
	SHA256      string
	Filename    string
	Size        int64
	InstallRoot string
	DestPath    string
	Current     string
}

func toolchainManifestURLs() []string {
	return []string{
		"https://cdn.blazium.app/toolchain/toolchain.json",
	}
}

// ToolchainDest is the toolchain manager path under installRoot.
func ToolchainDest(installRoot string) string {
	root := resolveInstallRoot(installRoot)
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "blazium-toolchain.exe")
	}
	return filepath.Join(root, "bin", "blazium-toolchain")
}

func toolchainVersionPath(dest string) string {
	return filepath.Join(filepath.Dir(dest), "blazium-toolchain.version")
}

func checkToolchain(installRoot string) ProductStatus {
	st := ProductStatus{Product: "toolchain"}
	root := resolveInstallRoot(installRoot)
	dest := ToolchainDest(root)
	doc, err := fetchToolManifest(toolchainManifestURLs())
	if err != nil {
		st.Error = err.Error()
		return st
	}
	latest := strings.TrimSpace(doc.Latest)
	st.LatestVersion = latest
	if latest == "" {
		st.Error = "toolchain manifest has no latest version"
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
	cur := resolveToolchainCurrent(dest, doc)
	st.CurrentVersion = cur
	if cur == "" {
		st.UpdateAvailable = true
	} else {
		st.UpdateAvailable = cdn.CompareSemver(latest, cur) > 0
	}
	if st.UpdateAvailable {
		st.Suggestion = "blazium-cli update apply --product toolchain"
	}
	return st
}

func resolveToolchainCurrent(dest string, doc manifest.Document) string {
	if data, err := os.ReadFile(toolchainVersionPath(dest)); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	sum, err := fileSHA256(dest)
	if err != nil {
		return ""
	}
	return versionForSidecarSHA(doc, sum)
}

// ResolveToolchainPlan picks the toolchain manager for this OS/arch.
func ResolveToolchainPlan(target, installRoot string) (ToolchainPlan, error) {
	doc, err := fetchToolManifest(toolchainManifestURLs())
	if err != nil {
		return ToolchainPlan{}, err
	}
	ver := strings.TrimSpace(target)
	if ver == "" {
		ver = strings.TrimSpace(doc.Latest)
	}
	if ver == "" {
		return ToolchainPlan{}, fmt.Errorf("toolchain manifest has no latest version")
	}
	dl, err := pickDownload(doc, ver, runtime.GOOS, runtimeArch())
	if err != nil {
		return ToolchainPlan{}, err
	}
	root := resolveInstallRoot(installRoot)
	dest := ToolchainDest(root)
	return ToolchainPlan{
		Version:     ver,
		URL:         dl.DownloadURL,
		SHA256:      dl.Sha256,
		Filename:    dl.Filename,
		Size:        dl.Size,
		InstallRoot: root,
		DestPath:    dest,
		Current:     resolveToolchainCurrent(dest, doc),
	}, nil
}

// ApplyToolchain downloads the manager, verifies SHA-256, and replaces the install copy.
func ApplyToolchain(target, installRoot string) (ToolchainPlan, error) {
	plan, err := ResolveToolchainPlan(target, installRoot)
	if err != nil {
		return ToolchainPlan{}, err
	}
	if plan.Current != "" && cdn.CompareSemver(plan.Version, plan.Current) <= 0 && strings.TrimSpace(target) == "" {
		return plan, fmt.Errorf("toolchain already at %s (latest %s)", plan.Current, plan.Version)
	}
	if err := downloadAndReplaceToolchain(&plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func downloadAndReplaceToolchain(plan *ToolchainPlan) error {
	tmpDir, err := os.MkdirTemp("", "blazium-toolchain-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	name := plan.Filename
	if name == "" {
		name = filepath.Base(plan.URL)
	}
	tmp := filepath.Join(tmpDir, name)
	resp, err := http.Get(plan.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download toolchain: HTTP %s", resp.Status)
	}
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	h := sha256.New()
	w := io.MultiWriter(out, h)
	if _, err := io.Copy(w, resp.Body); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if plan.SHA256 != "" && !strings.EqualFold(sum, plan.SHA256) {
		return fmt.Errorf("sha256 mismatch: got %s want %s", sum, plan.SHA256)
	}
	if err := os.MkdirAll(filepath.Dir(plan.DestPath), 0o755); err != nil {
		return err
	}
	if err := upgrade.ReplaceExecutable(plan.DestPath, tmp); err != nil {
		if !upgrade.IsPermissionError(err) {
			return err
		}
		if err2 := upgrade.ElevateReplace(tmp, plan.DestPath); err2 != nil {
			return err2
		}
	}
	return os.WriteFile(toolchainVersionPath(plan.DestPath), []byte(plan.Version+"\n"), 0o644)
}
