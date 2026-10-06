package hub

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindStarterAndDownload(t *testing.T) {
	zipBytes := starterZip(t, map[string]string{
		"project.blazium": "config/name=\"Empty\"\n",
		"README.md":       "hello\n",
	})
	sum := sha256.Sum256(zipBytes)
	digest := hex.EncodeToString(sum[:])

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/starters.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"latest":"0.1.0","starters":[{"name":"template_2d_empty","description":"A blank 2D root.","file":"` + server.URL + `/template_2d_empty.zip","sha256":"` + digest + `","size":1,"repo":"blazium-games/template_2d_empty","commit":"abc","github":"https://github.com/blazium-games/template_2d_empty","git":"https://github.com/blazium-games/template_2d_empty.git"}]}`))
		case "/template_2d_empty.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	catalog, err := LoadStartersCatalog(server.URL + "/starters.json")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Latest != "0.1.0" || len(catalog.Starters) != 1 {
		t.Fatalf("catalog %+v", catalog)
	}
	starter, err := FindStarter(catalog, "template_2d_empty")
	if err != nil {
		t.Fatal(err)
	}
	if starter.GitHub != "https://github.com/blazium-games/template_2d_empty" {
		t.Fatalf("github %s", starter.GitHub)
	}
	if _, err := FindStarter(catalog, "missing"); err == nil {
		t.Fatal("expected missing starter")
	}

	dest := t.TempDir()
	nested := filepath.Join(dest, "game")
	if err := DownloadStarter(starter, nested); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(nested, "project.blazium"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Empty") {
		t.Fatalf("project file %q", body)
	}
	if err := DownloadStarter(starter, nested); err == nil {
		t.Fatal("expected refuse of a non-empty directory")
	}
}

func TestDownloadRejectsBadHashAndZipSlip(t *testing.T) {
	zipBytes := starterZip(t, map[string]string{"ok.txt": "ok"})
	starter := Starter{
		Name:   "template_web",
		File:   "",
		SHA256: "deadbeef",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	}))
	defer server.Close()
	starter.File = server.URL + "/bad.zip"
	if err := DownloadStarter(starter, t.TempDir()); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("got %v", err)
	}

	slip := filepath.Join(t.TempDir(), "slip.zip")
	writeZip(t, slip, map[string]string{"../outside.txt": "nope"})
	if err := unpackStarterZip(slip, t.TempDir()); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("got %v", err)
	}
}

func starterZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "starter.zip")
	writeZip(t, path, files)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	for name, body := range files {
		entry, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
