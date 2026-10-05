package hub

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveHubExecutablePrefersEngine(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows install layout")
	}
	root := t.TempDir()
	t.Setenv("BLAZIUM", root)
	t.Setenv("ProgramFiles", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	engine := filepath.Join(root, "Engine", "BlaziumHub.exe")
	legacy := filepath.Join(root, "Hub", "BlaziumHub.exe")
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(engine, []byte("engine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("hub"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveHubExecutable()
	if err != nil {
		t.Fatal(err)
	}
	if got != engine {
		t.Fatalf("got %s want %s", got, engine)
	}
	if err := os.Remove(engine); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveHubExecutable()
	if err != nil {
		t.Fatal(err)
	}
	if got != legacy {
		t.Fatalf("got %s want %s", got, legacy)
	}
}

func TestResolveLauncherExecutablePrefersGamesFolder(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows install layout")
	}
	root := t.TempDir()
	t.Setenv("ProgramFiles", root)
	t.Setenv("BLAZIUM", filepath.Join(root, "Blazium"))
	games := filepath.Join(root, "Blazium", "Games", "BlaziumLauncher.exe")
	recorded := filepath.Join(root, "BlaziumGames.exe")
	legacy := filepath.Join(root, "Blazium Games", "BlaziumGames.exe")
	for _, path := range []string{games, recorded, legacy} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolveLauncherExecutable(recorded)
	if err != nil {
		t.Fatal(err)
	}
	if got != games {
		t.Fatalf("got %s want %s", got, games)
	}
	if err := os.Remove(games); err != nil {
		t.Fatal(err)
	}
	got, err = resolveLauncherExecutable(recorded)
	if err != nil {
		t.Fatal(err)
	}
	if got != recorded {
		t.Fatalf("got %s want %s", got, recorded)
	}
	if err := os.Remove(recorded); err != nil {
		t.Fatal(err)
	}
	got, err = resolveLauncherExecutable("")
	if err != nil {
		t.Fatal(err)
	}
	if got != legacy {
		t.Fatalf("got %s want %s", got, legacy)
	}
}

func TestResolveLauncherExecutableMissing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows install layout")
	}
	t.Setenv("ProgramFiles", t.TempDir())
	t.Setenv("BLAZIUM", t.TempDir())
	_, err := resolveLauncherExecutable("")
	if err == nil || err.Error() != "games launcher executable not found" {
		t.Fatalf("err=%v", err)
	}
}
