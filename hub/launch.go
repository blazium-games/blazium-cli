package hub

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/output"
	"github.com/blazium-games/blazium-cli/remote"
)

const (
	// LaunchModeEditor opens the project in the editor (--editor --path).
	LaunchModeEditor = "editor"
	// LaunchModeGame runs the project's main scene (--path, no --editor).
	LaunchModeGame = "game"
	// LaunchModeProjectManager starts the editor project manager (--project-manager).
	LaunchModeProjectManager = "project_manager"
)

// LaunchOptions configures editor, game, or project-manager launch.
type LaunchOptions struct {
	EditorPath        string
	EditorVersion     string
	ProjectPath       string
	Profile           ProjectProfile
	Quiet             bool
	CrashReporterPath string
	SkipCrashReporter bool
	AnalyticsConsent  string
	AnalyticsMode     string
	// Mode is editor, game, or project_manager. Empty means editor.
	Mode string
}

// LaunchResult describes a launched editor or game process.
type LaunchResult struct {
	Instance      remote.RemoteInstance
	Args          []string
	RemoteEnabled bool
	MCPEnabled    bool
	Mode          string
}

// DefaultCrashReporterDest is the Hub sidecar path under BLAZIUM (same as update.CrashReporterDest).
func DefaultCrashReporterDest() string {
	root := strings.TrimSpace(os.Getenv("BLAZIUM"))
	if root == "" {
		if runtime.GOOS == "windows" {
			pf := os.Getenv("ProgramFiles")
			if pf == "" {
				pf = `C:\Program Files`
			}
			root = filepath.Join(pf, "Blazium")
		} else {
			root = "/opt/blazium"
		}
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "Hub", "crash_reporter.exe")
	}
	return filepath.Join(root, "bin", "crash_reporter")
}

// ResolveCrashReporterPath returns an existing sidecar path. Explicit wins; otherwise
// DefaultCrashReporterDest. Missing file returns empty (launch still proceeds).
func ResolveCrashReporterPath(explicit string) string {
	p := strings.TrimSpace(explicit)
	if p != "" {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if crashReporterFileExists(p) {
			return p
		}
		return ""
	}
	dest := DefaultCrashReporterDest()
	if crashReporterFileExists(dest) {
		return dest
	}
	return ""
}

func crashReporterFileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// BuildEditorArgs builds argv for editor mode: --path, then --editor, then remote and sidecar flags.
func BuildEditorArgs(projectPath string, remotePort int, remoteToken string, enableRemote bool, mcpPort int, enableMCP bool, crashReporterPath string) []string {
	args := []string{"--path", projectPath, "--editor"}
	if enableRemote {
		args = append(args,
			"--enable-remote-control",
			fmt.Sprintf("--remote-control-port=%d", remotePort),
			fmt.Sprintf("--remote-control-token=%s", remoteToken),
		)
	}
	if enableMCP && mcpPort > 0 {
		args = append(args, "--enable-mcp", "--mcp-port", fmt.Sprintf("%d", mcpPort))
	}
	if p := strings.TrimSpace(crashReporterPath); p != "" {
		args = append(args, "--crash-reporter", p)
	}
	return args
}

// BuildGameArgs builds argv for playing a project. Remote control stays off.
func BuildGameArgs(projectPath string, crashReporterPath string) []string {
	args := []string{"--path", projectPath}
	if p := strings.TrimSpace(crashReporterPath); p != "" {
		args = append(args, "--crash-reporter", p)
	}
	return args
}

// BuildProjectManagerArgs builds argv for the editor project manager.
func BuildProjectManagerArgs(crashReporterPath string) []string {
	args := []string{"--project-manager"}
	if p := strings.TrimSpace(crashReporterPath); p != "" {
		args = append(args, "--crash-reporter", p)
	}
	return args
}

// ProjectMainScene returns application/run/main_scene, or empty when unset.
func ProjectMainScene(projectPath string) (string, error) {
	text, err := ReadProjectSettingsText(projectPath)
	if err != nil {
		return "", err
	}
	return firstString(parseSectionSettings(text), "application/run/main_scene", "run/main_scene"), nil
}

func (opts LaunchOptions) resolvedMode() string {
	if strings.TrimSpace(opts.Mode) == "" {
		return LaunchModeEditor
	}
	return opts.Mode
}

func resolvedCrashPath(opts LaunchOptions) string {
	if opts.SkipCrashReporter {
		return ""
	}
	return ResolveCrashReporterPath(opts.CrashReporterPath)
}

// NormalizeAnalyticsConsent maps Hub/CLI consent values to accepted|declined.
// Empty or "unset" means the editor should keep its own consent state.
func NormalizeAnalyticsConsent(s string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "", "unset":
		return "", nil
	case "accepted", "accept", "1", "true", "yes":
		return "accepted", nil
	case "declined", "decline", "0", "false", "no":
		return "declined", nil
	default:
		return "", fmt.Errorf("invalid --analytics %q (want accepted|declined)", s)
	}
}

// NormalizeAnalyticsMode maps Hub/CLI mode values to anonymous|identified.
func NormalizeAnalyticsMode(s string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "":
		return "", nil
	case "anonymous":
		return "anonymous", nil
	case "identified":
		return "identified", nil
	default:
		return "", fmt.Errorf("invalid --analytics-mode %q (want anonymous|identified)", s)
	}
}

// AppendAnalyticsArgs adds engine --analytics= and --analytics-mode= tokens.
func AppendAnalyticsArgs(args []string, consent, mode string) []string {
	if c := strings.TrimSpace(consent); c != "" {
		args = append(args, "--analytics="+c)
	}
	if m := strings.TrimSpace(mode); m != "" {
		args = append(args, "--analytics-mode="+m)
	}
	return args
}

// LaunchEditor starts the editor, a game, or the project manager.
// Editor mode registers the instance, waits for health, and POSTs the instance id.
// Game mode refuses to start when the project has no main scene and does not wait for remote control.
func LaunchEditor(opts LaunchOptions) (LaunchResult, error) {
	output.Quiet = opts.Quiet
	switch opts.resolvedMode() {
	case LaunchModeGame:
		return launchGame(opts)
	case LaunchModeProjectManager:
		return launchProjectManager(opts)
	case LaunchModeEditor:
	default:
		return LaunchResult{}, fmt.Errorf("invalid launch mode %q", opts.Mode)
	}

	cfg, _, err := remote.PruneDeadInstances(nil)
	if err != nil {
		return LaunchResult{}, err
	}
	cliCfg, err := remote.LoadCLIFile()
	if err != nil {
		return LaunchResult{}, err
	}

	enableRemote := remote.EnableOnOpen(cliCfg)
	enableMCP := remote.EnableMCPOnLoad(cliCfg) && opts.Profile.JustAMCP.Present

	host := "127.0.0.1"
	var remotePort, mcpPort int
	var token, instanceID string

	if enableRemote {
		need := 1
		if enableMCP {
			need = 2
		}
		ports, err := remote.AllocatePorts(host, need, remote.ClaimedPorts(cfg.Remote.Instances))
		if err != nil {
			return LaunchResult{}, err
		}
		remotePort = ports[0]
		if enableMCP {
			mcpPort = ports[1]
		}
		token, err = remote.GenerateToken()
		if err != nil {
			return LaunchResult{}, err
		}
		instanceID, err = remote.UniqueInstanceID(cfg.Remote.Instances)
		if err != nil {
			return LaunchResult{}, err
		}
	}

	crashPath := resolvedCrashPath(opts)
	args := BuildEditorArgs(opts.ProjectPath, remotePort, token, enableRemote, mcpPort, enableMCP, crashPath)
	args = AppendAnalyticsArgs(args, opts.AnalyticsConsent, opts.AnalyticsMode)
	pid, err := startEditorProcess(opts.EditorPath, args)
	if err != nil {
		return LaunchResult{}, err
	}

	inst := remote.RemoteInstance{
		ID:            instanceID,
		ProjectPath:   opts.ProjectPath,
		ProjectName:   opts.Profile.Name,
		Host:          host,
		RemotePort:    remotePort,
		RemoteToken:   token,
		MCPPort:       mcpPort,
		MCPEnabled:    enableMCP,
		PID:           pid,
		Bound:         false,
		EditorPath:    opts.EditorPath,
		EditorVersion: opts.EditorVersion,
		StartedAt:     time.Now().UTC(),
	}

	result := LaunchResult{Instance: inst, Args: args, RemoteEnabled: enableRemote, MCPEnabled: enableMCP, Mode: LaunchModeEditor}

	if !enableRemote {
		return result, nil
	}

	output.Notef("Waiting for remote_control on %s:%d (instance %s)...", host, remotePort, instanceID)
	client := remote.NewClient(remote.Config{
		Host: host, Port: remotePort, Token: token, Timeout: 2 * time.Second,
	})
	if err := client.WaitForHealth(60 * time.Second); err != nil {
		return result, fmt.Errorf("editor started (pid %d) but remote_control did not become ready: %w", pid, err)
	}
	if err := client.SetInstanceID(instanceID); err != nil {
		return result, fmt.Errorf("failed to bind instance id: %w", err)
	}
	inst.Bound = true
	result.Instance = inst

	cfg, err = remote.LoadCLIFile()
	if err != nil {
		return result, err
	}
	remote.AddActiveInstance(&cfg, inst)
	if err := remote.SaveCLIFile(cfg); err != nil {
		return result, err
	}
	return result, nil
}

func launchGame(opts LaunchOptions) (LaunchResult, error) {
	scene, err := ProjectMainScene(opts.ProjectPath)
	if err != nil {
		return LaunchResult{}, err
	}
	if scene == "" {
		return LaunchResult{}, fmt.Errorf("no main scene")
	}
	args := AppendAnalyticsArgs(BuildGameArgs(opts.ProjectPath, resolvedCrashPath(opts)), opts.AnalyticsConsent, opts.AnalyticsMode)
	return startDetached(opts, args, LaunchModeGame)
}

func launchProjectManager(opts LaunchOptions) (LaunchResult, error) {
	args := AppendAnalyticsArgs(BuildProjectManagerArgs(resolvedCrashPath(opts)), opts.AnalyticsConsent, opts.AnalyticsMode)
	return startDetached(opts, args, LaunchModeProjectManager)
}

func startDetached(opts LaunchOptions, args []string, mode string) (LaunchResult, error) {
	pid, err := startEditorProcess(opts.EditorPath, args)
	if err != nil {
		return LaunchResult{}, err
	}
	inst := remote.RemoteInstance{
		ProjectPath:   opts.ProjectPath,
		ProjectName:   opts.Profile.Name,
		PID:           pid,
		EditorPath:    opts.EditorPath,
		EditorVersion: opts.EditorVersion,
		StartedAt:     time.Now().UTC(),
	}
	return LaunchResult{Instance: inst, Args: args, Mode: mode}, nil
}

func startEditorProcess(editorPath string, args []string) (int, error) {
	var cmd *exec.Cmd
	if strings.HasSuffix(strings.ToLower(editorPath), ".app") {
		openArgs := []string{"-a", editorPath, "--args"}
		openArgs = append(openArgs, args...)
		cmd = exec.Command("open", openArgs...)
	} else {
		cmd = exec.Command(editorPath, args...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("launch editor: %w", err)
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	go func() { _ = cmd.Wait() }()
	return pid, nil
}
