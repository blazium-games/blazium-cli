package hub

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reEditorVersion = regexp.MustCompile(`(?m)^blazium/editor_version\s*=\s*"?([^"\r\n]+)"?`)
	reConfigName    = regexp.MustCompile(`(?m)^config/name\s*=\s*"([^"]*)"`)
	reFeatures      = regexp.MustCompile(`(?m)^config/features\s*=\s*PackedStringArray\(([^)]*)\)`)
)

// ResolveProjectPath turns a path or registered project name into an absolute project dir.
func (f *File) ResolveProjectPath(pathOrName string) (string, error) {
	pathOrName = strings.TrimSpace(pathOrName)
	if pathOrName == "" {
		return "", fmt.Errorf("project path or name is required")
	}
	if abs, err := NormalizeProjectDir(pathOrName); err == nil {
		return abs, nil
	}
	if p := f.FindProject(pathOrName); p != nil {
		return NormalizeProjectDir(p.Path)
	}
	return "", fmt.Errorf("project not found: %s (need a directory with project.blazium or project.godot, or a registered name)", pathOrName)
}

// ResolveEditorForProject picks an installed editor for a project.
// Order: blazium/editor_version → config/features match → default editor.
func (f *File) ResolveEditorForProject(projectPath string) (*Editor, string, error) {
	text, err := ReadProjectSettingsText(projectPath)
	if err != nil {
		return nil, "", err
	}

	if m := reEditorVersion.FindStringSubmatch(text); len(m) == 2 {
		ver := strings.TrimSpace(m[1])
		if ed := f.FindEditor(ver); ed != nil {
			return ed, "blazium/editor_version", nil
		}
		return nil, "", fmt.Errorf("project requests editor %q but it is not installed", ver)
	}

	if feats := parseFeatures(text); len(feats) > 0 {
		for _, feat := range feats {
			if ed := f.matchEditorByFeature(feat); ed != nil {
				return ed, "config/features", nil
			}
		}
	}

	ed, err := f.ResolveDefault()
	if err != nil {
		return nil, "", err
	}
	return ed, "default", nil
}

func (f *File) matchEditorByFeature(feat string) *Editor {
	feat = strings.TrimSpace(feat)
	if feat == "" {
		return nil
	}
	for i := range f.Editors {
		v := f.Editors[i].Version
		if v == feat || strings.HasPrefix(v, feat+".") || strings.HasPrefix(v, feat+"-") {
			return &f.Editors[i]
		}
		// feature "4.3" vs editor "0.6.0" won't match — also try contains for blazium versions
		if strings.HasPrefix(feat, v) {
			return &f.Editors[i]
		}
	}
	return nil
}

func parseFeatures(text string) []string {
	m := reFeatures.FindStringSubmatch(text)
	if len(m) != 2 {
		return nil
	}
	inner := m[1]
	var out []string
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseGodotSetting(text, key string) string {
	if key == "config/name" {
		if m := reConfigName.FindStringSubmatch(text); len(m) == 2 {
			return m[1]
		}
	}
	return ""
}
