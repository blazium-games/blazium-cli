package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"blazium-cli/editorinstall"
)

// FindEditor returns the registered editor for version, or nil.
func (f *File) FindEditor(version string) *Editor {
	version = strings.TrimSpace(version)
	for i := range f.Editors {
		if f.Editors[i].Version == version {
			return &f.Editors[i]
		}
	}
	return nil
}

// UpsertEditor adds or replaces an editor entry by version.
func (f *File) UpsertEditor(ed Editor) {
	for i := range f.Editors {
		if f.Editors[i].Version == ed.Version {
			f.Editors[i] = ed
			return
		}
	}
	f.Editors = append(f.Editors, ed)
}

// RemoveEditor removes a version from the registry. Returns the removed entry.
func (f *File) RemoveEditor(version string) (*Editor, error) {
	version = strings.TrimSpace(version)
	for i := range f.Editors {
		if f.Editors[i].Version != version {
			continue
		}
		ed := f.Editors[i]
		f.Editors = append(f.Editors[:i], f.Editors[i+1:]...)
		if f.DefaultEditor == version {
			f.DefaultEditor = ""
			if len(f.Editors) > 0 {
				f.DefaultEditor = f.Editors[0].Version
			}
		}
		return &ed, nil
	}
	return nil, fmt.Errorf("editor version %q is not registered", version)
}

// SetDefaultEditor sets the default editor version.
func (f *File) SetDefaultEditor(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return fmt.Errorf("version is required")
	}
	if f.FindEditor(version) == nil {
		return fmt.Errorf("editor version %q is not registered", version)
	}
	f.DefaultEditor = version
	return nil
}

// AddEditorFromPath registers an existing binary/dir.
func (f *File) AddEditorFromPath(path, version, platform, arch string, mono bool) (Editor, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Editor{}, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return Editor{}, err
	}
	bin := abs
	dir := abs
	if st.IsDir() {
		dir = abs
		found, findErr := editorinstall.FindBinaryInDir(abs)
		if findErr != nil {
			return Editor{}, findErr
		}
		bin = found
	} else {
		dir = filepath.Dir(abs)
	}
	if strings.TrimSpace(version) == "" {
		version = InferVersionFromPath(abs)
	}
	if version == "" {
		return Editor{}, fmt.Errorf("could not infer version; pass --version")
	}
	ed := Editor{
		Version:  version,
		Path:     bin,
		Dir:      dir,
		Platform: platform,
		Arch:     arch,
		Mono:     mono,
		Channel:  InferChannel(version, false),
	}
	f.UpsertEditor(ed)
	return ed, nil
}

// InferVersionFromPath guesses a version from path segments or filename.
func InferVersionFromPath(path string) string {
	base := filepath.Base(path)
	dir := filepath.Base(filepath.Dir(path))
	for _, candidate := range []string{dir, base} {
		if looksLikeVersion(candidate) {
			return candidate
		}
		// BlaziumEditor_v0.6.714_windows.64bit.exe
		if i := strings.Index(strings.ToLower(candidate), "_v"); i >= 0 {
			rest := candidate[i+2:]
			end := len(rest)
			for j, r := range rest {
				if r == '_' || r == '-' {
					end = j
					break
				}
			}
			v := rest[:end]
			if looksLikeVersion(v) {
				return v
			}
		}
	}
	return ""
}

func looksLikeVersion(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	dots := 0
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.':
			dots++
		default:
			return false
		}
	}
	return digits > 0 && dots >= 1
}
