package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/manifest"
)

// HubPlan is a resolved Hub installer download.
type HubPlan struct {
	Version       string
	URL           string
	SHA256        string
	Filename      string
	Size          int64
	InstallRoot   string
	Current       string
	InstallerPath string
	Launch        bool // Windows: pass Inno /LAUNCH when true
}

// ResolveHubPlan picks the Hub installer for this OS/arch.
func ResolveHubPlan(current, target, installRoot string) (HubPlan, error) {
	doc, err := fetchToolManifest(hubManifestURLs())
	if err != nil {
		return HubPlan{}, err
	}
	ver := strings.TrimSpace(target)
	if ver == "" {
		ver = strings.TrimSpace(doc.Latest)
	}
	if ver == "" {
		return HubPlan{}, fmt.Errorf("hub manifest has no latest version")
	}
	dl, err := pickDownload(doc, ver, runtime.GOOS, runtimeArch())
	if err != nil {
		return HubPlan{}, err
	}
	root := resolveInstallRoot(installRoot)
	cur := resolveHubCurrent(current, root)
	return HubPlan{
		Version:     ver,
		URL:         dl.DownloadURL,
		SHA256:      dl.Sha256,
		Filename:    dl.Filename,
		Size:        dl.Size,
		InstallRoot: root,
		Current:     cur,
	}, nil
}

func resolveInstallRoot(explicit string) string {
	if r := strings.TrimSpace(explicit); r != "" {
		return r
	}
	if r := strings.TrimSpace(os.Getenv("BLAZIUM")); r != "" {
		return r
	}
	if runtime.GOOS == "windows" {
		pf := os.Getenv("ProgramFiles")
		if pf == "" {
			pf = `C:\Program Files`
		}
		return filepath.Join(pf, "Blazium")
	}
	return "/opt/blazium"
}

// DownloadHubInstaller fetches and verifies the installer into a temp file.
func DownloadHubInstaller(plan *HubPlan) error {
	tmpDir, err := os.MkdirTemp("", "blazium-hub-update-*")
	if err != nil {
		return err
	}
	name := plan.Filename
	if name == "" {
		name = filepath.Base(plan.URL)
	}
	dest := filepath.Join(tmpDir, name)
	resp, err := http.Get(plan.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download hub installer: HTTP %s", resp.Status)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	h := sha256.New()
	w := io.MultiWriter(out, h)
	if _, err := io.Copy(w, resp.Body); err != nil {
		_ = out.Close()
		_ = os.RemoveAll(tmpDir)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if plan.SHA256 != "" && !strings.EqualFold(sum, plan.SHA256) {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("sha256 mismatch: got %s want %s", sum, plan.SHA256)
	}
	plan.InstallerPath = dest
	return nil
}

// WindowsHubInstallerArgs returns Inno Setup flags for a silent Hub install.
// When launch is true, appends /LAUNCH so Hub starts after install.
func WindowsHubInstallerArgs(installRoot string, launch bool) []string {
	args := []string{
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/SP-",
		"/DIR=" + installRoot,
	}
	if launch {
		args = append(args, "/LAUNCH")
	}
	return args
}

// LaunchHubInstaller starts the platform installer (detached where possible).
func LaunchHubInstaller(plan HubPlan) error {
	if plan.InstallerPath == "" {
		return fmt.Errorf("installer path empty; download first")
	}
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(plan.InstallerPath, WindowsHubInstallerArgs(plan.InstallRoot, plan.Launch)...)
		return cmd.Start()
	case "linux":
		return launchLinuxDeb(plan.InstallerPath)
	default:
		return fmt.Errorf("hub installer apply not supported on %s", runtime.GOOS)
	}
}

func launchLinuxDeb(deb string) error {
	if os.Geteuid() == 0 {
		cmd := exec.Command("dpkg", "-i", deb)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	}
	if _, err := exec.LookPath("pkexec"); err == nil {
		cmd := exec.Command("pkexec", "dpkg", "-i", deb)
		return cmd.Start()
	}
	cmd := exec.Command("sudo", "dpkg", "-i", deb)
	return cmd.Start()
}

// ApplyHub downloads and launches the Hub installer.
// When launch is true on Windows, the installer is started with /LAUNCH.
func ApplyHub(current, target, installRoot string, launch bool) (HubPlan, error) {
	plan, err := ResolveHubPlan(current, target, installRoot)
	if err != nil {
		return HubPlan{}, err
	}
	plan.Launch = launch
	if plan.Current != "" && cdn.CompareSemver(plan.Version, plan.Current) <= 0 && strings.TrimSpace(target) == "" {
		return plan, fmt.Errorf("hub already at %s (latest %s)", plan.Current, plan.Version)
	}
	if err := DownloadHubInstaller(&plan); err != nil {
		return plan, err
	}
	if err := LaunchHubInstaller(plan); err != nil {
		return plan, err
	}
	return plan, nil
}

// ParseHubManifest is exported for tests.
func ParseHubManifest(body []byte) (manifest.Document, error) {
	return manifest.ParseDocument(body)
}
