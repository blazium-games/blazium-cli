package exporttemplates

import (
	"fmt"
	"os"
	"strings"
)

// FilterOpts selects a subset of catalog entries.
type FilterOpts struct {
	Files    []string
	Platform string
	MonoOnly bool
	SkipMono bool
}

// Filter returns entries matching opts.
func Filter(all []Entry, opts FilterOpts) []Entry {
	var out []Entry
	wantFiles := make(map[string]bool)
	for _, f := range opts.Files {
		wantFiles[strings.ToLower(strings.TrimSpace(f))] = true
	}
	platform := strings.ToLower(strings.TrimSpace(opts.Platform))

	for _, m := range all {
		if opts.MonoOnly && !m.Mono {
			continue
		}
		if opts.SkipMono && m.Mono {
			continue
		}
		if platform != "" && strings.ToLower(m.Platform) != platform {
			continue
		}
		if len(wantFiles) > 0 && !wantFiles[strings.ToLower(m.Filename)] {
			continue
		}
		out = append(out, m)
	}
	return out
}

// InstallVersion picks a version label from entries or fallback.
func InstallVersion(entries []Entry, fallback string) string {
	for _, e := range entries {
		if v := strings.TrimSpace(e.Version); v != "" {
			return v
		}
	}
	return fallback
}

// SelectRuntime picks web, linux, and windows templates for the active variant.
// Windows requires both normal and console executables.
func SelectRuntime(all []Entry, variant Variant) []Entry {
	var out []Entry

	if m := pickByFilename(all, webTemplateName(variant)); m != nil {
		out = append(out, *m)
	} else if m := pickPlatformFallback(all, "web", variant); m != nil {
		logf("Warning: %s web template not found; using %s", webTemplateName(variant), m.Filename)
		out = append(out, *m)
	}

	if m := pickByFilename(all, linuxTemplateName(variant)); m != nil {
		out = append(out, *m)
	} else if m := pickPlatformFallback(all, "linux", variant); m != nil {
		logf("Warning: %s linux template not found; using %s", linuxTemplateName(variant), m.Filename)
		out = append(out, *m)
	}

	normal := windowsNormalTemplateName(variant)
	console := windowsConsoleTemplateName(variant)
	if m := pickByFilename(all, normal); m != nil {
		out = append(out, *m)
	} else if m := pickWindowsNormalFallback(all, variant); m != nil {
		logf("Warning: %s not found; using %s", normal, m.Filename)
		out = append(out, *m)
	}
	if m := pickByFilename(all, console); m != nil {
		out = append(out, *m)
	} else if m := pickWindowsConsoleFallback(all, variant); m != nil {
		logf("Warning: %s not found; using %s", console, m.Filename)
		out = append(out, *m)
	}

	return out
}

func pickByFilename(all []Entry, filename string) *Entry {
	want := strings.ToLower(filename)
	for i := range all {
		if strings.ToLower(all[i].Filename) == want {
			return &all[i]
		}
	}
	return nil
}

func pickPlatformFallback(all []Entry, platform string, variant Variant) *Entry {
	for i := range all {
		p := strings.ToLower(all[i].Platform)
		if p != platform {
			continue
		}
		if matchesTemplateVariant(all[i].Filename, variant) {
			return &all[i]
		}
	}
	for i := range all {
		if strings.ToLower(all[i].Platform) == platform {
			return &all[i]
		}
	}
	return nil
}

func pickWindowsNormalFallback(all []Entry, variant Variant) *Entry {
	for i := range all {
		if strings.ToLower(all[i].Platform) != "windows" {
			continue
		}
		if !isWindowsNormalTemplate(all[i].Filename) {
			continue
		}
		if matchesTemplateVariant(all[i].Filename, variant) {
			return &all[i]
		}
	}
	for i := range all {
		if strings.ToLower(all[i].Platform) == "windows" && isWindowsNormalTemplate(all[i].Filename) {
			return &all[i]
		}
	}
	return nil
}

func pickWindowsConsoleFallback(all []Entry, variant Variant) *Entry {
	for i := range all {
		if strings.ToLower(all[i].Platform) != "windows" {
			continue
		}
		if !isWindowsConsoleTemplate(all[i].Filename) {
			continue
		}
		if matchesTemplateVariant(all[i].Filename, variant) {
			return &all[i]
		}
	}
	for i := range all {
		if strings.ToLower(all[i].Platform) == "windows" && isWindowsConsoleTemplate(all[i].Filename) {
			return &all[i]
		}
	}
	return nil
}

// Logf is overridable for tests; defaults to stderr.
var Logf = func(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func logf(format string, args ...interface{}) {
	Logf(format, args...)
}
