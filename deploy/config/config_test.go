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
	t.Setenv("BLAZIUM_STEAM_USERNAME", "builder")
	t.Setenv("BLAZIUM_STEAM_API_KEY", "pubkey")
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
