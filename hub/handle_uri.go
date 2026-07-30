package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/editorinstall"
	"github.com/blazium-games/blazium-cli/exporttemplates"
	"github.com/blazium-games/blazium-cli/remote"
)

// InstallParams configures editor installation from install command or handle-uri.
type InstallParams struct {
	VersionArg     string
	Channel        string
	Platform       string
	Arch           string
	Mono           bool
	WithTemplates  bool
	DefaultRelease string
}

// RunInstall downloads and registers a Blazium editor.
func RunInstall(p InstallParams) (map[string]any, error) {
	baseVersion, isNightly, err := cdn.ResolveInstallVersion(p.VersionArg, p.Channel, p.DefaultRelease)
	if err != nil {
		return nil, err
	}
	platform := p.Platform
	arch := p.Arch
	if platform == "" || arch == "" {
		dp, da := cdn.DefaultPlatformArch()
		if platform == "" {
			platform = dp
		}
		if arch == "" {
			arch = da
		}
	}
	list, err := cdn.LoadEditorMetadata(baseVersion, isNightly)
	if err != nil {
		return nil, err
	}
	meta, err := cdn.FindEditorMetadata(list, baseVersion, platform, arch, p.Mono)
	if err != nil {
		return nil, err
	}

	f, err := Load()
	if err != nil {
		return nil, err
	}
	installRoot, err := f.EffectiveInstallPath()
	if err != nil {
		return nil, err
	}
	destDir := filepath.Join(installRoot, baseVersion)
	tmpZip := filepath.Join(os.TempDir(), meta.Filename)
	fmt.Fprintf(os.Stderr, "Downloading %s\n", meta.DownloadURL)
	if err := cdn.DownloadFile(meta.DownloadURL, tmpZip); err != nil {
		return nil, err
	}
	defer os.Remove(tmpZip)

	fmt.Fprintf(os.Stderr, "Installing to %s\n", destDir)
	bin, err := editorinstall.InstallEditorTree(tmpZip, destDir)
	if err != nil {
		return nil, err
	}
	ed := Editor{
		Version:  baseVersion,
		Path:     bin,
		Dir:      destDir,
		Platform: platform,
		Arch:     arch,
		Mono:     p.Mono,
		Channel:  InferChannel(baseVersion, isNightly),
	}
	f.UpsertEditor(ed)
	if err := Save(f); err != nil {
		return nil, err
	}

	if p.WithTemplates {
		tplDest := editorinstall.DefaultTemplatesDest()
		installed, err := exporttemplates.InstallTPZ(baseVersion, p.Mono, isNightly, tplDest)
		if err != nil {
			return nil, fmt.Errorf("templates: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Templates installed to %s/%s\n", tplDest, installed)
	}

	resolved, _ := f.ResolveDefault()
	resolvedVer := ""
	if resolved != nil {
		resolvedVer = resolved.Version
	}
	return map[string]any{
		"ok":       true,
		"action":   "install",
		"version":  ed.Version,
		"channel":  ed.Channel,
		"path":     ed.Path,
		"dir":      ed.Dir,
		"default":  resolvedVer,
		"platform": ed.Platform,
		"arch":     ed.Arch,
	}, nil
}

// LaunchProject resolves the project/editor and launches the editor (open or load profile).
func LaunchProject(projectArg string, fullProfile bool, quiet bool) (map[string]any, error) {
	f, err := Load()
	if err != nil {
		return nil, err
	}
	projectPath, err := f.ResolveProjectPath(projectArg)
	if err != nil {
		return nil, err
	}
	profile, err := LoadProjectProfile(projectPath)
	if err != nil {
		return nil, err
	}
	ed, reason, err := f.ResolveEditorForProject(projectPath)
	if err != nil {
		return nil, err
	}
	if err := f.TouchProjectLastOpened(projectPath); err != nil {
		return nil, err
	}
	if err := Save(f); err != nil {
		return nil, err
	}
	result, err := LaunchEditor(LaunchOptions{
		EditorPath:    ed.Path,
		EditorVersion: ed.Version,
		ProjectPath:   projectPath,
		Profile:       profile,
		Quiet:         quiet,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"ok":             true,
		"project":        projectPath,
		"editor":         ed.Version,
		"editor_path":    ed.Path,
		"resolve_reason": reason,
		"pid":            result.Instance.PID,
		"launched":       true,
	}
	if result.RemoteEnabled {
		out["instance_id"] = result.Instance.ID
		out["remote_control"] = map[string]any{
			"host":  result.Instance.Host,
			"port":  result.Instance.RemotePort,
			"token": result.Instance.RemoteToken,
			"bound": result.Instance.Bound,
		}
	}
	if result.MCPEnabled {
		out["justamcp"] = map[string]any{
			"enabled": true,
			"port":    result.Instance.MCPPort,
		}
	}
	if fullProfile {
		out["profile"] = map[string]any{
			"name":           profile.Name,
			"features":       profile.Features,
			"editor_version": profile.EditorVersion,
			"justamcp":       profile.JustAMCP,
			"remote_control": profile.RemoteControl,
		}
	}
	return out, nil
}

var focusCommandCandidates = []string{
	"focus",
	"focus_window",
	"bring_to_front",
	"window_focus",
}

// HandleURIOptions configures deep-link handling.
type HandleURIOptions struct {
	URI            string
	DefaultRelease string
	Quiet          bool
}

// HandleURI parses and executes a blazium:// deep link.
func HandleURI(opts HandleURIOptions) (map[string]any, error) {
	parsed, err := ParseBlaziumURI(opts.URI)
	if err != nil {
		return nil, err
	}

	base := map[string]any{
		"ok":     true,
		"action": parsed.Action,
		"uri":    parsed.Raw,
	}

	switch parsed.Action {
	case "hub":
		return base, nil

	case "install":
		out, err := RunInstall(InstallParams{
			VersionArg:     parsed.Version,
			Channel:        parsed.Channel,
			DefaultRelease: opts.DefaultRelease,
		})
		if err != nil {
			return nil, err
		}
		out["uri"] = parsed.Raw
		return out, nil

	case "open", "load":
		return handleURIProject(opts, parsed)
	default:
		return nil, fmt.Errorf("unsupported action %q", parsed.Action)
	}
}

func handleURIProject(opts HandleURIOptions, parsed ParsedURI) (map[string]any, error) {
	resolved, err := ResolveURIPath(parsed.Path)
	if err != nil {
		return nil, err
	}

	f, err := Load()
	if err != nil {
		return nil, err
	}
	projectPath, err := f.ResolveProjectPath(resolved)
	if err != nil {
		return nil, err
	}

	healthCheck := func(inst remote.RemoteInstance) bool {
		c := remote.NewClient(remote.ConfigFromInstance(inst, 400*time.Millisecond))
		_, err := c.Health()
		return err == nil
	}
	cfg, _, err := remote.PruneDeadInstances(healthCheck)
	if err != nil {
		return nil, err
	}

	live := remote.FindLiveByProject(cfg.Remote.Instances, projectPath)
	if len(live) > 0 {
		inst := live[0]
		focused := tryFocusLiveInstance(inst)
		out := map[string]any{
			"ok":           true,
			"action":       parsed.Action,
			"uri":          parsed.Raw,
			"project":      projectPath,
			"instance_id":  inst.ID,
			"launched":     false,
			"already_open": true,
			"focused":      focused,
		}
		return out, nil
	}

	fullProfile := parsed.Action == "load"
	launchOut, err := LaunchProject(projectPath, fullProfile, opts.Quiet)
	if err != nil {
		return nil, err
	}
	launchOut["action"] = parsed.Action
	launchOut["uri"] = parsed.Raw
	return launchOut, nil
}

func tryFocusLiveInstance(inst remote.RemoteInstance) bool {
	if inst.RemotePort <= 0 || strings.TrimSpace(inst.RemoteToken) == "" {
		return false
	}
	client := remote.NewClient(remote.ConfigFromInstance(inst, 2*time.Second))

	if cmds, err := client.Commands(); err == nil {
		if names := extractCommandNames(cmds); len(names) > 0 {
			for _, want := range focusCommandCandidates {
				for _, have := range names {
					if strings.EqualFold(have, want) {
						if _, err := client.Exec(have, nil); err == nil {
							return true
						}
					}
				}
			}
		}
	}

	for _, name := range focusCommandCandidates {
		if _, err := client.Exec(name, nil); err == nil {
			return true
		}
	}
	return false
}

func extractCommandNames(cmds map[string]any) []string {
	raw, ok := cmds["commands"]
	if !ok {
		return nil
	}
	switch list := raw.(type) {
	case []any:
		var names []string
		for _, item := range list {
			switch v := item.(type) {
			case string:
				if v != "" {
					names = append(names, v)
				}
			case map[string]any:
				if n, _ := v["name"].(string); n != "" {
					names = append(names, n)
				}
			}
		}
		return names
	case []string:
		return list
	default:
		return nil
	}
}
