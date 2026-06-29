package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func defaultTemplatesDest() string {
	if v := strings.TrimSpace(os.Getenv("BLAZIUM_EXPORT_TEMPLATES_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/root", ".local", "share", "blazium", "export_templates")
	}
	return filepath.Join(home, ".local", "share", "blazium", "export_templates")
}

// InstallTemplatesFromTPZ extracts export template zips from a .tpz into destRoot/version/.
func InstallTemplatesFromTPZ(tpzPath, destRoot string) (string, error) {
	r, err := zip.OpenReader(tpzPath)
	if err != nil {
		return "", fmt.Errorf("open tpz: %w", err)
	}
	defer r.Close()

	version, contentsPrefix, err := readTPZVersion(r.File)
	if err != nil {
		return "", err
	}

	templateDir := filepath.Join(destRoot, version)
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir template dir: %w", err)
	}

	for _, f := range r.File {
		name := filepath.ToSlash(f.Name)
		if strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		if strings.HasSuffix(name, "/") {
			continue
		}
		base := filepath.Base(name)
		if base == "" || base == "version.txt" {
			continue
		}
		rel := name
		if contentsPrefix != "" && strings.HasPrefix(rel, contentsPrefix+"/") {
			rel = strings.TrimPrefix(rel, contentsPrefix+"/")
		}
		if rel == "" {
			continue
		}
		outPath := filepath.Join(templateDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
		}
		if err := extractZipFile(f, outPath); err != nil {
			return "", fmt.Errorf("extract %s: %w", name, err)
		}
	}

	if err := ensureTemplateAlias(destRoot, version); err != nil {
		return "", err
	}
	return version, nil
}

func readTPZVersion(files []*zip.File) (version, contentsPrefix string, err error) {
	for _, f := range files {
		if !strings.HasSuffix(filepath.ToSlash(f.Name), "version.txt") {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return "", "", fmt.Errorf("open version.txt: %w", openErr)
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return "", "", fmt.Errorf("read version.txt: %w", readErr)
		}
		version = strings.TrimSpace(string(data))
		if strings.Count(version, ".") < 2 {
			return "", "", fmt.Errorf("invalid version.txt: %q", version)
		}
		dir := filepath.ToSlash(filepath.Dir(f.Name))
		dir = strings.TrimSuffix(dir, "/")
		return version, dir, nil
	}
	return "", "", fmt.Errorf("version.txt not found in tpz")
}

// InstallTemplateFiles copies downloaded template zips into destRoot/engineVersion/.
func InstallTemplateFiles(destRoot, engineVersion string, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no template files to install")
	}
	templateDir := filepath.Join(destRoot, engineVersion)
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		return fmt.Errorf("mkdir template dir: %w", err)
	}
	for _, src := range files {
		base := filepath.Base(src)
		if base == "" || base == "." {
			return fmt.Errorf("invalid template file path: %s", src)
		}
		dst := filepath.Join(templateDir, base)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("install %s: %w", base, err)
		}
	}
	versionFile := filepath.Join(templateDir, "version.txt")
	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		if err := os.WriteFile(versionFile, []byte(engineVersion), 0o644); err != nil {
			return fmt.Errorf("write version.txt: %w", err)
		}
	}
	return ensureTemplateAlias(destRoot, engineVersion)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()|0o644)
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

func ensureTemplateAlias(destRoot, version string) error {
	short := templateShortVersion(version)
	if short == version {
		return nil
	}
	src := filepath.Join(destRoot, version)
	dst := filepath.Join(destRoot, short)
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	return copyDir(src, dst)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func extractZipFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, rc)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// InstallEditorFromZip extracts a Linux editor zip and installs the binary at engineDest.
func InstallEditorFromZip(zipPath, engineDest string) error {
	tmpDir, err := os.MkdirTemp("", "blazium-editor-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := unzipArchive(zipPath, tmpDir); err != nil {
		return fmt.Errorf("unzip editor: %w", err)
	}
	bin, err := findEditorBinary(tmpDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(engineDest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(bin)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(engineDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func unzipArchive(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		name := filepath.FromSlash(f.Name)
		if strings.HasPrefix(name, "__MACOSX") {
			continue
		}
		target := filepath.Join(dest, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func findEditorBinary(root string) (string, error) {
	var candidates []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		base := strings.ToLower(filepath.Base(path))
		if strings.HasSuffix(base, ".dll") || strings.HasSuffix(base, ".pdb") || strings.HasSuffix(base, ".so") {
			return nil
		}
		if strings.Contains(base, "blazium") && !strings.Contains(base, ".") {
			candidates = append(candidates, path)
			return nil
		}
		if strings.HasPrefix(base, "blazium.") || strings.HasPrefix(base, "blaziumeditor") {
			if info.Mode()&0o111 != 0 || strings.Contains(base, "linux") || strings.Contains(base, "x86_64") {
				candidates = append(candidates, path)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no editor binary found in %s", root)
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if strings.Count(filepath.Base(c), ".") < strings.Count(filepath.Base(best), ".") {
			best = c
		}
	}
	return best, nil
}
