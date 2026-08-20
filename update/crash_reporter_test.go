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

func crashReporterManifestJSON(latest, platform, arch, url, sha string, size int) []byte {
	return []byte(fmt.Sprintf(`{
		"latest":%q,
		"versions":{
			%q:{"released_on":"2026-01-01T00:00:00Z","downloads":[
				{"platform":%q,"arch":%q,"filename":"crash_reporter","download_url":%q,"sha256":%q,"size":%d}
			]}
		}
	}`, latest, latest, platform, arch, url, sha, size))
}

func TestCrashReporterDest(t *testing.T) {
	got := CrashReporterDest(`C:\Blazium`)
	if runtime.GOOS == "windows" {
		want := filepath.Join(`C:\Blazium`, "Hub", "crash_reporter.exe")
		if got != want {
			t.Fatalf("dest=%s want %s", got, want)
		}
		return
	}
	want := filepath.Join(`C:\Blazium`, "bin", "crash_reporter")
	if got != want {
		t.Fatalf("dest=%s want %s", got, want)
	}
}

func TestCheckCrashReporterWithMock(t *testing.T) {
	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		if strings.Contains(u, "crash_reporter.json") {
			return crashReporterManifestJSON("1.2.0", runtime.GOOS, runtimeArch(), "https://cdn.example/cr", "aa", 4), nil
		}
		return nil, fmt.Errorf("unexpected url %s", u)
	}
	root := t.TempDir()
	st := checkCrashReporter(root)
	if st.Error != "" {
		t.Fatalf("error: %s", st.Error)
	}
	if !st.UpdateAvailable {
		t.Fatalf("expected update when sidecar missing: %+v", st)
	}
	if st.LatestVersion != "1.2.0" {
		t.Fatalf("latest=%s", st.LatestVersion)
	}
	if st.Suggestion == "" {
		t.Fatal("expected apply suggestion")
	}

	dest := CrashReporterDest(root)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(crashReporterVersionPath(dest), []byte("1.2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st2 := checkCrashReporter(root)
	if st2.UpdateAvailable {
		t.Fatalf("should be up to date: %+v", st2)
	}
	if st2.CurrentVersion != "1.2.0" {
		t.Fatalf("current=%s", st2.CurrentVersion)
	}
}

func TestResolveCrashReporterCurrentFromSHA(t *testing.T) {
	payload := []byte("sidecar-body")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	doc, err := ParseHubManifest(crashReporterManifestJSON("3.0.0", runtime.GOOS, runtimeArch(), "https://cdn.example/cr", hexSum, len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "crash_reporter")
	if err := os.WriteFile(dest, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveCrashReporterCurrent(dest, doc)
	if got != "3.0.0" {
		t.Fatalf("current=%s", got)
	}
}

func TestCheckIncludesCrashReporter(t *testing.T) {
	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		return nil, fmt.Errorf("offline %s", u)
	}
	out, err := Check([]string{"crash_reporter"}, CheckOptions{InstallRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Product != "crash_reporter" {
		t.Fatalf("got %+v", out)
	}
	unknown, err := Check([]string{"nope"}, CheckOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if unknown[0].Error == "" || !strings.Contains(unknown[0].Error, "unknown product") {
		t.Fatalf("expected unknown product: %+v", unknown[0])
	}
}

func TestApplyCrashReporterDryPlanAndReplace(t *testing.T) {
	payload := []byte("official-sidecar")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		if strings.Contains(u, "crash_reporter.json") {
			return crashReporterManifestJSON("0.4.0", runtime.GOOS, runtimeArch(), srv.URL+"/crash_reporter", hexSum, len(payload)), nil
		}
		return nil, fmt.Errorf("unexpected url %s", u)
	}

	root := t.TempDir()
	plan, err := ResolveCrashReporterPlan("", root)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Version != "0.4.0" || plan.DestPath != CrashReporterDest(root) {
		t.Fatalf("plan=%+v", plan)
	}
	if plan.SHA256 != hexSum {
		t.Fatalf("sha=%s", plan.SHA256)
	}

	applied, err := ApplyCrashReporter("", root)
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
	ver, err := os.ReadFile(crashReporterVersionPath(applied.DestPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != "0.4.0" {
		t.Fatalf("version file %q", ver)
	}
	st := checkCrashReporter(root)
	if st.UpdateAvailable {
		t.Fatalf("after apply should be current: %+v", st)
	}
}
