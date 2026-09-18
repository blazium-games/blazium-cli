package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteVDFs(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteVDFs(dir, AppBuild{
		AppID:       "1000",
		Description: "1.0.0",
		ContentRoot: dir,
		Depots: []Depot{
			{ID: "1001", Path: "win64"},
			{ID: "1002", Path: "linux64"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"AppID" "1000"`, `"SetLive" ""`, `"1001" "depot_build_1001.vdf"`, `"1002" "depot_build_1002.vdf"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	depot, err := os.ReadFile(filepath.Join(dir, "depot_build_1001.vdf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(depot), `"DepotID" "1001"`) {
		t.Fatalf("%s", depot)
	}
}
