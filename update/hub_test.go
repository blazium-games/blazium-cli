package update

import (
	"slices"
	"testing"
)

func TestWindowsHubInstallerArgs_noLaunchByDefault(t *testing.T) {
	args := WindowsHubInstallerArgs(`C:\Program Files\Blazium`, false)
	want := []string{
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/SP-",
		`/DIR=C:\Program Files\Blazium`,
	}
	if !slices.Equal(args, want) {
		t.Fatalf("WindowsHubInstallerArgs(launch=false) = %#v, want %#v", args, want)
	}
	if slices.Contains(args, "/LAUNCH") {
		t.Fatal("expected no /LAUNCH when launch=false")
	}
}

func TestWindowsHubInstallerArgs_withLaunch(t *testing.T) {
	args := WindowsHubInstallerArgs(`C:\Program Files\Blazium`, true)
	want := []string{
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/SP-",
		`/DIR=C:\Program Files\Blazium`,
		"/LAUNCH",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("WindowsHubInstallerArgs(launch=true) = %#v, want %#v", args, want)
	}
}
