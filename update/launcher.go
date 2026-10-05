package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
)

// LauncherPlan is a resolved BlaziumLauncher installer download.
type LauncherPlan struct {
	Version       string
	URL           string
	SHA256        string
	Filename      string
	Size          int64
	InstallRoot   string
	Current       string
	InstallerPath string
	Launch        bool
}

func launcherManifestURLs() []string {
	return []string{
		"https://cdn.blazium.app/launcher/launcher.json",
		"https://github.com/blazium-games/games_launcher/releases/latest/download/launcher.json",
	}
}

// ResolveLauncherPlan picks the BlaziumLauncher installer for this OS/arch.
// installRoot is the Games directory, not the shared Blazium root.
func ResolveLauncherPlan(current, target, installRoot string) (LauncherPlan, error) {
	doc, err := fetchToolManifest(launcherManifestURLs())
	if err != nil {
		return LauncherPlan{}, err
	}
	ver := strings.TrimSpace(target)
	if ver == "" {
		ver = strings.TrimSpace(doc.Latest)
	}
	if ver == "" {
		return LauncherPlan{}, fmt.Errorf("launcher manifest has no latest version")
	}
	dl, err := pickDownload(doc, ver, runtime.GOOS, runtimeArch())
	if err != nil {
		return LauncherPlan{}, err
	}
	root := resolveLauncherInstallRoot(installRoot)
	cur := resolveLauncherCurrent(current, root)
	return LauncherPlan{
		Version:     ver,
		URL:         dl.DownloadURL,
		SHA256:      dl.Sha256,
		Filename:    dl.Filename,
		Size:        dl.Size,
		InstallRoot: root,
		Current:     cur,
	}, nil
}

func resolveLauncherInstallRoot(explicit string) string {
	if r := strings.TrimSpace(explicit); r != "" {
		return r
	}
	if runtime.GOOS == "windows" {
		base := strings.TrimSpace(os.Getenv("BLAZIUM"))
		if base == "" {
			pf := os.Getenv("ProgramFiles")
			if pf == "" {
				pf = `C:\Program Files`
			}
			base = filepath.Join(pf, "Blazium")
		}
		return filepath.Join(base, "Games")
	}
	base := strings.TrimSpace(os.Getenv("BLAZIUM"))
	if base == "" {
		base = "/opt/blazium"
	}
	return filepath.Join(base, "games")
}

func resolveLauncherCurrent(explicit, installRoot string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("BLAZIUM_LAUNCHER_VERSION")); v != "" {
		return v
	}
	root := strings.TrimSpace(installRoot)
	if root == "" {
		root = resolveLauncherInstallRoot("")
	}
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WindowsLauncherInstallerArgs returns Inno Setup flags for a silent launcher install.
// /NOCLI leaves an existing blazium-cli protocol handler alone.
// /DIR is the Games directory. When launch is true, appends /LAUNCH.
func WindowsLauncherInstallerArgs(installRoot string, launch bool) []string {
	args := []string{
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/SP-",
		"/NOCLI",
		"/DIR=" + installRoot,
	}
	if launch {
		args = append(args, "/LAUNCH")
	}
	return args
}

// LaunchLauncherInstaller starts the platform installer.
func LaunchLauncherInstaller(plan LauncherPlan) error {
	if plan.InstallerPath == "" {
		return fmt.Errorf("installer path empty; download first")
	}
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(plan.InstallerPath, WindowsLauncherInstallerArgs(plan.InstallRoot, plan.Launch)...)
		return cmd.Start()
	case "linux":
		return launchLinuxDeb(plan.InstallerPath)
	default:
		return fmt.Errorf("launcher installer apply not supported on %s", runtime.GOOS)
	}
}

// ApplyLauncher downloads and launches the BlaziumLauncher installer.
func ApplyLauncher(current, target, installRoot string, launch bool) (LauncherPlan, error) {
	plan, err := ResolveLauncherPlan(current, target, installRoot)
	if err != nil {
		return LauncherPlan{}, err
	}
	plan.Launch = launch
	if plan.Current != "" && cdn.CompareSemver(plan.Version, plan.Current) <= 0 && strings.TrimSpace(target) == "" {
		return plan, fmt.Errorf("launcher already at %s (latest %s)", plan.Current, plan.Version)
	}
	hubPlan := HubPlan{
		Version:     plan.Version,
		URL:         plan.URL,
		SHA256:      plan.SHA256,
		Filename:    plan.Filename,
		Size:        plan.Size,
		InstallRoot: plan.InstallRoot,
	}
	if err := DownloadHubInstaller(&hubPlan); err != nil {
		return plan, err
	}
	plan.InstallerPath = hubPlan.InstallerPath
	if err := LaunchLauncherInstaller(plan); err != nil {
		return plan, err
	}
	return plan, nil
}
