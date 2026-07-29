package hub

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/output"
	"github.com/blazium-games/blazium-cli/remote"
)

// LaunchOptions configures editor launch with remote_control / JustAMCP.
type LaunchOptions struct {
	EditorPath    string
	EditorVersion string
	ProjectPath   string
	Profile       ProjectProfile
	Quiet         bool
}

// LaunchResult describes a launched editor instance.
type LaunchResult struct {
	Instance      remote.RemoteInstance
	Args          []string
	RemoteEnabled bool
	MCPEnabled    bool
}

// BuildEditorArgs builds argv after the editor binary (or after macOS --args).
func BuildEditorArgs(projectPath string, remotePort int, remoteToken string, enableRemote bool, mcpPort int, enableMCP bool) []string {
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

	args := BuildEditorArgs(opts.ProjectPath, remotePort, token, enableRemote, mcpPort, enableMCP)
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
