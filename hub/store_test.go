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

func TestProjectSettingsFilePrefersBlazium(t *testing.T) {
	dir := t.TempDir()
	if got := ProjectSettingsFile(dir); got != "" {
		t.Fatalf("empty dir: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, ProjectFileGodot), []byte("config/name=\"G\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ProjectSettingsFile(dir); got != ProjectFileGodot {
		t.Fatalf("godot only: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, ProjectFileBlazium), []byte("config/name=\"B\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ProjectSettingsFile(dir); got != ProjectFileBlazium {
		t.Fatalf("prefer blazium: %q", got)
	}
	absGodot, err := filepath.Abs(filepath.Join(dir, ProjectFileGodot))
	if err != nil {
		t.Fatal(err)
	}
	norm, err := NormalizeProjectDir(absGodot)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(norm, want) {
		t.Fatalf("normalize file: %q want %q", norm, want)
	}
}

func TestProjectsAddBlaziumOnly(t *testing.T) {
	withTempHub(t)
	proj := filepath.Join(t.TempDir(), "BlaziumGame")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(proj, ProjectFileBlazium)
	if err := os.WriteFile(cfg, []byte("config/name=\"BlaziumGame\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.AddProject(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "BlaziumGame" {
		t.Fatalf("name %q", p.Name)
	}
	want, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(p.Path, want) {
		t.Fatalf("path %q want %q", p.Path, want)
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

func TestRelocateEditorsRewritesPaths(t *testing.T) {
	oldRoot := filepath.Join(t.TempDir(), "old")
	newRoot := filepath.Join(t.TempDir(), "new")
	edDir := filepath.Join(oldRoot, "release", "0.6.1")
	bin := filepath.Join(edDir, "blazium.exe")
	if err := os.MkdirAll(edDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := File{
		InstallPath: oldRoot,
		Editors: []Editor{{
			Version: "0.6.1",
			Dir:     edDir,
			Path:    bin,
			Channel: ChannelRelease,
		}},
	}
	moved, err := f.RelocateEditors(oldRoot, newRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 1 || moved[0] != "0.6.1" {
		t.Fatalf("moved=%v", moved)
	}
	wantDir := filepath.Join(newRoot, "release", "0.6.1")
	if !samePath(f.Editors[0].Dir, wantDir) {
		t.Fatalf("dir %q want %q", f.Editors[0].Dir, wantDir)
	}
	wantBin := filepath.Join(wantDir, "blazium.exe")
	if !samePath(f.Editors[0].Path, wantBin) {
		t.Fatalf("path %q want %q", f.Editors[0].Path, wantBin)
	}
	if _, err := os.Stat(wantBin); err != nil {
		t.Fatal(err)
	}
	moved2, err := f.RelocateEditors(newRoot, newRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved2) != 0 {
		t.Fatalf("same-path move should be no-op: %v", moved2)
	}
}

func TestCreateProjectDir(t *testing.T) {
	withTempHub(t)
	dir := filepath.Join(t.TempDir(), "fresh-game")
	abs, err := CreateProjectDir(dir, "Fresh Game")
	if err != nil {
		t.Fatal(err)
	}
	if ProjectSettingsFile(abs) != ProjectFileBlazium {
		t.Fatalf("missing project.blazium")
	}
	if _, err := CreateProjectDir(dir, "Again"); err == nil {
		t.Fatal("expected reject existing project")
	}
	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.AddProject(abs)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Fresh Game" {
		t.Fatalf("name %q", p.Name)
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
