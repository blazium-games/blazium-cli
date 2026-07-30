package hub

import (
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
	got, err := ParseBlaziumURI("blazium://install?version=0.6.714&channel=release")
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "install" || got.Version != "0.6.714" || got.Channel != "release" {
		t.Fatalf("install: %#v", got)
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
