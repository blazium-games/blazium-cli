package update

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestWindowsLauncherInstallerArgs(t *testing.T) {
	root := `C:\Program Files\Blazium\Games`
	args := WindowsLauncherInstallerArgs(root, false)
	want := []string{
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/SP-",
		"/NOCLI",
		`/DIR=C:\Program Files\Blazium\Games`,
	}
	if !slices.Equal(args, want) {
		t.Fatalf("WindowsLauncherInstallerArgs(launch=false) = %#v, want %#v", args, want)
	}
	if !slices.Contains(args, "/NOCLI") {
		t.Fatal("expected /NOCLI")
	}
	withLaunch := WindowsLauncherInstallerArgs(root, true)
	if !slices.Contains(withLaunch, "/LAUNCH") {
		t.Fatal("expected /LAUNCH")
	}
}

func TestResolveLauncherInstallRoot(t *testing.T) {
	t.Setenv("BLAZIUM", "")
	root := resolveLauncherInstallRoot("")
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(root, filepath.Join("Blazium", "Games")) {
			t.Fatalf("default root %q", root)
		}
	} else if root != filepath.Join("/opt/blazium", "games") {
		t.Fatalf("default root %q", root)
	}
	custom := `D:\Custom\Games`
	if got := resolveLauncherInstallRoot(custom); got != custom {
		t.Fatalf("explicit root %q", got)
	}
	t.Setenv("BLAZIUM", `D:\Blazium`)
	withEnv := resolveLauncherInstallRoot("")
	want := filepath.Join(`D:\Blazium`, "Games")
	if runtime.GOOS != "windows" {
		want = filepath.Join(`D:\Blazium`, "games")
	}
	if withEnv != want {
		t.Fatalf("BLAZIUM root %q want %q", withEnv, want)
	}
}

func TestResolveLauncherCurrentFromVersionFile(t *testing.T) {
	t.Setenv("BLAZIUM_LAUNCHER_VERSION", "")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveLauncherCurrent("", dir); got != "0.1.0" {
		t.Fatalf("version file %q", got)
	}
	if got := resolveLauncherCurrent("9.9.9", dir); got != "9.9.9" {
		t.Fatalf("explicit %q", got)
	}
}

func TestCheckLauncherWithMock(t *testing.T) {
	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		if strings.Contains(u, "launcher.json") {
			return []byte(`{
				"latest":"0.2.0",
				"versions":{"0.2.0":{"released_on":"2026-01-01T00:00:00Z","downloads":[
					{"platform":"windows","arch":"x86_64","filename":"BlaziumLauncher-Setup-0.2.0.exe","download_url":"https://cdn.example/BlaziumLauncher-Setup-0.2.0.exe","sha256":"aa","size":1},
					{"platform":"linux","arch":"x86_64","filename":"blazium-games_0.2.0_amd64.deb","download_url":"https://cdn.example/blazium-games.deb","sha256":"bb","size":2}
				]}}
			}`), nil
		}
		return nil, fmt.Errorf("unexpected url %s", u)
	}
	games := t.TempDir()
	st := checkLauncher("0.1.0", games)
	if st.Error != "" {
		t.Fatalf("error: %s", st.Error)
	}
	if !st.UpdateAvailable {
		t.Fatalf("expected update available: %+v", st)
	}
	if st.LatestVersion != "0.2.0" {
		t.Fatalf("latest=%s", st.LatestVersion)
	}
	if st.Suggestion != "blazium-cli update apply --product launcher" {
		t.Fatalf("suggestion %q", st.Suggestion)
	}
	plan, err := ResolveLauncherPlan("0.1.0", "", games)
	if err != nil {
		t.Fatal(err)
	}
	if plan.InstallRoot != games {
		t.Fatalf("install root %q", plan.InstallRoot)
	}
	if !strings.Contains(plan.URL, "BlaziumLauncher") && !strings.Contains(plan.URL, "blazium-games") {
		t.Fatalf("url %q", plan.URL)
	}
	st2 := checkLauncher("0.2.0", games)
	if st2.UpdateAvailable {
		t.Fatalf("should be up to date: %+v", st2)
	}
}
