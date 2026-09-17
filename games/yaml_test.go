package games

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBuildYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "build.yml")
	body := []byte(`version: v1
spec: build
asset:
  title: Demo
  type: game
  description: A demo
  version: "1.0.0"
`)
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseYAML(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec != "build" || cfg.BuildAsset == nil || cfg.BuildAsset.Title != "Demo" {
		t.Fatalf("%+v", cfg)
	}
}
