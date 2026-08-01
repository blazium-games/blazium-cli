package update

import (
	"fmt"
	"strings"
	"testing"

	"github.com/blazium-games/blazium-cli/cdn"
)

func TestArchMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"x86_64", "x86_64", true},
		{"amd64", "x86_64", true},
		{"x86_32", "386", true},
		{"i386", "x86_32", true},
		{"x86_64", "x86_32", false},
	}
	for _, tc := range cases {
		if got := archMatch(tc.a, tc.b); got != tc.want {
			t.Fatalf("archMatch(%q,%q)=%v want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestPickDownload(t *testing.T) {
	body := []byte(`{
		"latest": "0.1.1",
		"versions": {
			"0.1.1": {
				"released_on": "2026-07-31T00:00:00Z",
				"downloads": [
					{"platform":"linux","arch":"x86_64","filename":"hub.deb","download_url":"https://example/hub.deb","sha256":"abc","size":10},
					{"platform":"windows","arch":"x86_32","filename":"hub.exe","download_url":"https://example/hub.exe","sha256":"def","size":20}
				]
			}
		}
	}`)
	doc, err := ParseHubManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	dl, err := pickDownload(doc, "0.1.1", "linux", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if dl.Filename != "hub.deb" {
		t.Fatalf("got %s", dl.Filename)
	}
	dl, err = pickDownload(doc, "0.1.1", "windows", "x86_32")
	if err != nil {
		t.Fatal(err)
	}
	if dl.Filename != "hub.exe" {
		t.Fatalf("got %s", dl.Filename)
	}
	if _, err := pickDownload(doc, "0.1.1", "darwin", "x86_64"); err == nil {
		t.Fatal("expected missing darwin download")
	}
}

func TestUpdateAvailableSemver(t *testing.T) {
	if cdn.CompareSemver("0.1.1", "0.1.0") <= 0 {
		t.Fatal("expected newer")
	}
	if cdn.CompareSemver("0.1.0", "0.1.0") != 0 {
		t.Fatal("expected equal")
	}
}

func TestCheckHubWithMock(t *testing.T) {
	orig := HTTPGet
	t.Cleanup(func() { HTTPGet = orig })
	HTTPGet = func(u string) ([]byte, error) {
		if strings.Contains(u, "hub.json") || strings.Contains(u, "manifest.json") {
			return []byte(`{
				"latest":"0.2.0",
				"versions":{"0.2.0":{"released_on":"2026-01-01T00:00:00Z","downloads":[
					{"platform":"windows","arch":"x86_64","filename":"Setup.exe","download_url":"https://cdn.example/Setup.exe","sha256":"aa","size":1},
					{"platform":"linux","arch":"x86_64","filename":"hub.deb","download_url":"https://cdn.example/hub.deb","sha256":"bb","size":2}
				]}}
			}`), nil
		}
		return nil, fmt.Errorf("unexpected url %s", u)
	}
	st := checkHub("0.1.0", "")
	if st.Error != "" {
		t.Fatalf("error: %s", st.Error)
	}
	if !st.UpdateAvailable {
		t.Fatalf("expected update available: %+v", st)
	}
	if st.LatestVersion != "0.2.0" {
		t.Fatalf("latest=%s", st.LatestVersion)
	}
	st2 := checkHub("0.2.0", "")
	if st2.UpdateAvailable {
		t.Fatalf("should be up to date: %+v", st2)
	}
}

func TestLooksLikeVersion(t *testing.T) {
	if !looksLikeVersion("0.6.748") {
		t.Fatal("expected version")
	}
	if looksLikeVersion("..") {
		t.Fatal("not a version")
	}
}
