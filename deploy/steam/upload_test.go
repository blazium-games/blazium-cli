package steam

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestUploadDryRun(t *testing.T) {
	dir := t.TempDir()
	res, err := Upload(context.Background(), UploadInput{
		DryRun:  true,
		WorkDir: dir,
		App: AppBuild{
			AppID: "42",
			Depots: []Depot{
				{ID: "43", Path: "win64"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.AppVDF == "" {
		t.Fatalf("%+v", res)
	}
	b, err := os.ReadFile(res.AppVDF)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"AppID" "42"`) {
		t.Fatalf("%s", b)
	}
}
