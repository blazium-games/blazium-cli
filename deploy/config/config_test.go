package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileExpandsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blazium-deploy.yml")
	body := []byte("steam:\n  app_id: \"123\"\n  username: ${BLAZIUM_STEAM_USERNAME}\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLAZIUM_STEAM_APP_ID", "999")
	t.Setenv("BLAZIUM_STEAM_USERNAME", "builder")
	t.Setenv("BLAZIUM_STEAM_API_KEY", "pubkey")
	t.Setenv("BLAZIUM_STEAM_REFRESH_TOKEN", "rtok")
	t.Setenv("BLAZIUM_STEAM_PUBLISHER_KEY", "pkey")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Steam.AppID != "123" || cfg.Steam.Username != "builder" {
		t.Fatalf("%+v", cfg.Steam)
	}
	if cfg.Steam.APIKey != "pubkey" {
		t.Fatalf("default api key %q", cfg.Steam.APIKey)
	}
	red := cfg.Redacted()
	steam := red["steam"].(map[string]any)
	if steam["username"] != "(set)" || steam["api_key"] != "(set)" {
		t.Fatalf("%v", steam)
	}
	if steam["refresh_token"] != "(set)" || steam["publisher_key"] != "(set)" {
		t.Fatalf("redact extras %v", steam)
	}
	names := EnvNamesUsed(cfg)
	want := map[string]bool{"BLAZIUM_STEAM_USERNAME": true, "BLAZIUM_STEAM_APP_ID": true, "BLAZIUM_STEAM_API_KEY": true}
	for n := range want {
		found := false
		for _, got := range names {
			if got == n {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("env names %v missing %s", names, n)
		}
	}
	for _, n := range names {
		if n == "pubkey" || n == "builder" || n == "rtok" {
			t.Fatalf("value leaked in env names: %v", names)
		}
	}
}

func TestAppIDFromEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blazium-deploy.yml")
	if err := os.WriteFile(path, []byte("steam:\n  username: builder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLAZIUM_STEAM_APP_ID", "480")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Steam.AppID != "480" {
		t.Fatalf("app_id %q", cfg.Steam.AppID)
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "blazium-deploy.yml")
	if err := os.WriteFile(p, []byte("steam:\n  app_id: \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("got %s want %s", got, p)
	}
}
