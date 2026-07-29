package upgrade

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
	"time"

	"blazium-cli/manifest"
	"blazium-cli/output"

	"github.com/spf13/cobra"
)

const manifestURL = "https://cdn.blazium.app/cli/cli.json"

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
				"ok":              true,
				"previous":        opts.CurrentVersion,
				"installed":       plan.Version,
				"path":            plan.DestPath,
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
	resp, err := http.Get(manifestURL)
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

// Apply downloads and replaces the current binary.
func Apply(plan Plan) error {
	tmp := plan.DestPath + ".new"
	resp, err := http.Get(plan.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download CLI: HTTP %s", resp.Status)
	}
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
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
	return replaceExecutable(plan.DestPath, tmp)
}

func replaceExecutable(dest, newPath string) error {
	backup := dest + ".bak." + time.Now().Format("20060102150405")
	if runtime.GOOS == "windows" {
		// On Windows, the running exe may be locked; rename aside then swap.
		_ = os.Remove(dest + ".old")
		if err := os.Rename(dest, dest+".old"); err != nil {
			// Fall back: copy over if rename fails
			if err := copyFile(newPath, dest); err != nil {
				return err
			}
			_ = os.Remove(newPath)
			return nil
		}
		if err := os.Rename(newPath, dest); err != nil {
			_ = os.Rename(dest+".old", dest)
			return err
		}
		return nil
	}
	if err := os.Rename(dest, backup); err != nil {
		return err
	}
	if err := os.Rename(newPath, dest); err != nil {
		_ = os.Rename(backup, dest)
		return err
	}
	_ = os.Remove(backup)
	return nil
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
