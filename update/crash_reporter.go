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

// CrashReporterPlan is a resolved Hub sidecar download.
type CrashReporterPlan struct {
	Version     string
	URL         string
	SHA256      string
	Filename    string
	Size        int64
	InstallRoot string
	DestPath    string
	Current     string
}

func crashReporterManifestURLs() []string {
	return []string{
		"https://cdn.blazium.app/crash_reporter/crash_reporter.json",
	}
}

// CrashReporterDest is the Hub sidecar path under installRoot.
func CrashReporterDest(installRoot string) string {
	root := resolveInstallRoot(installRoot)
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "Hub", "crash_reporter.exe")
	}
	return filepath.Join(root, "bin", "crash_reporter")
}

func crashReporterVersionPath(dest string) string {
	return filepath.Join(filepath.Dir(dest), "crash_reporter.version")
}

func checkCrashReporter(installRoot string) ProductStatus {
	st := ProductStatus{Product: "crash_reporter"}
	root := resolveInstallRoot(installRoot)
	dest := CrashReporterDest(root)
	doc, err := fetchToolManifest(crashReporterManifestURLs())
	if err != nil {
		st.Error = err.Error()
		return st
	}
	latest := strings.TrimSpace(doc.Latest)
	st.LatestVersion = latest
	if latest == "" {
		st.Error = "crash_reporter manifest has no latest version"
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
	cur := resolveCrashReporterCurrent(dest, doc)
	st.CurrentVersion = cur
	if cur == "" {
		st.UpdateAvailable = true
	} else {
		st.UpdateAvailable = cdn.CompareSemver(latest, cur) > 0
	}
	if st.UpdateAvailable {
		st.Suggestion = "blazium-cli update apply --product crash_reporter"
	}
	return st
}

func resolveCrashReporterCurrent(dest string, doc manifest.Document) string {
	if data, err := os.ReadFile(crashReporterVersionPath(dest)); err == nil {
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

func versionForSidecarSHA(doc manifest.Document, sha string) string {
	want := strings.ToLower(strings.TrimSpace(sha))
	if want == "" {
		return ""
	}
	for ver, entry := range doc.Versions {
		for _, d := range entry.Downloads {
			if strings.EqualFold(strings.TrimSpace(d.Sha256), want) {
				return ver
			}
		}
	}
	return ""
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ResolveCrashReporterPlan picks the Hub sidecar for this OS/arch.
func ResolveCrashReporterPlan(target, installRoot string) (CrashReporterPlan, error) {
	doc, err := fetchToolManifest(crashReporterManifestURLs())
	if err != nil {
		return CrashReporterPlan{}, err
	}
	ver := strings.TrimSpace(target)
	if ver == "" {
		ver = strings.TrimSpace(doc.Latest)
	}
	if ver == "" {
		return CrashReporterPlan{}, fmt.Errorf("crash_reporter manifest has no latest version")
	}
	dl, err := pickDownload(doc, ver, runtime.GOOS, runtimeArch())
	if err != nil {
		return CrashReporterPlan{}, err
	}
	root := resolveInstallRoot(installRoot)
	dest := CrashReporterDest(root)
	return CrashReporterPlan{
		Version:     ver,
		URL:         dl.DownloadURL,
		SHA256:      dl.Sha256,
		Filename:    dl.Filename,
		Size:        dl.Size,
		InstallRoot: root,
		DestPath:    dest,
		Current:     resolveCrashReporterCurrent(dest, doc),
	}, nil
}

// ApplyCrashReporter downloads the sidecar, verifies SHA-256, and replaces the Hub copy.
func ApplyCrashReporter(target, installRoot string) (CrashReporterPlan, error) {
	plan, err := ResolveCrashReporterPlan(target, installRoot)
	if err != nil {
		return CrashReporterPlan{}, err
	}
	if plan.Current != "" && cdn.CompareSemver(plan.Version, plan.Current) <= 0 && strings.TrimSpace(target) == "" {
		return plan, fmt.Errorf("crash_reporter already at %s (latest %s)", plan.Current, plan.Version)
	}
	if err := downloadAndReplaceCrashReporter(&plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func downloadAndReplaceCrashReporter(plan *CrashReporterPlan) error {
	tmpDir, err := os.MkdirTemp("", "blazium-crash-reporter-update-*")
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
		return fmt.Errorf("download crash_reporter: HTTP %s", resp.Status)
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
	return os.WriteFile(crashReporterVersionPath(plan.DestPath), []byte(plan.Version+"\n"), 0o644)
}
