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

// LaunchOptions configures editor launch with remote_control / JustAMCP.
type LaunchOptions struct {
	EditorPath        string
	EditorVersion     string
	ProjectPath       string
	Profile           ProjectProfile
	Quiet             bool
	CrashReporterPath string
}

// LaunchResult describes a launched editor instance.
type LaunchResult struct {
	Instance      remote.RemoteInstance
	Args          []string
	RemoteEnabled bool
	MCPEnabled    bool
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

// BuildEditorArgs builds argv after the editor binary (or after macOS --args).
func BuildEditorArgs(projectPath string, remotePort int, remoteToken string, enableRemote bool, mcpPort int, enableMCP bool, crashReporterPath string) []string {
	args := []string{"--path", projectPath}
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

// LaunchEditor starts the editor, registers the instance, waits for health, and POSTs instance id.
func LaunchEditor(opts LaunchOptions) (LaunchResult, error) {
	output.Quiet = opts.Quiet

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

	args := BuildEditorArgs(opts.ProjectPath, remotePort, token, enableRemote, mcpPort, enableMCP, ResolveCrashReporterPath(opts.CrashReporterPath))
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

	result := LaunchResult{Instance: inst, Args: args, RemoteEnabled: enableRemote, MCPEnabled: enableMCP}

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
