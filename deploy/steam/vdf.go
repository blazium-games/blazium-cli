package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Depot struct {
	ID          string
	Path        string
	Description string
}

type AppBuild struct {
	AppID       string
	Description string
	ContentRoot string
	OutputDir   string
	Depots      []Depot
	SetLive     string
}

func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// WriteVDFs writes app_build.vdf and depot_build_*.vdf into dir. Returns app vdf path.
func WriteVDFs(dir string, b AppBuild) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if b.OutputDir == "" {
		b.OutputDir = filepath.Join(dir, "BuildOutput")
	}
	if err := os.MkdirAll(b.OutputDir, 0o755); err != nil {
		return "", err
	}
	var depotSection strings.Builder
	for _, d := range b.Depots {
		name := fmt.Sprintf("depot_build_%s.vdf", d.ID)
		path := filepath.Join(dir, name)
		body := fmt.Sprintf("\"DepotBuild\"\n{\n    \"DepotID\" \"%s\"\n    \"FileMapping\"\n    {\n        \"LocalPath\" \"%s/*\"\n        \"DepotPath\" \".\"\n        \"recursive\" \"1\"\n    }\n}\n",
			quote(d.ID), quote(filepath.ToSlash(d.Path)))
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
		depotSection.WriteString(fmt.Sprintf("        \"%s\" \"%s\"\n", quote(d.ID), quote(name)))
	}
	desc := b.Description
	if desc == "" {
		desc = time.Now().UTC().Format("2006-01-02 15:04:05")
	}
	appPath := filepath.Join(dir, "app_build.vdf")
	app := fmt.Sprintf("\"AppBuild\"\n{\n    \"AppID\" \"%s\"\n    \"Desc\" \"%s\"\n    \"BuildOutput\" \"%s\"\n    \"ContentRoot\" \"%s\"\n    \"SetLive\" \"%s\"\n    \"Depots\"\n    {\n%s    }\n}\n",
		quote(b.AppID), quote(desc), quote(filepath.ToSlash(b.OutputDir)), quote(filepath.ToSlash(b.ContentRoot)), quote(b.SetLive), depotSection.String())
	if err := os.WriteFile(appPath, []byte(app), 0o644); err != nil {
		return "", err
	}
	return appPath, nil
}
