package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blazium-cli/output"

	"github.com/spf13/cobra"
)

// NewCommand returns the `remote` command group.
func NewCommand() *cobra.Command {
	cfg := DefaultConfig()
	var format string
	var discover bool

	remoteCmd := &cobra.Command{
		Use:   "remote",
		Short: "Control a running Blazium editor via remote_control HTTP API",
		Long:  "Talk to the remote_control module (GET/POST /v1/*) for status, command execution, and gated eval.",
	}

	remoteCmd.PersistentFlags().StringVar(&cfg.Host, "host", cfg.Host, "Remote control host")
	remoteCmd.PersistentFlags().IntVar(&cfg.Port, "port", cfg.Port, "Remote control port")
	remoteCmd.PersistentFlags().StringVar(&cfg.Token, "token", cfg.Token, "Bearer token (or BLAZIUM_REMOTE_TOKEN)")
	remoteCmd.PersistentFlags().DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "HTTP timeout")
	remoteCmd.PersistentFlags().StringVar(&format, "format", "human", "Output format: human, json, or tsv")
	remoteCmd.PersistentFlags().BoolVar(&discover, "discover", false, "Scan local ports 6500-6520 for /v1/health")

	clientFrom := func() *Client {
		return NewClient(cfg)
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show connected editor/runtime status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if discover {
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
				return output.Write(format, map[string]any{"instances": list})
			}
			st, err := clientFrom().Status()
			if err != nil {
				output.ErrorJSON(format, err)
				return err
			}
			return output.Write(format, st)
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List commands registered on the connected instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := clientFrom().Commands()
			if err != nil {
				output.ErrorJSON(format, err)
				return err
			}
			return output.Write(format, out)
		},
	}

	var jsonArgs string
	execCmd := &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute a named remote command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			out, err := clientFrom().Exec(args[0], argMap)
			if err != nil {
				output.ErrorJSON(format, err)
				return err
			}
			if ok, exists := out["ok"].(bool); exists && !ok {
				output.ErrorJSON(format, fmt.Errorf("%v", out["error"]))
				return fmt.Errorf("%v", out["error"])
			}
			return output.Write(format, out)
		},
	}
	execCmd.Flags().StringVar(&jsonArgs, "json-args", "", "JSON object of command arguments")

	runEval := func(language, expression string) error {
		out, err := clientFrom().Eval(expression, language)
		if err != nil {
			output.ErrorJSON(format, err)
			return err
		}
		return output.Write(format, out)
	}

	evalCmd := &cobra.Command{
		Use:   "eval <expression>",
		Short: "Evaluate using the configured default language (see remote config eval-default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lang, err := EvalDefault()
			if err != nil {
				output.ErrorJSON(format, err)
				return err
			}
			return runEval(lang, args[0])
		},
	}

	evalGDCmd := &cobra.Command{
		Use:   "eval-gdscript <expression>",
		Short: "Evaluate a GDScript Expression on the connected instance (requires allow_eval)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEval(LangGDScript, args[0])
		},
	}

	evalLuaCmd := &cobra.Command{
		Use:   "eval-lua <expression>",
		Short: "Evaluate Luau on the connected instance (requires allow_eval + luau_module)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEval(LangLuau, args[0])
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Get or set blazium-cli remote preferences",
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "get [key]",
		Short: "Print a config value (default: eval-default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := "eval-default"
			if len(args) > 0 {
				key = args[0]
			}
			if key != "eval-default" {
				return fmt.Errorf("unknown config key %q (supported: eval-default)", key)
			}
			lang, err := EvalDefault()
			if err != nil {
				output.ErrorJSON(format, err)
				return err
			}
			path, _ := ConfigPath()
			return output.Write(format, map[string]any{
				"key":              "eval-default",
				"value":            lang,
				"config_path":      path,
				"env_override_set": strings.TrimSpace(os.Getenv("BLAZIUM_REMOTE_EVAL_DEFAULT")) != "",
			})
		},
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "set eval-default <gdscript|luau>",
		Short: "Persist the default language for remote eval",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "eval-default" {
				return fmt.Errorf("unknown config key %q (supported: eval-default)", args[0])
			}
			lang, err := SetEvalDefault(args[1])
			if err != nil {
				output.ErrorJSON(format, err)
				return err
			}
			path, _ := ConfigPath()
			return output.Write(format, map[string]any{
				"ok":          true,
				"key":         "eval-default",
				"value":       lang,
				"config_path": path,
			})
		},
	})

	var projectPath string
	var allowEval bool
	var projectPort int
	enableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable remote_control in a Blazium project (project.godot settings)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectPath == "" {
				projectPath = "."
			}
			if err := enableProject(projectPath, projectPort, allowEval); err != nil {
				output.ErrorJSON(format, err)
				return err
			}
			msg := map[string]any{
				"ok":      true,
				"path":    projectPath,
				"enabled": true,
				"hint":    "Restart the editor with this project, or pass --enable-remote-control",
			}
			return output.Write(format, msg)
		},
	}
	enableCmd.Flags().StringVar(&projectPath, "path", ".", "Project directory containing project.godot")
	enableCmd.Flags().BoolVar(&allowEval, "allow-eval", false, "Also enable blazium/remote_control/allow_eval")
	enableCmd.Flags().IntVar(&projectPort, "project-port", 6507, "Port written to project settings")

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose connectivity to remote_control",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{
				"host": cfg.Host,
				"port": cfg.Port,
			}
			c := clientFrom()
			health, err := c.Health()
			if err != nil {
				report["healthy"] = false
				report["error"] = err.Error()
				found := Discover(cfg.Host, 6500, 6520, cfg.Token, 400*time.Millisecond)
				ports := make([]int, 0, len(found))
				for _, f := range found {
					ports = append(ports, f.Port)
				}
				report["discovered_ports"] = ports
				_ = output.Write(format, report)
				return fmt.Errorf("remote_control not reachable at %s:%d", cfg.Host, cfg.Port)
			}
			report["healthy"] = true
			report["health"] = health
			if st, err := c.Status(); err == nil {
				report["status"] = st
			}
			return output.Write(format, report)
		},
	}

	remoteCmd.AddCommand(statusCmd, listCmd, execCmd, evalCmd, evalGDCmd, evalLuaCmd, configCmd, enableCmd, doctorCmd)
	return remoteCmd
}

func enableProject(projectDir string, port int, allowEval bool) error {
	godot := filepath.Join(projectDir, "project.godot")
	data, err := os.ReadFile(godot)
	if err != nil {
		return fmt.Errorf("read project.godot: %w", err)
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
	return os.WriteFile(godot, []byte(text), 0o644)
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
