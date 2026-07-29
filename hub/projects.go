package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

// AddProject registers a project directory that contains project.godot.
func (f *File) AddProject(path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	if err := ValidateProjectDir(abs); err != nil {
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

// ValidateProjectDir ensures path is a directory containing project.godot.
func ValidateProjectDir(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	godot := filepath.Join(path, "project.godot")
	if _, err := os.Stat(godot); err != nil {
		return fmt.Errorf("missing project.godot in %s", path)
	}
	return nil
}

func projectDisplayName(abs string) string {
	cfg := filepath.Join(abs, "project.godot")
	data, err := os.ReadFile(cfg)
	if err == nil {
		if name := parseGodotSetting(string(data), "config/name"); name != "" {
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
