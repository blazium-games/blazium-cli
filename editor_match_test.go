package main

import "testing"

func TestFilenameMatchesEditorWindows64bit(t *testing.T) {
	name := "BlaziumEditor_v0.6.714_windows.64bit.zip"
	if !filenameMatchesEditor(name, "0.6.714", "windows", "x86_64", false) {
		t.Fatal("expected windows x86_64 non-mono match")
	}
	if filenameMatchesEditor(name, "0.6.714", "windows", "x86_64", true) {
		t.Fatal("mono flag should not match non-mono zip")
	}
}

func TestFilenameMatchesEditorLinuxX8664(t *testing.T) {
	name := "BlaziumEditor_v0.6.714_linux.x86_64.zip"
	if !filenameMatchesEditor(name, "0.6.714", "linux", "x86_64", false) {
		t.Fatal("expected linux x86_64 match")
	}
}

func TestFindEditorMetadataFromManifest(t *testing.T) {
	editorMetadata = []EditorMetadata{
		{Filename: "BlaziumEditor_v0.6.714_windows.64bit.zip"},
		{Filename: "BlaziumEditor_v0.6.714_windows.mono.64bit.zip"},
	}
	got, err := findEditorMetadata("0.6.714", "windows", "x86_64", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "BlaziumEditor_v0.6.714_windows.64bit.zip" {
		t.Fatalf("got %q", got.Filename)
	}
}
