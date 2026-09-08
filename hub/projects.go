package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ProjectFileBlazium = "project.blazium"
	ProjectFileGodot   = "project.godot"
)

// ProjectSettingsFile returns project.blazium or project.godot in dir (prefer blazium).
func ProjectSettingsFile(dir string) string {
	for _, name := range []string{ProjectFileBlazium, ProjectFileGodot} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !st.IsDir() {
			return name
		}
	}
	return ""
}

// ReadProjectSettingsText reads the preferred project settings file in dir.
func ReadProjectSettingsText(dir string) (string, error) {
	name := ProjectSettingsFile(dir)
	if name == "" {
		return "", fmt.Errorf("missing project.blazium or project.godot in %s", dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	return string(data), nil
}

// FindProject returns a project by absolute path or name (case-insensitive).
func (f *File) FindProject(pathOrName string) *Project {
	want := strings.TrimSpace(pathOrName)
	if want == "" {
		return nil
	}
	abs, absErr := filepath.Abs(want)
	for i := range f.Projects {
		p := &f.Projects[i]
		if absErr == nil && samePath(p.Path, abs) {
			return p
		}
		if strings.EqualFold(p.Name, want) {
			return p
		}
	}
	return nil
}

// AddProject registers a project directory that contains project.blazium or project.godot.
func (f *File) AddProject(path string) (Project, error) {
	abs, err := NormalizeProjectDir(path)
	if err != nil {
		return Project{}, err
	}
	name := projectDisplayName(abs)
	for i := range f.Projects {
		if samePath(f.Projects[i].Path, abs) {
			f.Projects[i].Name = name
			return f.Projects[i], nil
		}
	}
	p := Project{Path: abs, Name: name}
	f.Projects = append(f.Projects, p)
	return p, nil
}

// RemoveProject removes a project by path or name.
func (f *File) RemoveProject(pathOrName string) error {
	p := f.FindProject(pathOrName)
	if p == nil {
		return fmt.Errorf("project %q is not registered", pathOrName)
	}
	for i := range f.Projects {
		if samePath(f.Projects[i].Path, p.Path) {
			f.Projects = append(f.Projects[:i], f.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project %q is not registered", pathOrName)
}

// TouchProjectLastOpened updates last_opened and ensures the project is registered.
func (f *File) TouchProjectLastOpened(path string) error {
	p, err := f.AddProject(path)
	if err != nil {
		return err
	}
	for i := range f.Projects {
		if samePath(f.Projects[i].Path, p.Path) {
			f.Projects[i].LastOpened = NowRFC3339()
			return nil
		}
	}
	return nil
}

// IsProjectSettingsFile reports whether name is project.blazium or project.godot.
func IsProjectSettingsFile(name string) bool {
	base := filepath.Base(name)
	return base == ProjectFileBlazium || base == ProjectFileGodot
}

// NormalizeProjectDir accepts a project folder or a project.blazium/project.godot file.
func NormalizeProjectDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		if !IsProjectSettingsFile(abs) {
			return "", fmt.Errorf("not a directory: %s", path)
		}
		abs = filepath.Dir(abs)
	}
	if err := ValidateProjectDir(abs); err != nil {
		return "", err
	}
	return abs, nil
}

// ValidateProjectDir ensures path is a directory containing project.blazium or project.godot.
// CreateProjectDir writes a minimal project.blazium in dir. Fails if a project file already exists.
func CreateProjectDir(dir, name string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(abs); err == nil && !st.IsDir() {
		return "", fmt.Errorf("not a directory: %s", dir)
	}
	if ProjectSettingsFile(abs) != "" {
		return "", fmt.Errorf("already a project: %s", abs)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(abs)
	}
	name = strings.ReplaceAll(name, `"`, "")
	body := fmt.Sprintf("; Engine configuration file.\nconfig_version=5\n\n[application]\n\nconfig/name=\"%s\"\nconfig/features=PackedStringArray(\"4.5\", \"Forward Plus\")\n\n[rendering]\n\nrenderer/rendering_method=\"forward_plus\"\n", name)
	if err := os.WriteFile(filepath.Join(abs, ProjectFileBlazium), []byte(body), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}

func ValidateProjectDir(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	if ProjectSettingsFile(path) == "" {
		return fmt.Errorf("missing project.blazium or project.godot in %s", path)
	}
	return nil
}

func projectDisplayName(abs string) string {
	text, err := ReadProjectSettingsText(abs)
	if err == nil {
		if name := parseGodotSetting(text, "config/name"); name != "" {
			return strings.Trim(name, `"`)
		}
	}
	return filepath.Base(abs)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtimeEqualFoldPaths() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func runtimeEqualFoldPaths() bool {
	return filepath.Separator == '\\'
}
