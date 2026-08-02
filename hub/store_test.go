package hub

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempHub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "Local"))
	t.Setenv("HOME", dir)
	return dir
}

func TestInstallPathGetSet(t *testing.T) {
	withTempHub(t)
	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(t.TempDir(), "Editors")
	f.InstallPath = want
	if err := Save(f); err != nil {
		t.Fatal(err)
	}
	f2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if f2.InstallPath != want {
		t.Fatalf("got %q want %q", f2.InstallPath, want)
	}
}

func TestEditorsAddDefaultRemove(t *testing.T) {
	withTempHub(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "0.6.1", "blazium.exe")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	ed, err := f.AddEditorFromPath(bin, "0.6.1", "windows", "x86_64", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if ed.Version != "0.6.1" {
		t.Fatalf("version %q", ed.Version)
	}
	if err := f.SetDefaultEditor("0.6.1"); err != nil {
		t.Fatal(err)
	}
	if err := Save(f); err != nil {
		t.Fatal(err)
	}
	f2, _ := Load()
	if f2.DefaultEditor != "0.6.1" {
		t.Fatal(f2.DefaultEditor)
	}
	if _, err := f2.RemoveEditor("0.6.1", ""); err != nil {
		t.Fatal(err)
	}
	if len(f2.Editors) != 0 {
		t.Fatal("expected empty")
	}
}

func TestProjectsAddRemove(t *testing.T) {
	withTempHub(t)
	proj := filepath.Join(t.TempDir(), "Game")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "project.godot"), []byte("config/name=\"Game\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, _ := Load()
	p, err := f.AddProject(proj)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Game" {
		t.Fatalf("name %q", p.Name)
	}
	if err := Save(f); err != nil {
		t.Fatal(err)
	}
	f2, _ := Load()
	if err := f2.RemoveProject("Game"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveEditorForProject(t *testing.T) {
	withTempHub(t)
	f := File{
		DefaultEditor: "0.6.0",
		Editors: []Editor{
			{Version: "0.6.0", Path: "a"},
			{Version: "0.6.5", Path: "b"},
		},
	}
	proj := t.TempDir()
	cfg := `config/features=PackedStringArray("4.3", "GL Compatibility")
blazium/editor_version="0.6.5"
`
	if err := os.WriteFile(filepath.Join(proj, "project.godot"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	ed, reason, err := f.ResolveEditorForProject(proj)
	if err != nil {
		t.Fatal(err)
	}
	if ed.Version != "0.6.5" || reason != "blazium/editor_version" {
		t.Fatalf("got %s via %s", ed.Version, reason)
	}
}
