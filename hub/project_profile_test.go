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
	args := BuildEditorArgs("/proj", 6508, "tok", true, 6509, true, "")
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

func TestBuildEditorArgsCrashReporter(t *testing.T) {
	args := BuildEditorArgs("/proj", 0, "", false, 0, false, `C:\Blazium\Hub\crash_reporter.exe`)
	want := []string{"--path", "/proj", "--crash-reporter", `C:\Blazium\Hub\crash_reporter.exe`}
	if len(args) != len(want) {
		t.Fatalf("args=%v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q", i, args[i], want[i])
		}
	}
}

func TestResolveCrashReporterPathExplicit(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "crash_reporter")
	if err := os.WriteFile(dest, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveCrashReporterPath(dest)
	abs, err := filepath.Abs(dest)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("got %q want %q", got, abs)
	}
	if ResolveCrashReporterPath(filepath.Join(dir, "missing")) != "" {
		t.Fatal("missing explicit path should be omitted")
	}
}

func TestNormalizeAnalyticsConsent(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"", "", false},
		{"unset", "", false},
		{"accepted", "accepted", false},
		{"ACCEPT", "accepted", false},
		{"true", "accepted", false},
		{"declined", "declined", false},
		{"0", "declined", false},
		{"maybe", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeAnalyticsConsent(c.in)
		if c.err {
			if err == nil {
				t.Fatalf("consent %q: want error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("consent %q: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("consent %q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeAnalyticsMode(t *testing.T) {
	got, err := NormalizeAnalyticsMode("ANONYMOUS")
	if err != nil || got != "anonymous" {
		t.Fatalf("anonymous: %q %v", got, err)
	}
	got, err = NormalizeAnalyticsMode("identified")
	if err != nil || got != "identified" {
		t.Fatalf("identified: %q %v", got, err)
	}
	if _, err := NormalizeAnalyticsMode("secret"); err == nil {
		t.Fatal("secret mode should error")
	}
	got, err = NormalizeAnalyticsMode("")
	if err != nil || got != "" {
		t.Fatalf("empty: %q %v", got, err)
	}
}

func TestAppendAnalyticsArgs(t *testing.T) {
	got := AppendAnalyticsArgs([]string{"--path", "/proj"}, "accepted", "anonymous")
	want := []string{"--path", "/proj", "--analytics=accepted", "--analytics-mode=anonymous"}
	if len(got) != len(want) {
		t.Fatalf("args=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q", i, got[i], want[i])
		}
	}
	got = AppendAnalyticsArgs([]string{"--path", "/proj"}, "declined", "")
	if len(got) != 3 || got[2] != "--analytics=declined" {
		t.Fatalf("declined args=%v", got)
	}
	got = AppendAnalyticsArgs([]string{"--path", "/proj"}, "", "")
	if len(got) != 2 {
		t.Fatalf("unset should not add flags: %v", got)
	}
}

func TestResolveCrashReporterPathFallback(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BLAZIUM", root)
	if ResolveCrashReporterPath("") != "" {
		t.Fatal("missing default sidecar should be omitted")
	}
	dest := DefaultCrashReporterDest()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveCrashReporterPath("")
	if got != dest {
		t.Fatalf("got %q want %q", got, dest)
	}
}
