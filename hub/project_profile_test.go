package hub

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectProfileJustAMCP(t *testing.T) {
	dir := t.TempDir()
	godot := `; Engine configuration file.
config_version=5

[application]

config/name="Demo"
config/features=PackedStringArray("4.3", "Forward Plus")

[blazium]

justamcp/server_enabled=true
justamcp/server_port=6506
justamcp/oauth_enabled=false
justamcp/client_id="abc"
remote_control/server_enabled=true
remote_control/allow_eval=true
blazium/editor_version="0.6.5"
`
	if err := os.WriteFile(filepath.Join(dir, "project.godot"), []byte(godot), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProjectProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Demo" {
		t.Fatalf("name %q", p.Name)
	}
	if !p.JustAMCP.Present || !p.JustAMCP.ServerEnabled || !p.JustAMCP.HasClientID {
		t.Fatalf("justamcp %+v", p.JustAMCP)
	}
	if !p.RemoteControl.Present || !p.RemoteControl.AllowEval {
		t.Fatalf("remote %+v", p.RemoteControl)
	}
	if p.EditorVersion != "0.6.5" {
		t.Fatalf("editor %q", p.EditorVersion)
	}
}

func TestBuildEditorArgs(t *testing.T) {
	args := BuildEditorArgs("/proj", 6508, "tok", true, 6509, true)
	joined := filepath.ToSlash(args[0] + args[1])
	_ = joined
	want := []string{
		"--path", "/proj",
		"--enable-remote-control",
		"--remote-control-port=6508",
		"--remote-control-token=tok",
		"--enable-mcp",
		"--mcp-port", "6509",
	}
	if len(args) != len(want) {
		t.Fatalf("args=%v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q", i, args[i], want[i])
		}
	}
}
