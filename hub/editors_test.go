package hub

import (
	"path/filepath"
	"testing"
)

func TestEditorInstallDir(t *testing.T) {
	got := EditorInstallDir("/opt/editors", "nightly", "0.6.751")
	want := filepath.Join("/opt/editors", "nightly", "0.6.751")
	if got != want {
		t.Fatalf("nightly dir: %q want %q", got, want)
	}
	got = EditorInstallDir("/opt/editors", "", "0.6.725")
	want = filepath.Join("/opt/editors", "release", "0.6.725")
	if got != want {
		t.Fatalf("release dir: %q want %q", got, want)
	}
	got = EditorInstallDir("/opt/editors", "prerelease", "0.6.700")
	want = filepath.Join("/opt/editors", "prerelease", "0.6.700")
	if got != want {
		t.Fatalf("prerelease dir: %q want %q", got, want)
	}
}

func TestFindUpsertRemoveByChannel(t *testing.T) {
	f := &File{}
	f.UpsertEditor(Editor{Version: "0.6.725", Channel: ChannelRelease, Path: "/r"})
	f.UpsertEditor(Editor{Version: "0.6.751", Channel: ChannelNightly, Path: "/n"})
	f.UpsertEditor(Editor{Version: "0.6.725", Channel: ChannelNightly, Path: "/n725"})

	if ed := f.FindEditor("0.6.725", ChannelRelease); ed == nil || ed.Path != "/r" {
		t.Fatalf("release find: %+v", ed)
	}
	if ed := f.FindEditor("0.6.725", ChannelNightly); ed == nil || ed.Path != "/n725" {
		t.Fatalf("nightly same-version find: %+v", ed)
	}
	if _, err := f.RemoveEditor("0.6.725", ""); err == nil {
		t.Fatal("expected error when multiple channels share version")
	}
	ed, err := f.RemoveEditor("0.6.725", ChannelRelease)
	if err != nil || ed.Path != "/r" {
		t.Fatalf("remove release: ed=%+v err=%v", ed, err)
	}
	if f.FindEditor("0.6.725", ChannelNightly) == nil {
		t.Fatal("nightly twin should remain")
	}
}
