package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds connection settings for a remote_control editor instance.
type Config struct {
	Host    string
	Port    int
	Token   string
	Timeout time.Duration
}

// DefaultConfig reads host/port/token from env with sensible localhost defaults.
func DefaultConfig() Config {
	cfg := Config{
		Host:    envOr("BLAZIUM_REMOTE_HOST", "127.0.0.1"),
		Port:    6507,
		Token:   os.Getenv("BLAZIUM_REMOTE_TOKEN"),
		Timeout: 30 * time.Second,
	}
	if p := os.Getenv("BLAZIUM_REMOTE_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			cfg.Port = n
		}
	}
	if t := os.Getenv("BLAZIUM_REMOTE_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			cfg.Timeout = d
		}
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func (c Config) baseURL() string {
	return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
}

// Client talks to the remote_control HTTP API.
type Client struct {
	Cfg  Config
	HTTP *http.Client
}

// NewClient builds an HTTP client for the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		Cfg: cfg,
		HTTP: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *Client) do(method, path string, body any) (map[string]any, int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.Cfg.baseURL()+path, reader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Cfg.Token)
		req.Header.Set("X-Remote-Control-Token", c.Cfg.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	var out map[string]any
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("decode response: %w (body=%s)", err, string(data))
		}
	}
	if out == nil {
		out = map[string]any{}
	}
	if resp.StatusCode >= 400 {
		msg := string(data)
		if e, ok := out["error"].(string); ok && e != "" {
			msg = e
		}
		return out, resp.StatusCode, fmt.Errorf("%s", msg)
	}
	return out, resp.StatusCode, nil
}

// Health calls GET /v1/health.
func (c *Client) Health() (map[string]any, error) {
	out, _, err := c.do(http.MethodGet, "/v1/health", nil)
	return out, err
}

// Status calls GET /v1/status.
func (c *Client) Status() (map[string]any, error) {
	out, _, err := c.do(http.MethodGet, "/v1/status", nil)
	return out, err
}

// Commands calls GET /v1/commands.
func (c *Client) Commands() (map[string]any, error) {
	out, _, err := c.do(http.MethodGet, "/v1/commands", nil)
	return out, err
}

// Exec calls POST /v1/exec.
func (c *Client) Exec(command string, args map[string]any) (map[string]any, error) {
	if args == nil {
		args = map[string]any{}
	}
	out, _, err := c.do(http.MethodPost, "/v1/exec", map[string]any{
		"command": command,
		"args":    args,
	})
	return out, err
}

// Eval calls POST /v1/eval with an explicit language (gdscript or luau).
func (c *Client) Eval(expression, language string) (map[string]any, error) {
	lang, err := NormalizeEvalLang(language)
	if err != nil {
		return nil, err
	}
	out, _, err := c.do(http.MethodPost, "/v1/eval", map[string]any{
		"expression": expression,
		"language":   lang,
	})
	return out, err
}

// Logs calls GET /v1/logs with optional since/cursor/limit/errors_only.
func (c *Client) Logs(since, cursor uint64, limit int, errorsOnly bool) (map[string]any, error) {
	if limit <= 0 {
		limit = 200
	}
	q := fmt.Sprintf("/v1/logs?limit=%d", limit)
	if since > 0 {
		q += fmt.Sprintf("&since=%d", since)
	}
	if cursor > 0 {
		q += fmt.Sprintf("&cursor=%d", cursor)
	}
	if errorsOnly {
		q += "&errors_only=1"
	}
	out, _, err := c.do(http.MethodGet, q, nil)
	return out, err
}

// WaitAutowork polls autowork_status until done/failed or timeout.
func (c *Client) WaitAutowork(timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	var last map[string]any
	for time.Now().Before(deadline) {
		st, err := c.Exec("autowork_status", nil)
		if err != nil {
			return nil, err
		}
		last = st
		state, _ := st["state"].(string)
		if state == "done" || state == "failed" || state == "idle" {
			return c.Exec("autowork_results", nil)
		}
		time.Sleep(500 * time.Millisecond)
	}
	if last == nil {
		return nil, fmt.Errorf("autowork wait timed out")
	}
	return last, fmt.Errorf("autowork wait timed out (state=%v)", last["state"])
}

// Discover scans local ports for /v1/health.
func Discover(host string, startPort, endPort int, token string, timeout time.Duration) []Config {
	var found []Config
	for port := startPort; port <= endPort; port++ {
		cfg := Config{Host: host, Port: port, Token: token, Timeout: timeout}
		client := NewClient(cfg)
		if _, err := client.Health(); err == nil {
			found = append(found, cfg)
		}
	}
	return found
}

// WaitForHealth polls GET /v1/health until success or timeout.
func (c *Client) WaitForHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		_, err := c.Health()
		if err == nil {
			return nil
		}
		last = err
		time.Sleep(400 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("timeout")
	}
	return fmt.Errorf("health check failed: %w", last)
}

// SetInstanceID calls POST /v1/instance.
func (c *Client) SetInstanceID(id string) error {
	out, _, err := c.do(http.MethodPost, "/v1/instance", map[string]any{
		"instance_id": id,
	})
	if err != nil {
		return err
	}
	if ok, exists := out["ok"].(bool); exists && !ok {
		return fmt.Errorf("%v", out["error"])
	}
	return nil
}

// ConfigFromInstance builds a client Config from a registry entry.
func ConfigFromInstance(inst RemoteInstance, timeout time.Duration) Config {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	host := inst.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return Config{Host: host, Port: inst.RemotePort, Token: inst.RemoteToken, Timeout: timeout}
}
