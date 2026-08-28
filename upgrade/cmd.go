package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/blazium-games/blazium-cli/manifest"
	"github.com/blazium-games/blazium-cli/output"

	"github.com/spf13/cobra"
)

const manifestURL = "https://cdn.blazium.app/cli/cli.json"

// ManifestURL returns the CDN cli.json URL with a cache-bust query.
func ManifestURL() string {
	return fmt.Sprintf("%s?nocache=%d", manifestURL, time.Now().Unix())
}

// Options configures the upgrade command.
type Options struct {
	CurrentVersion string
	Format         *string
}

// NewCommand returns the upgrade Cobra command.
func NewCommand(opts Options) *cobra.Command {
	var target string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Update blazium-cli from the CDN manifest",
		Long: `Downloads the latest (or --target) blazium-cli build from https://cdn.blazium.app/cli/cli.json,
verifies sha256, and replaces the running binary.`,
		Example: `  blazium-cli upgrade --dry-run
  blazium-cli upgrade
  blazium-cli upgrade --target 1.2.3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format := "human"
			if opts.Format != nil {
				format = *opts.Format
			}
			plan, err := ResolvePlan(opts.CurrentVersion, target)
			if err != nil {
				return err
			}
			if dryRun {
				return output.Write(format, map[string]any{
					"dry_run":         true,
					"current_version": opts.CurrentVersion,
					"target_version":  plan.Version,
					"url":             plan.URL,
					"sha256":          plan.SHA256,
					"filename":        plan.Filename,
					"size":            plan.Size,
				})
			}
			if err := Apply(plan); err != nil {
				return err
			}
			return output.Write(format, map[string]any{
				"ok":        true,
				"previous":  opts.CurrentVersion,
				"installed": plan.Version,
				"path":      plan.DestPath,
			})
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Specific CLI version to install (default: manifest latest)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the planned download without installing")
	return cmd
}

// Plan holds a resolved upgrade download.
type Plan struct {
	Version  string
	URL      string
	SHA256   string
	Filename string
	Size     int64
	DestPath string
}

// ResolvePlan resolves which CLI artifact to install.
func ResolvePlan(current, target string) (Plan, error) {
	resp, err := http.Get(ManifestURL())
	if err != nil {
		return Plan{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Plan{}, fmt.Errorf("fetch CLI manifest: HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Plan{}, err
	}
	doc, err := manifest.ParseDocument(body)
	if err != nil {
		return Plan{}, err
	}
	ver := strings.TrimSpace(target)
	if ver == "" {
		ver = strings.TrimSpace(doc.Latest)
	}
	if ver == "" {
		return Plan{}, fmt.Errorf("manifest has no latest version")
	}
	entry, ok := doc.Versions[ver]
	if !ok {
		return Plan{}, fmt.Errorf("version %q not found in CLI manifest", ver)
	}
	platform := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	var dl *manifest.Download
	for i := range entry.Downloads {
		d := &entry.Downloads[i]
		if strings.EqualFold(d.Platform, platform) && archMatches(d.Arch, arch) {
			dl = d
			break
		}
	}
	if dl == nil {
		return Plan{}, fmt.Errorf("no CLI download for %s/%s in version %s", platform, arch, ver)
	}
	exe, err := os.Executable()
	if err != nil {
		return Plan{}, err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		Version:  ver,
		URL:      dl.DownloadURL,
		SHA256:   dl.Sha256,
		Filename: dl.Filename,
		Size:     dl.Size,
		DestPath: exe,
	}, nil
}

func archMatches(manifestArch, want string) bool {
	a := strings.ToLower(manifestArch)
	w := strings.ToLower(want)
	if a == w {
		return true
	}
	if (a == "amd64" && w == "x86_64") || (a == "x86_64" && w == "amd64") {
		return true
	}
	return false
}

// DownloadTempPath returns a writable temp file path for a CLI download.
func DownloadTempPath(version string) (string, error) {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-':
			return r
		default:
			return '-'
		}
	}, strings.TrimSpace(version))
	if safe == "" {
		safe = "cli"
	}
	f, err := os.CreateTemp("", fmt.Sprintf("blazium-cli-%s-*.download", safe))
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// IsPermissionError reports whether err is an access/permission denial.
func IsPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		if errors.Is(pe.Err, os.ErrPermission) || errors.Is(pe.Err, syscall.EACCES) || errors.Is(pe.Err, syscall.EPERM) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") || strings.Contains(msg, "permission denied")
}

// Apply downloads and replaces the current binary.
// Downloads always land in the process temp directory; when the install
// directory is not writable, the final replace is retried with elevation.
func Apply(plan Plan) error {
	tmp, err := DownloadTempPath(plan.Version)
	if err != nil {
		return err
	}
	resp, err := http.Get(plan.URL)
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(tmp)
		return fmt.Errorf("download CLI: HTTP %s", resp.Status)
	}
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	h := sha256.New()
	w := io.MultiWriter(out, h)
	if _, err := io.Copy(w, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if plan.SHA256 != "" && !strings.EqualFold(sum, plan.SHA256) {
		_ = os.Remove(tmp)
		return fmt.Errorf("sha256 mismatch: got %s want %s", sum, plan.SHA256)
	}

	if err := ReplaceExecutable(plan.DestPath, tmp); err != nil {
		if !IsPermissionError(err) {
			_ = os.Remove(tmp)
			return err
		}
		if err2 := ElevateReplace(tmp, plan.DestPath); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	_ = os.Remove(tmp)
	return nil
}

// ReplaceExecutable swaps newPath into dest (Windows: rename-aside the running exe).
// When dest does not exist yet (first install), newPath is installed in place.
// When newPath is on another volume, falls back to copy.
func ReplaceExecutable(dest, newPath string) error {
	dest = filepath.Clean(dest)
	newPath = filepath.Clean(newPath)
	if _, err := os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.Rename(newPath, dest); err != nil {
			if err2 := copyFile(newPath, dest); err2 != nil {
				return err2
			}
			_ = os.Remove(newPath)
		}
		return nil
	}
	backup := dest + ".bak." + time.Now().Format("20060102150405")
	if runtime.GOOS == "windows" {
		// On Windows, the running exe may be locked; rename aside then swap.
		_ = os.Remove(dest + ".old")
		if err := os.Rename(dest, dest+".old"); err != nil {
			if err := copyFile(newPath, dest); err != nil {
				return err
			}
			_ = os.Remove(newPath)
			return nil
		}
		if err := os.Rename(newPath, dest); err != nil {
			// Cross-volume rename fails; copy into the vacated path.
			if err2 := copyFile(newPath, dest); err2 != nil {
				_ = os.Rename(dest+".old", dest)
				return err2
			}
			_ = os.Remove(newPath)
			return nil
		}
		return nil
	}
	if err := os.Rename(dest, backup); err != nil {
		return err
	}
	if err := os.Rename(newPath, dest); err != nil {
		if err2 := copyFile(newPath, dest); err2 != nil {
			_ = os.Rename(backup, dest)
			return err2
		}
		_ = os.Remove(newPath)
		_ = os.Remove(backup)
		return nil
	}
	_ = os.Remove(backup)
	return nil
}

// ElevateReplace re-runs replace-bin with admin/root privileges.
func ElevateReplace(from, to string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	from, err = filepath.Abs(from)
	if err != nil {
		return err
	}
	to, err = filepath.Abs(to)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "windows":
		return elevateReplaceWindows(exe, from, to)
	case "linux":
		return elevateReplaceLinux(exe, from, to)
	default:
		return fmt.Errorf("cannot elevate CLI replace on %s; install to a user-writable path or re-run as root", runtime.GOOS)
	}
}

func elevateReplaceWindows(exe, from, to string) error {
	psArgs := strings.Join([]string{
		psQuote("update"),
		psQuote("replace-bin"),
		psQuote("--from"),
		psQuote(from),
		psQuote("--to"),
		psQuote(to),
	}, ",")
	script := fmt.Sprintf(
		"$p = Start-Process -FilePath %s -ArgumentList @(%s) -Verb RunAs -Wait -PassThru; if ($null -eq $p) { exit 1223 }; exit $p.ExitCode",
		psQuote(exe),
		psArgs,
	)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1223 {
			return fmt.Errorf("elevation cancelled; cannot write %s (Access is denied)", to)
		}
		return fmt.Errorf("elevated CLI replace failed: %w", err)
	}
	return nil
}

func elevateReplaceLinux(exe, from, to string) error {
	args := []string{exe, "update", "replace-bin", "--from", from, "--to", to}
	var cmd *exec.Cmd
	if _, err := exec.LookPath("pkexec"); err == nil {
		cmd = exec.Command("pkexec", args...)
	} else if _, err := exec.LookPath("sudo"); err == nil {
		cmd = exec.Command("sudo", args...)
	} else {
		return fmt.Errorf("permission denied writing %s; install pkexec/sudo or re-run as root", to)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("elevated CLI replace failed: %w", err)
	}
	return nil
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
