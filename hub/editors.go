package hub

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/blazium-games/blazium-cli/editorinstall"
)

// EditorInstallDir returns the channel-scoped install directory.
// Layout: {installRoot}/{channel}/{version}
func EditorInstallDir(installRoot, channel, version string) string {
	ch, err := NormalizeChannel(channel)
	if err != nil {
		ch = ChannelRelease
	}
	return filepath.Join(installRoot, ch, strings.TrimSpace(version))
}

// FindEditor returns the registered editor for version (+ optional channel).
// With channel empty, returns the first version match (legacy).
func (f *File) FindEditor(version string, channel ...string) *Editor {
	version = strings.TrimSpace(version)
	ch := ""
	if len(channel) > 0 {
		ch = strings.TrimSpace(channel[0])
	}
	if ch != "" {
		norm, err := NormalizeChannel(ch)
		if err != nil {
			return nil
		}
		for i := range f.Editors {
			if f.Editors[i].Version == version && f.Editors[i].EditorChannel() == norm {
				return &f.Editors[i]
			}
		}
		return nil
	}
	for i := range f.Editors {
		if f.Editors[i].Version == version {
			return &f.Editors[i]
		}
	}
	return nil
}

// UpsertEditor adds or replaces an editor entry by version+channel.
func (f *File) UpsertEditor(ed Editor) {
	ch := ed.EditorChannel()
	ed.Channel = ch
	for i := range f.Editors {
		if f.Editors[i].Version == ed.Version && f.Editors[i].EditorChannel() == ch {
			f.Editors[i] = ed
			return
		}
	}
	f.Editors = append(f.Editors, ed)
}

// RemoveEditor removes a version (+ optional channel) from the registry.
// When channel is empty and multiple editors share the version, returns an error.
func (f *File) RemoveEditor(version, channel string) (*Editor, error) {
	version = strings.TrimSpace(version)
	channel = strings.TrimSpace(channel)

	if channel != "" {
		norm, err := NormalizeChannel(channel)
		if err != nil {
			return nil, err
		}
		for i := range f.Editors {
			if f.Editors[i].Version != version || f.Editors[i].EditorChannel() != norm {
				continue
			}
			return f.removeEditorAt(i), nil
		}
		return nil, fmt.Errorf("editor version %q channel %q is not registered", version, norm)
	}

	var idxs []int
	for i := range f.Editors {
		if f.Editors[i].Version == version {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) == 0 {
		return nil, fmt.Errorf("editor version %q is not registered", version)
	}
	if len(idxs) > 1 {
		chans := make([]string, 0, len(idxs))
		for _, i := range idxs {
			chans = append(chans, f.Editors[i].EditorChannel())
		}
		return nil, fmt.Errorf("multiple editors for version %q (%s); pass --channel", version, strings.Join(chans, ", "))
	}
	return f.removeEditorAt(idxs[0]), nil
}

func (f *File) removeEditorAt(i int) *Editor {
	ed := f.Editors[i]
	f.Editors = append(f.Editors[:i], f.Editors[i+1:]...)
	if f.DefaultEditor == ed.Version {
		f.DefaultEditor = ""
		if len(f.Editors) > 0 {
			f.DefaultEditor = f.Editors[0].Version
		}
	}
	return &ed
}

// SetDefaultEditor sets the default editor version (hard pin).
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
// Channel may be empty; when empty, InferChannel is used, then parent-dir channel
// inference (…/nightly/0.6.751) if applicable.
func (f *File) AddEditorFromPath(path, version, platform, arch, channel string, mono bool) (Editor, error) {
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
	if strings.TrimSpace(channel) != "" {
		ch, err := NormalizeChannel(channel)
		if err != nil {
			return Editor{}, err
		}
		ed.Channel = ch
	} else {
		// Prefer parent dir name when it is a known channel (…/nightly/0.6.751).
		parent := filepath.Base(filepath.Dir(dir))
		if ch, err := NormalizeChannel(parent); err == nil {
			ed.Channel = ch
		}
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

// RelocateEditors moves editor trees that live under oldRoot into newRoot and rewrites Dir/Path.
func (f *File) RelocateEditors(oldRoot, newRoot string) (moved []string, err error) {
	oldAbs, err := filepath.Abs(strings.TrimSpace(oldRoot))
	if err != nil {
		return nil, err
	}
	newAbs, err := filepath.Abs(strings.TrimSpace(newRoot))
	if err != nil {
		return nil, err
	}
	if samePath(oldAbs, newAbs) {
		return nil, nil
	}
	for i := range f.Editors {
		ed := &f.Editors[i]
		relDir, ok := relUnderRoot(ed.Dir, oldAbs)
		if !ok {
			continue
		}
		destDir := filepath.Join(newAbs, relDir)
		if err := moveDir(ed.Dir, destDir); err != nil {
			return moved, fmt.Errorf("move %s -> %s: %w", ed.Dir, destDir, err)
		}
		if relBin, ok := relUnderRoot(ed.Path, ed.Dir); ok {
			ed.Path = filepath.Join(destDir, relBin)
		} else if relBin, ok := relUnderRoot(ed.Path, oldAbs); ok {
			ed.Path = filepath.Join(newAbs, relBin)
		}
		ed.Dir = destDir
		moved = append(moved, ed.Version)
	}
	return moved, nil
}

func relUnderRoot(path, root string) (string, bool) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || abs == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return rel, true
}

func moveDir(from, to string) error {
	from = filepath.Clean(from)
	to = filepath.Clean(to)
	if samePath(from, to) {
		return nil
	}
	if _, err := os.Stat(from); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	if err := copyDir(from, to); err != nil {
		_ = os.RemoveAll(to)
		return err
	}
	return os.RemoveAll(from)
}

func copyDir(from, to string) error {
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode())
		}
		return copyFileMode(path, dest, info.Mode())
	})
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
