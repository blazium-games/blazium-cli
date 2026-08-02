package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func withTempHubRemoteEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("PROGRAMDATA", filepath.Join(dir, "ProgramData"))
	t.Setenv("BLAZIUM_HUB_REMOTE_MACHINE", filepath.Join(dir, "machine", "hub_remote.json"))
	return dir
}

func TestEnsureHubRemoteSecretRoundTrip(t *testing.T) {
	withTempHubRemoteEnv(t)
	f, err := EnsureHubRemoteSecret()
	if err != nil {
		t.Fatal(err)
	}
	if f.Port != HubRemotePort || len(f.Token) != 64 || f.Host != "127.0.0.1" {
		t.Fatalf("%#v", f)
	}
	again, err := EnsureHubRemoteSecret()
	if err != nil {
		t.Fatal(err)
	}
	if again.Token != f.Token {
		t.Fatalf("token regenerated unexpectedly")
	}
	path, err := HubRemoteConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestFocusHubUsesRemoteControl(t *testing.T) {
	withTempHubRemoteEnv(t)

	var sawAuth bool
	var sawShow bool
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer test-hub-token" {
			sawAuth = true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/v1/exec", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["command"] == "show_hub" || body["command"] == "focus_window" {
			sawShow = true
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "focused": true})
			return
		}
		http.Error(w, `{"ok":false,"error":"unknown"}`, http.StatusBadRequest)
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

	if err := SaveHubRemote(HubRemoteFile{
		Host:  u.Hostname(),
		Port:  port,
		Token: "test-hub-token",
	}); err != nil {
		t.Fatal(err)
	}

	out, err := FocusHub(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true || out["action"] != "hub" || out["focused"] != true {
		t.Fatalf("%#v", out)
	}
	if out["launched"] != false {
		t.Fatalf("expected already-running hub, got launched=%v", out["launched"])
	}
	if !sawAuth || !sawShow {
		t.Fatalf("auth=%v show=%v", sawAuth, sawShow)
	}
}

func TestLoadHubRemotePrefersUserThenMachine(t *testing.T) {
	dir := withTempHubRemoteEnv(t)
	machine := filepath.Join(dir, "machine", "hub_remote.json")
	user, err := HubRemoteConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveHubRemoteAt(machine, HubRemoteFile{
		Host:  "127.0.0.1",
		Port:  HubRemotePort,
		Token: "machine-token-abcdefghijklmnopqrstuvwxyz012345",
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadHubRemote()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "machine-token-abcdefghijklmnopqrstuvwxyz012345" {
		t.Fatalf("expected machine token, got %#v", loaded)
	}
	if err := SaveHubRemoteAt(user, HubRemoteFile{
		Host:  "127.0.0.1",
		Port:  HubRemotePort,
		Token: "user-token-abcdefghijklmnopqrstuvwxyz0123456789ab",
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadHubRemote()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "user-token-abcdefghijklmnopqrstuvwxyz0123456789ab" {
		t.Fatalf("expected user token, got %#v", loaded)
	}
}

func TestEnsureHubRemoteSecretAtIdempotent(t *testing.T) {
	dir := withTempHubRemoteEnv(t)
	path := filepath.Join(dir, "custom", "hub_remote.json")
	first, err := EnsureHubRemoteSecretAt(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EnsureHubRemoteSecretAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Token != second.Token || len(first.Token) != 64 {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestHandleURIHubFocusesLiveHub(t *testing.T) {
	withTempHubRemoteEnv(t)
	// Also isolate hub.json so register/other helpers stay clean if touched.
	_ = filepath.Join(os.Getenv("APPDATA"), "blazium")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/v1/exec", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "focused": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	_ = SaveHubRemote(HubRemoteFile{Host: u.Hostname(), Port: port, Token: "tok"})

	out, err := HandleURI(HandleURIOptions{URI: "blazium://hub"})
	if err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true || out["action"] != "hub" || out["uri"] != "blazium://hub" {
		t.Fatalf("%#v", out)
	}
}
