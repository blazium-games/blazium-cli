package main_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestLobbyCreateAndDelete(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "lobby", "add-game", "--lobby-control", "lua", "--game-id", "ea322545-4a60-4a3d-a94e-97cd6a30088c", "./tmp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run lobby create help: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Game ea322545-4a60-4a3d-a94e-97cd6a30088c created and added to games.ini with folder ./tmp") {
		t.Errorf("expected lobby create help output, got: %s", string(out))
	}
	cmd = exec.Command("go", "run", "./...", "lobby", "delete-game", "./tmp")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run lobby delete help: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Game ea322545-4a60-4a3d-a94e-97cd6a30088c deleted and removed from games.ini") {
		t.Errorf("expected lobby delete help output, got: %s", string(out))
	}
}

func TestLobbyHelp(t *testing.T) {
	cmd := exec.Command("go", "run", "./...", "lobby", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run lobby help: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Manage lobby server games and scripts") {
		t.Errorf("expected lobby help output, got: %s", string(out))
	}
}
