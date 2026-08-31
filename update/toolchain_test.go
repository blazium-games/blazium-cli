package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func toolchainManifestJSON(latest, platform, arch, url, sha string, size int) []byte {
	return []byte(fmt.Sprintf(`{
		"latest":%q,
		"versions":{
			%q:{"released_on":"2026-01-01T00:00:00Z","downloads":[
				{"platform":%q,"arch":%q,"filename":"blazium-toolchain","download_url":%q,"sha256":%q,"size":%d}
			]}
		}
	}`, latest, latest, platform, arch, url, sha, size))
}

func TestToolchainDest(t *testing.T) {
	got := ToolchainDest(`C:\Blazium`)
	if runtime.GOOS == "windows" {
		want := filepath.Join(`C:\Blazium`, "blazium-toolchain.exe")
		if got != want {
			t.Fatalf("dest=%s want %s", got, want)
		}
		return
	}
	want := filepath.Join(`C:\Blazium`, "bin", "blazium-toolchain")
	if got != want {
		t.Fatalf("dest=%s want %s", got, want)
	}
}

func TestCheckToolchainWithMock(t *testing.T) {
	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		if strings.Contains(u, "toolchain.json") {
			return toolchainManifestJSON("1.2.0", runtime.GOOS, runtimeArch(), "https://cdn.example/tc", "aa", 4), nil
		}
		return nil, fmt.Errorf("unexpected url %s", u)
	}
	root := t.TempDir()
	st := checkToolchain(root)
	if st.Error != "" {
		t.Fatalf("error: %s", st.Error)
	}
	if !st.UpdateAvailable {
		t.Fatalf("expected update when toolchain missing: %+v", st)
	}
	if st.LatestVersion != "1.2.0" {
		t.Fatalf("latest=%s", st.LatestVersion)
	}
	if st.Suggestion == "" {
		t.Fatal("expected apply suggestion")
	}

	dest := ToolchainDest(root)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toolchainVersionPath(dest), []byte("1.2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st2 := checkToolchain(root)
	if st2.UpdateAvailable {
		t.Fatalf("should be up to date: %+v", st2)
	}
	if st2.CurrentVersion != "1.2.0" {
		t.Fatalf("current=%s", st2.CurrentVersion)
	}
}

func TestCheckIncludesToolchain(t *testing.T) {
	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		return nil, fmt.Errorf("offline %s", u)
	}
	out, err := Check([]string{"toolchain"}, CheckOptions{InstallRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Product != "toolchain" {
		t.Fatalf("got %+v", out)
	}
}

func TestApplyToolchainDryPlanAndReplace(t *testing.T) {
	payload := []byte("official-toolchain")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		if strings.Contains(u, "toolchain.json") {
			return toolchainManifestJSON("0.4.0", runtime.GOOS, runtimeArch(), srv.URL+"/blazium-toolchain", hexSum, len(payload)), nil
		}
		return nil, fmt.Errorf("unexpected url %s", u)
	}

	root := t.TempDir()
	plan, err := ResolveToolchainPlan("", root)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Version != "0.4.0" || plan.DestPath != ToolchainDest(root) {
		t.Fatalf("plan=%+v", plan)
	}

	applied, err := ApplyToolchain("", root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(applied.DestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("dest contents %q", got)
	}
	ver, err := os.ReadFile(toolchainVersionPath(applied.DestPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != "0.4.0" {
		t.Fatalf("version file %q", ver)
	}
	st := checkToolchain(root)
	if st.UpdateAvailable {
		t.Fatalf("after apply should be current: %+v", st)
	}
}
