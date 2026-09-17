package itch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blazium-games/blazium-cli/deploy/config"
)

func TestEntriesFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blazium-deploy.yml")
	body := []byte(`itch:
  target: user/game
  cache_dir: .steam-sync-cache
  steam_sync:
    - app: "123456"
      branch: public
      skip: ["9"]
      map:
        "1001": windows
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := EntriesFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len %d", len(entries))
	}
	if entries[0].App != 123456 || entries[0].Target != "user/game" || entries[0].Branch != "public" {
		t.Fatalf("%+v", entries[0])
	}
	if entries[0].Map["1001"] != "windows" {
		t.Fatalf("map %+v", entries[0].Map)
	}
	if len(entries[0].Skip) != 1 || entries[0].Skip[0] != 9 {
		t.Fatalf("skip %+v", entries[0].Skip)
	}
}

func TestApplyConfigEnvUsesYAMLAPIKey(t *testing.T) {
	t.Setenv("BUTLER_API_KEY", "")
	t.Setenv("BLAZIUM_BUTLER_API_KEY", "")
	ApplyConfigEnv(&config.Config{Itch: config.ItchConfig{APIKey: "from-yaml"}})
	if os.Getenv("BUTLER_API_KEY") != "from-yaml" {
		t.Fatalf("got %q", os.Getenv("BUTLER_API_KEY"))
	}
}
