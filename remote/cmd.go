package remote

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blazium-games/blazium-cli/output"

	"github.com/spf13/cobra"
)

// Options configures the remote command group from root flags.
type Options struct {
	Format *string
	Quiet  *bool
}

// NewCommand returns the `remote` command group.
func NewCommand(opts Options) *cobra.Command {
	base := DefaultConfig()
	format := func() string {
		if opts.Format != nil {
			return *opts.Format
		}
		return "human"
	}
	var discover bool
	var instanceID string
	var projectRef string
	var portFlag int
	var tokenFlag string
	var hostFlag string

	remoteCmd := &cobra.Command{
		Use:   "remote",
		Short: "Control a running Blazium editor via remote_control HTTP API",
		Long:  "Talk to the remote_control module (GET/POST /v1/*) for status, command execution, and gated eval.",
	}

	remoteCmd.PersistentFlags().StringVar(&hostFlag, "host", base.Host, "Remote control host")
	remoteCmd.PersistentFlags().IntVar(&portFlag, "port", base.Port, "Remote control port")
	remoteCmd.PersistentFlags().StringVar(&tokenFlag, "token", base.Token, "Bearer token (or BLAZIUM_REMOTE_TOKEN)")
	remoteCmd.PersistentFlags().DurationVar(&base.Timeout, "timeout", base.Timeout, "HTTP timeout")
	remoteCmd.PersistentFlags().BoolVar(&discover, "discover", false, "Scan local ports 6500-6520 for /v1/health")
	remoteCmd.PersistentFlags().StringVar(&instanceID, "instance", "", "Target instance id (6-char)")
	remoteCmd.PersistentFlags().StringVar(&projectRef, "project", "", "Target project path or registered name")

	resolveCfg := func(cmd *cobra.Command) (Config, error) {
		cfg := Config{
			Host:    hostFlag,
			Port:    portFlag,
			Token:   tokenFlag,
			Timeout: base.Timeout,
		}
		portChanged := cmd.Flags().Changed("port")
		tokenChanged := cmd.Flags().Changed("token")
		hostChanged := cmd.Flags().Changed("host")
		envPort := strings.TrimSpace(os.Getenv("BLAZIUM_REMOTE_PORT")) != ""
		envToken := strings.TrimSpace(os.Getenv("BLAZIUM_REMOTE_TOKEN")) != ""
		envHost := strings.TrimSpace(os.Getenv("BLAZIUM_REMOTE_HOST")) != ""

		explicit := portChanged || tokenChanged || envPort || envToken
		if explicit {
			if !hostChanged && !envHost {
				cfg.Host = envOr("BLAZIUM_REMOTE_HOST", "127.0.0.1")
			}
			return cfg, nil
		}

		healthCheck := func(inst RemoteInstance) bool {
			c := NewClient(ConfigFromInstance(inst, 400*time.Millisecond))
			_, err := c.Health()
			return err == nil
		}
		file, _, err := PruneDeadInstances(healthCheck)
		if err != nil {
			return cfg, err
		}

		if instanceID != "" {
			if inst := FindLiveByID(file.Remote.Instances, instanceID); inst != nil {
				return ConfigFromInstance(*inst, cfg.Timeout), nil
			}
			if FindRetiredID(file.Remote.Closed, instanceID) != nil {
				return cfg, fmt.Errorf("instance %s is no longer active", strings.ToUpper(instanceID))
			}
			return cfg, fmt.Errorf("unknown instance %q", instanceID)
		}

		candidates := file.Remote.Instances
		if projectRef != "" {
			path := projectRef
			if abs, err := filepath.Abs(projectRef); err == nil {
				if projectSettingsFile(abs) != "" {
					path = abs
				}
			}
			candidates = FindLiveByProject(file.Remote.Instances, path)
			if len(candidates) == 0 {
				// try case-insensitive path match already done; also try as-is
				candidates = FindLiveByProject(file.Remote.Instances, projectRef)
			}
			if len(candidates) == 0 {
				return cfg, fmt.Errorf("no active remote instances for project %q", projectRef)
			}
		}

		if len(candidates) == 0 {
			return cfg, nil // legacy defaults
		}
		sorted := SortNewestFirst(candidates)
		chosen := sorted[0]
		if len(sorted) > 1 {
			ids := make([]string, 0, len(sorted))
			for _, s := range sorted {
				ids = append(ids, s.ID)
			}
			output.Warnf("%d active instances; using %s (newest). Pass --instance <id> to target another. Active: %s",
				len(sorted), chosen.ID, strings.Join(ids, ", "))
		}
		return ConfigFromInstance(chosen, cfg.Timeout), nil
	}

	clientFrom := func(cmd *cobra.Command) (*Client, error) {
		cfg, err := resolveCfg(cmd)
		if err != nil {
			return nil, err
		}
		return NewClient(cfg), nil
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show connected editor/runtime status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if discover {
				cfg, err := resolveCfg(cmd)
				if err != nil {
					return err
				}
				found := Discover(cfg.Host, 6500, 6520, cfg.Token, 500*time.Millisecond)
				if len(found) == 0 {
					return fmt.Errorf("no remote_control instances found on %s:6500-6520", cfg.Host)
				}
				list := make([]any, 0, len(found))
				for _, f := range found {
					c := NewClient(f)
					st, err := c.Status()
					if err != nil {
						st = map[string]any{"host": f.Host, "port": f.Port, "error": err.Error()}
					} else {
						st["host"] = f.Host
						st["port"] = f.Port
					}
					list = append(list, st)
				}
				return output.Write(format(), map[string]any{"instances": list})
			}
			c, err := clientFrom(cmd)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			st, err := c.Status()
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), st)
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List commands registered on the connected instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			out, err := c.Commands()
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	}

	var jsonArgs string
	execCmd := &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute a named remote command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			argMap := map[string]any{}
			if strings.TrimSpace(jsonArgs) != "" {
				if err := json.Unmarshal([]byte(jsonArgs), &argMap); err != nil {
					return fmt.Errorf("parse --json-args: %w", err)
				}
			}
			for _, a := range args[1:] {
				if k, v, ok := strings.Cut(a, "="); ok {
					argMap[k] = v
				}
			}
			out, err := c.Exec(args[0], argMap)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			if ok, exists := out["ok"].(bool); exists && !ok {
				output.ErrorJSON(format(), fmt.Errorf("%v", out["error"]))
				return fmt.Errorf("%v", out["error"])
			}
			return output.Write(format(), out)
		},
	}
	execCmd.Flags().StringVar(&jsonArgs, "json-args", "", "JSON object of command arguments")

	runEval := func(cmd *cobra.Command, language, expression string) error {
		c, err := clientFrom(cmd)
		if err != nil {
			output.ErrorJSON(format(), err)
			return err
		}
		out, err := c.Eval(expression, language)
		if err != nil {
			output.ErrorJSON(format(), err)
			return err
		}
		if ok, exists := out["ok"].(bool); exists && !ok {
			output.ErrorJSON(format(), fmt.Errorf("%v", out["error"]))
			return fmt.Errorf("%v", out["error"])
		}
		return output.Write(format(), out)
	}

	evalCmd := &cobra.Command{
		Use:   "eval <expression>",
		Short: "Evaluate using the configured default language (see remote config eval-default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lang, err := EvalDefault()
			if err != nil {
				return err
			}
			return runEval(cmd, lang, args[0])
		},
	}
	evalGDCmd := &cobra.Command{
		Use:   "eval-gdscript <expression>",
		Short: "Evaluate a GDScript expression on the remote instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEval(cmd, LangGDScript, args[0])
		},
	}
	evalLuaCmd := &cobra.Command{
		Use:   "eval-lua <expression>",
		Short: "Evaluate a Luau expression on the remote instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEval(cmd, LangLuau, args[0])
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Get or set blazium-cli remote preferences",
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "get [key]",
		Short: "Print a config value (eval-default|enable-on-open|enable-mcp-on-load)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := "eval-default"
			if len(args) > 0 {
				key = args[0]
			}
			path, _ := ConfigPath()
			cfg, err := LoadCLIFile()
			if err != nil {
				return err
			}
			switch key {
			case "eval-default":
				lang, err := EvalDefault()
				if err != nil {
					return err
				}
				return output.Write(format(), map[string]any{
					"key": key, "value": lang, "config_path": path,
					"env_override_set": strings.TrimSpace(os.Getenv("BLAZIUM_REMOTE_EVAL_DEFAULT")) != "",
				})
			case "enable-on-open":
				return output.Write(format(), map[string]any{"key": key, "value": EnableOnOpen(cfg), "config_path": path})
			case "enable-mcp-on-load":
				return output.Write(format(), map[string]any{"key": key, "value": EnableMCPOnLoad(cfg), "config_path": path})
			default:
				return fmt.Errorf("unknown config key %q (supported: eval-default, enable-on-open, enable-mcp-on-load)", key)
			}
		},
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist a remote preference",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := ConfigPath()
			switch args[0] {
			case "eval-default":
				lang, err := SetEvalDefault(args[1])
				if err != nil {
					return err
				}
				return output.Write(format(), map[string]any{"ok": true, "key": args[0], "value": lang, "config_path": path})
			case "enable-on-open":
				v, err := strconv.ParseBool(args[1])
				if err != nil {
					return fmt.Errorf("enable-on-open expects true/false")
				}
				if err := SetEnableOnOpen(v); err != nil {
					return err
				}
				return output.Write(format(), map[string]any{"ok": true, "key": args[0], "value": v, "config_path": path})
			case "enable-mcp-on-load":
				v, err := strconv.ParseBool(args[1])
				if err != nil {
					return fmt.Errorf("enable-mcp-on-load expects true/false")
				}
				if err := SetEnableMCPOnLoad(v); err != nil {
					return err
				}
				return output.Write(format(), map[string]any{"ok": true, "key": args[0], "value": v, "config_path": path})
			default:
				return fmt.Errorf("unknown config key %q (supported: eval-default, enable-on-open, enable-mcp-on-load)", args[0])
			}
		},
	})

	var showAll bool
	instancesCmd := &cobra.Command{
		Use:   "instances",
		Short: "List active remote_control editor instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _, err := PruneDeadInstances(func(inst RemoteInstance) bool {
				c := NewClient(ConfigFromInstance(inst, 400*time.Millisecond))
				_, err := c.Health()
				return err == nil
			})
			if err != nil {
				return err
			}
			rows := make([]any, 0, len(file.Remote.Instances))
			for _, inst := range SortNewestFirst(file.Remote.Instances) {
				row := map[string]any{
					"id":           inst.ID,
					"project":      inst.ProjectPath,
					"project_name": inst.ProjectName,
					"host":         inst.Host,
					"port":         inst.RemotePort,
					"pid":          inst.PID,
					"bound":        inst.Bound,
					"mcp_port":     inst.MCPPort,
					"mcp_enabled":  inst.MCPEnabled,
					"editor":       inst.EditorVersion,
					"started_at":   inst.StartedAt,
				}
				if strings.EqualFold(format(), "json") {
					row["token"] = inst.RemoteToken
				} else {
					row["token"] = redactToken(inst.RemoteToken)
				}
				rows = append(rows, row)
			}
			out := map[string]any{"instances": rows}
			if showAll {
				closed := make([]any, 0, len(file.Remote.Closed))
				for _, c := range file.Remote.Closed {
					closed = append(closed, c)
				}
				out["closed"] = closed
			}
			return output.Write(format(), out)
		},
	}
	instancesCmd.Flags().BoolVar(&showAll, "all", false, "Include closed instance history")

	var projectPath string
	var allowEval bool
	var projectPort int
	enableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable remote_control in a Blazium project (project.blazium or project.godot settings)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectPath == "" {
				projectPath = "."
			}
			if err := enableProject(projectPath, projectPort, allowEval); err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			msg := map[string]any{
				"ok":      true,
				"path":    projectPath,
				"enabled": true,
				"hint":    "Restart the editor with this project, or pass --enable-remote-control",
			}
			return output.Write(format(), msg)
		},
	}
	enableCmd.Flags().StringVar(&projectPath, "path", ".", "Project directory containing project.blazium or project.godot")
	enableCmd.Flags().BoolVar(&allowEval, "allow-eval", false, "Also enable blazium/remote_control/allow_eval")
	enableCmd.Flags().IntVar(&projectPort, "project-port", 6507, "Port written to project settings")

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose connectivity to remote_control",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			report := map[string]any{
				"host": c.Cfg.Host,
				"port": c.Cfg.Port,
			}
			health, err := c.Health()
			if err != nil {
				report["healthy"] = false
				report["error"] = err.Error()
				found := Discover(c.Cfg.Host, 6500, 6599, c.Cfg.Token, 400*time.Millisecond)
				ports := make([]int, 0, len(found))
				for _, f := range found {
					ports = append(ports, f.Port)
				}
				report["discovered_ports"] = ports
				_ = output.Write(format(), report)
				return fmt.Errorf("remote_control not reachable at %s:%d", c.Cfg.Host, c.Cfg.Port)
			}
			report["healthy"] = true
			report["health"] = health
			if st, err := c.Status(); err == nil {
				report["status"] = st
			}
			return output.Write(format(), report)
		},
	}

	var logSince, logCursor uint64
	var logLimit int
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Fetch engine/editor logs (incremental via --since)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			out, err := c.Logs(logSince, logCursor, logLimit, false)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	}
	logsCmd.Flags().Uint64Var(&logSince, "since", 0, "Only entries with id greater than this")
	logsCmd.Flags().Uint64Var(&logCursor, "cursor", 0, "Start paging at this entry id")
	logsCmd.Flags().IntVar(&logLimit, "limit", 200, "Page size (max 1000)")

	var errSince uint64
	var errLimit int
	errorsCmd := &cobra.Command{
		Use:   "errors",
		Short: "Fetch error/warning log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			out, err := c.Logs(errSince, 0, errLimit, true)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	}
	errorsCmd.Flags().Uint64Var(&errSince, "since", 0, "Only entries with id greater than this")
	errorsCmd.Flags().IntVar(&errLimit, "limit", 200, "Page size (max 1000)")

	debuggerCmd := &cobra.Command{
		Use:   "debugger",
		Short: "Debugger status, stack, breakpoints, and clear",
	}
	debuggerCmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Full debugger dump (stack, errors, breakpoints, error breaks)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("debugger_info", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	})
	debuggerCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Debugger status subset",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("debugger_status", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	})
	debuggerCmd.AddCommand(&cobra.Command{
		Use:   "stack",
		Short: "Stack frames when broken",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("debugger_stack", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	})
	debuggerCmd.AddCommand(&cobra.Command{
		Use:   "breakpoints",
		Short: "List breakpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("debugger_list_breakpoints", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	})
	debuggerCmd.AddCommand(&cobra.Command{
		Use:   "error-breaks",
		Short: "Recent error-breakpoint / failed-break hits",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("debugger_error_breaks", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	})
	var clearErrors, clearErrorBreaks, clearLogs bool
	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear debugger errors and remote error-break ring",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			// Defaults: errors+error_breaks true, logs false — unless flags explicitly set.
			argsMap := map[string]any{
				"errors":       true,
				"error_breaks": true,
				"logs":         false,
			}
			if cmd.Flags().Changed("errors") {
				argsMap["errors"] = clearErrors
			}
			if cmd.Flags().Changed("error-breaks") {
				argsMap["error_breaks"] = clearErrorBreaks
			}
			if cmd.Flags().Changed("logs") {
				argsMap["logs"] = clearLogs
			}
			out, err := c.Exec("debugger_clear", argsMap)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	}
	clearCmd.Flags().BoolVar(&clearErrors, "errors", true, "Clear Errors tab")
	clearCmd.Flags().BoolVar(&clearErrorBreaks, "error-breaks", true, "Clear error-break ring")
	clearCmd.Flags().BoolVar(&clearLogs, "logs", false, "Also clear remote print log ring")
	debuggerCmd.AddCommand(clearCmd)

	failedRunCmd := &cobra.Command{
		Use:   "failed-run",
		Short: "Bundle logs, debugger, Autowork, and evidence files",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("get_failed_run", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	}

	autoworkCmd := &cobra.Command{
		Use:   "autowork",
		Short: "Trigger Autowork and fetch results",
	}
	var awDir, awFile, awTest, awPrefix, awSuffix string
	var awWait bool
	var awWaitTimeout time.Duration
	awRunCmd := &cobra.Command{
		Use:   "run",
		Short: "Start an Autowork job",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			argMap := map[string]any{}
			if awDir != "" {
				argMap["dir"] = awDir
			}
			if awFile != "" {
				argMap["file"] = awFile
			}
			if awTest != "" {
				argMap["test_name"] = awTest
			}
			if awPrefix != "" {
				argMap["prefix"] = awPrefix
			}
			if awSuffix != "" {
				argMap["suffix"] = awSuffix
			}
			out, err := c.Exec("autowork_run", argMap)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			if awWait {
				results, err := c.WaitAutowork(awWaitTimeout)
				if err != nil {
					output.ErrorJSON(format(), err)
					return err
				}
				return output.Write(format(), results)
			}
			return output.Write(format(), out)
		},
	}
	awRunCmd.Flags().StringVar(&awDir, "dir", "", "Test directory (res://...)")
	awRunCmd.Flags().StringVar(&awFile, "file", "", "Single test script path")
	awRunCmd.Flags().StringVar(&awTest, "test", "", "Filter by test name")
	awRunCmd.Flags().StringVar(&awPrefix, "prefix", "", "Script prefix filter")
	awRunCmd.Flags().StringVar(&awSuffix, "suffix", "", "Script suffix filter")
	awRunCmd.Flags().BoolVar(&awWait, "wait", false, "Poll until job completes")
	awRunCmd.Flags().DurationVar(&awWaitTimeout, "wait-timeout", 10*time.Minute, "Timeout for --wait")
	autoworkCmd.AddCommand(awRunCmd)
	autoworkCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Autowork job status",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("autowork_status", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	})
	autoworkCmd.AddCommand(&cobra.Command{
		Use:   "results",
		Short: "Autowork job results",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			out, err := c.Exec("autowork_results", nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			return output.Write(format(), out)
		},
	})

	var snapshotOutput string
	snapshotCmd := &cobra.Command{
		Use:   "snapshot <editor|scene>",
		Short: "Capture an editor or playing-scene PNG snapshot",
		Long:  "Calls remote exec snapshot_editor / snapshot_scene, writes PNG locally, and prints metadata (omits png_base64 by default).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := strings.ToLower(strings.TrimSpace(args[0]))
			command := ""
			switch target {
			case "editor":
				command = "snapshot_editor"
			case "scene", "play", "game":
				command = "snapshot_scene"
				target = "scene"
			default:
				return fmt.Errorf("target must be editor or scene (got %q)", args[0])
			}

			c, err := clientFrom(cmd)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			out, err := c.Exec(command, nil)
			if err != nil {
				output.ErrorJSON(format(), err)
				return err
			}
			if ok, _ := out["ok"].(bool); !ok {
				msg, _ := out["error"].(string)
				if msg == "" {
					msg = "snapshot failed"
				}
				err := fmt.Errorf("%s", msg)
				output.ErrorJSON(format(), err)
				return err
			}

			b64, _ := out["png_base64"].(string)
			if strings.TrimSpace(b64) == "" {
				err := fmt.Errorf("snapshot response missing png_base64")
				output.ErrorJSON(format(), err)
				return err
			}
			png, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				err = fmt.Errorf("decode png_base64: %w", err)
				output.ErrorJSON(format(), err)
				return err
			}

			outPath := strings.TrimSpace(snapshotOutput)
			if outPath == "" {
				ts := time.Now().Format("20060102_150405")
				outPath = fmt.Sprintf("snapshot_%s_%s.png", target, ts)
			}
			if err := os.WriteFile(outPath, png, 0o644); err != nil {
				err = fmt.Errorf("write %s: %w", outPath, err)
				output.ErrorJSON(format(), err)
				return err
			}
			abs, _ := filepath.Abs(outPath)

			meta := map[string]any{
				"ok":     true,
				"path":   abs,
				"target": target,
				"width":  out["width"],
				"height": out["height"],
				"source": out["source"],
				"mime":   out["mime"],
			}
			if v, ok := out["playing"]; ok {
				meta["playing"] = v
			}
			if v, ok := out["paused"]; ok {
				meta["paused"] = v
			}
			if v, ok := out["scene"]; ok {
				meta["scene"] = v
			}
			return output.Write(format(), meta)
		},
	}
	snapshotCmd.Flags().StringVarP(&snapshotOutput, "output", "o", "", "Local PNG output path (default snapshot_<target>_<timestamp>.png)")

	remoteCmd.AddCommand(statusCmd, listCmd, execCmd, evalCmd, evalGDCmd, evalLuaCmd, configCmd, instancesCmd, enableCmd, doctorCmd, logsCmd, errorsCmd, debuggerCmd, failedRunCmd, autoworkCmd, snapshotCmd)
	return remoteCmd
}

func redactToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "…" + token[len(token)-4:]
}

func projectSettingsFile(dir string) string {
	for _, name := range []string{"project.blazium", "project.godot"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !st.IsDir() {
			return name
		}
	}
	return ""
}

func enableProject(projectDir string, port int, allowEval bool) error {
	name := projectSettingsFile(projectDir)
	if name == "" {
		return fmt.Errorf("missing project.blazium or project.godot in %s", projectDir)
	}
	cfg := filepath.Join(projectDir, name)
	data, err := os.ReadFile(cfg)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	text := string(data)
	settings := fmt.Sprintf(
		"remote_control/server_enabled=true\nremote_control/server_port=%d\nremote_control/bind_address=\"127.0.0.1\"\nremote_control/allow_eval=%v\n",
		port, allowEval,
	)

	const marker = "remote_control/server_enabled"
	if strings.Contains(text, marker) {
		text = replaceOrAppendSetting(text, "remote_control/server_enabled", "true")
		text = replaceOrAppendSetting(text, "remote_control/server_port", fmt.Sprintf("%d", port))
		text = replaceOrAppendSetting(text, "remote_control/allow_eval", fmt.Sprintf("%v", allowEval))
	} else if strings.Contains(text, "[blazium]") {
		text = strings.Replace(text, "[blazium]", "[blazium]\n"+settings, 1)
	} else {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n[blazium]\n\n" + settings
	}
	return os.WriteFile(cfg, []byte(text), 0o644)
}

func replaceOrAppendSetting(text, key, value string) string {
	lines := strings.Split(text, "\n")
	found := false
	prefix := key + "="
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}
	return strings.Join(lines, "\n")
}
