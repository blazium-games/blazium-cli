package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClientHealthAndExec(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "ok"})
	})
	mux.HandleFunc("/v1/exec", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["command"] != "ping" {
			http.Error(w, `{"error":"bad command"}`, http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "type": "pong"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.Host = u.Hostname()
	cfg.Port = port
	c := NewClient(cfg)

	health, err := c.Health()
	if err != nil {
		t.Fatal(err)
	}
	if health["ok"] != true {
		t.Fatalf("health: %#v", health)
	}
	out, err := c.Exec("ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["type"] != "pong" {
		t.Fatalf("exec: %#v", out)
	}
}

func TestClientEvalForwardsLanguage(t *testing.T) {
	var gotLang string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/eval", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotLang, _ = body["language"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": 4, "language": gotLang})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	c := NewClient(Config{Host: u.Hostname(), Port: port})

	if _, err := c.Eval("2+2", "gdscript"); err != nil {
		t.Fatal(err)
	}
	if gotLang != LangGDScript {
		t.Fatalf("expected gdscript, got %q", gotLang)
	}
	if _, err := c.Eval("1+1", "lua"); err != nil {
		t.Fatal(err)
	}
	if gotLang != LangLuau {
		t.Fatalf("expected luau alias, got %q", gotLang)
	}
}

func TestEvalDefaultEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("BLAZIUM_REMOTE_EVAL_DEFAULT", "")

	if _, err := SetEvalDefault("luau"); err != nil {
		t.Fatal(err)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	lang, err := EvalDefault()
	if err != nil {
		t.Fatal(err)
	}
	if lang != LangLuau {
		t.Fatalf("file default: got %q", lang)
	}

	t.Setenv("BLAZIUM_REMOTE_EVAL_DEFAULT", "gdscript")
	lang, err = EvalDefault()
	if err != nil {
		t.Fatal(err)
	}
	if lang != LangGDScript {
		t.Fatalf("env override: got %q", lang)
	}

	rel, relErr := filepath.Rel(dir, path)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("config path not under temp dir: %s", path)
	}
}
