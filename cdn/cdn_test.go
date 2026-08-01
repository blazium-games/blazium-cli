package cdn

import "testing"

func TestFilenameMatchesEditorWindows64bit(t *testing.T) {
	name := "BlaziumEditor_v0.6.714_windows.64bit.zip"
	if !FilenameMatchesEditor(name, "0.6.714", "windows", "x86_64", false) {
		t.Fatal("expected windows x86_64 non-mono match")
	}
	if FilenameMatchesEditor(name, "0.6.714", "windows", "x86_64", true) {
		t.Fatal("mono flag should not match non-mono zip")
	}
}

func TestFilenameMatchesEditorLinuxX8664(t *testing.T) {
	name := "BlaziumEditor_v0.6.714_linux.x86_64.zip"
	if !FilenameMatchesEditor(name, "0.6.714", "linux", "x86_64", false) {
		t.Fatal("expected linux x86_64 match")
	}
}

func TestFindEditorMetadataFromManifest(t *testing.T) {
	list := []EditorMetadata{
		{Filename: "BlaziumEditor_v0.6.714_windows.64bit.zip"},
		{Filename: "BlaziumEditor_v0.6.714_windows.mono.64bit.zip"},
	}
	got, err := FindEditorMetadata(list, "0.6.714", "windows", "x86_64", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "BlaziumEditor_v0.6.714_windows.64bit.zip" {
		t.Fatalf("got %q", got.Filename)
	}
}

func TestCompareSemver(t *testing.T) {
	if CompareSemver("0.6.707", "0.6.705") <= 0 {
		t.Fatal("0.6.707 should be greater than 0.6.705")
	}
	if CompareSemver("0.5.421", "0.6.707") >= 0 {
		t.Fatal("0.5.421 should be less than 0.6.707")
	}
}

func TestResolveLatestNightlyMock(t *testing.T) {
	old := HTTPGet
	defer func() { HTTPGet = old }()
	HTTPGet = func(url string) ([]byte, error) {
		if url != "https://cdn.blazium.app/catalog/versions/nightly/latest.json" {
			t.Fatalf("unexpected url %s", url)
		}
		return []byte(`{"version":"0.6.707","channel":"nightly"}`), nil
	}
	got, err := ResolveLatestNightly()
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.6.707" {
		t.Fatalf("got %q want 0.6.707", got)
	}
}

func TestResolveInstallVersionAliases(t *testing.T) {
	old := HTTPGet
	defer func() { HTTPGet = old }()
	HTTPGet = func(url string) ([]byte, error) {
		switch {
		case url == "https://cdn.blazium.app/catalog/versions/nightly/latest.json":
			return []byte(`{"version":"0.6.800","channel":"nightly"}`), nil
		case url == "https://cdn.blazium.app/catalog/versions/release/latest.json":
			return []byte(`{"version":"0.6.725","channel":"release"}`), nil
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, nil
		}
	}
	v, night, err := ResolveInstallVersion("nightly", "", "0.6.1")
	if err != nil || !night || v != "0.6.800" {
		t.Fatalf("nightly: v=%q night=%v err=%v", v, night, err)
	}
	v, night, err = ResolveInstallVersion("latest", "release", "0.6.1")
	if err != nil || night || v != "0.6.725" {
		t.Fatalf("latest+release: v=%q night=%v err=%v", v, night, err)
	}
	v, night, err = ResolveInstallVersion("latest", "", "0.6.1")
	if err != nil || night || v != "0.6.725" {
		t.Fatalf("latest default release: v=%q night=%v err=%v", v, night, err)
	}
	v, night, err = ResolveInstallVersion("lts", "", "0.6.1")
	if err != nil || night || v != "0.6.1" {
		t.Fatalf("lts: v=%q night=%v err=%v", v, night, err)
	}
	v, night, err = ResolveInstallVersion("0.6.751", "nightly", "")
	if err != nil || !night || v != "0.6.751" {
		t.Fatalf("concrete nightly: v=%q night=%v err=%v", v, night, err)
	}
}

func TestEditorsJSONURL(t *testing.T) {
	if got := EditorsJSONURL("nightly", "0.6.751"); got != "https://cdn.blazium.app/nightly/0.6.751/editors.json" {
		t.Fatalf("got %q", got)
	}
	if got := EditorsJSONURL("", "0.6.725"); got != "https://cdn.blazium.app/release/0.6.725/editors.json" {
		t.Fatalf("empty channel -> release: %q", got)
	}
}
