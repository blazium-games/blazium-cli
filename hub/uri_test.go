package hub

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseBlaziumURIHub(t *testing.T) {
	for _, raw := range []string{
		"blazium://hub",
		"blazium://",
		"blazium:",
	} {
		got, err := ParseBlaziumURI(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if got.Action != "hub" {
			t.Fatalf("%q: action=%q want hub", raw, got.Action)
		}
	}
}

func TestParseBlaziumURIOpenLoad(t *testing.T) {
	open, err := ParseBlaziumURI("blazium://open?path=/tmp/MyGame")
	if err != nil {
		t.Fatal(err)
	}
	if open.Action != "open" || open.Path != "/tmp/MyGame" {
		t.Fatalf("open: %#v", open)
	}

	load, err := ParseBlaziumURI("blazium://load?path=C%3A%5CGames%5CFoo")
	if err != nil {
		t.Fatal(err)
	}
	if load.Action != "load" || load.Path != `C:\Games\Foo` {
		t.Fatalf("load: %#v", load)
	}
}

func TestParseBlaziumURIProjectShorthand(t *testing.T) {
	got, err := ParseBlaziumURI("blazium://project/C%3A%5CUsers%5Cdev%5CGame")
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "open" {
		t.Fatalf("action=%q", got.Action)
	}
	if got.Path != `C:\Users\dev\Game` {
		t.Fatalf("path=%q", got.Path)
	}
}

func TestParseBlaziumURIInstall(t *testing.T) {
	got, err := ParseBlaziumURI("blazium://install?version=0.6.714&channel=release&platform=windows&arch=x86_64&mono=true")
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "install" || got.Version != "0.6.714" || got.Channel != "release" {
		t.Fatalf("install: %#v", got)
	}
	if got.Platform != "windows" || got.Arch != "x86_64" || !got.Mono {
		t.Fatalf("install platform/arch/mono: %#v", got)
	}
}

func TestParseBlaziumURIRegister(t *testing.T) {
	got, err := ParseBlaziumURI("blazium://register?path=C%3A%5CEditors%5Cblazium.exe&version=0.6.714&channel=nightly&platform=windows&arch=x86_64&mono=1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "register" {
		t.Fatalf("action=%q", got.Action)
	}
	if got.Path != `C:\Editors\blazium.exe` {
		t.Fatalf("path=%q", got.Path)
	}
	if got.Version != "0.6.714" || got.Channel != "nightly" {
		t.Fatalf("version/channel: %#v", got)
	}
	if got.Platform != "windows" || got.Arch != "x86_64" || !got.Mono {
		t.Fatalf("platform/arch/mono: %#v", got)
	}
}

func TestParseBlaziumURIErrors(t *testing.T) {
	cases := []struct {
		raw string
	}{
		{""},
		{"https://example.com"},
		{"blazium://open"},
		{"blazium://load?channel=nightly"},
		{"blazium://project/"},
		{"blazium://install?channel=release"},
		{"blazium://register?version=0.6.1"},
		{"blazium://unknown?x=1"},
	}
	for _, tc := range cases {
		if _, err := ParseBlaziumURI(tc.raw); err == nil {
			t.Fatalf("expected error for %q", tc.raw)
		}
	}
}

func TestResolveURIPathPercentDecode(t *testing.T) {
	got, err := ResolveURIPath("C%3A%5CGames%5CFoo")
	if err != nil {
		t.Fatal(err)
	}
	want := `C:\Games\Foo`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveURIPathFileURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		got, err := ResolveURIPath("file:///C:/Games/Foo")
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.FromSlash("C:/Games/Foo")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
		return
	}
	got, err := ResolveURIPath("file:///tmp/MyGame")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/MyGame" {
		t.Fatalf("got %q", got)
	}
}

// TestHandleURIHub lives in hub_remote_test.go (requires live Hub remote_control fake).

func TestHandleURIRegisterIdempotent(t *testing.T) {
	withTempHub(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "blazium.exe")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	uri := "blazium://register?path=" + url.QueryEscape(bin) + "&version=0.6.714&channel=release&platform=windows&arch=x86_64"
	out, err := HandleURI(HandleURIOptions{URI: uri})
	if err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true || out["action"] != "register" || out["version"] != "0.6.714" {
		t.Fatalf("first: %#v", out)
	}
	if out["channel"] != "release" {
		t.Fatalf("channel=%v", out["channel"])
	}

	out2, err := HandleURI(HandleURIOptions{URI: uri})
	if err != nil {
		t.Fatal(err)
	}
	if out2["ok"] != true || out2["action"] != "register" {
		t.Fatalf("second: %#v", out2)
	}

	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Editors) != 1 {
		t.Fatalf("editors=%d", len(f.Editors))
	}
	if f.Editors[0].Version != "0.6.714" || f.Editors[0].EditorChannel() != ChannelRelease {
		t.Fatalf("%#v", f.Editors[0])
	}
}
