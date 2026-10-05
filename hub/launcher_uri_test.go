package hub

import (
	"path/filepath"
	"testing"
)

const exampleUID = "550e8400-e29b-41d4-a716-446655440000"

func TestLauncherURIStaysSeparateFromHubInstall(t *testing.T) {
	hub, err := ParseBlaziumURI("blazium://hub")
	if err != nil {
		t.Fatal(err)
	}
	if hub.Action != "hub" {
		t.Fatalf("hub action %q", hub.Action)
	}
	editor, err := ParseBlaziumURI("blazium://install?version=4.3")
	if err != nil {
		t.Fatal(err)
	}
	if editor.Action != "install" || editor.Version != "4.3" {
		t.Fatalf("editor install %#v", editor)
	}
	game, err := ParseBlaziumURI("blazium://install/" + exampleUID)
	if err != nil {
		t.Fatal(err)
	}
	if game.Action != "launcher" || game.Path != exampleUID {
		t.Fatalf("game install %#v", game)
	}
	page, err := ParseBlaziumURI("blazium://game/" + exampleUID)
	if err != nil {
		t.Fatal(err)
	}
	if page.Action != "launcher" || page.Path != exampleUID {
		t.Fatalf("game %#v", page)
	}
	buy, err := ParseBlaziumURI("blazium://buy/" + exampleUID)
	if err != nil {
		t.Fatal(err)
	}
	if buy.Action != "launcher" || buy.Path != exampleUID {
		t.Fatalf("buy must be forwarded, got %#v", buy)
	}
	if buy.Action == "install" {
		t.Fatal("buy ran an install")
	}
	if _, err := ParseBlaziumURI("blazium://not-a-real-host"); err == nil {
		t.Fatal("unknown host accepted")
	}
}

func TestLauncherRemotePortAndEnsure(t *testing.T) {
	if LauncherRemotePort != 39220 {
		t.Fatalf("port %d", LauncherRemotePort)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "launcher_remote.json")
	first, err := EnsureLauncherRemoteSecretAt(path, filepath.Join(dir, "BlaziumGames.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Port != 39220 || len(first.Token) < 16 {
		t.Fatalf("%#v", first)
	}
	again, err := EnsureLauncherRemoteSecretAt(path, first.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if again.Token != first.Token {
		t.Fatal("valid token was rotated")
	}
}
