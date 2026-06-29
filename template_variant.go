package main

import (
	"os"
	"strings"
)

// TemplateVariant selects debug or release export template files.
type TemplateVariant string

const (
	TemplateVariantDebug   TemplateVariant = "debug"
	TemplateVariantRelease TemplateVariant = "release"
)

func TemplateVariantFromEnv() TemplateVariant {
	return TemplateVariantFromString(os.Getenv("BLAZIUM_TEMPLATE_VARIANT"))
}

func TemplateVariantFromString(v string) TemplateVariant {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "release":
		return TemplateVariantRelease
	default:
		return TemplateVariantDebug
	}
}

func (v TemplateVariant) String() string {
	if v == TemplateVariantRelease {
		return "release"
	}
	return "debug"
}

func webTemplateName(v TemplateVariant) string {
	return "web_nothreads_" + v.String() + ".zip"
}

func linuxTemplateName(v TemplateVariant) string {
	return "linux_" + v.String() + ".x86_64"
}

func windowsNormalTemplateName(v TemplateVariant) string {
	return "windows_" + v.String() + "_x86_64.exe"
}

func windowsConsoleTemplateName(v TemplateVariant) string {
	return "windows_" + v.String() + "_x86_64_console.exe"
}

func matchesTemplateVariant(name string, variant TemplateVariant) bool {
	lower := strings.ToLower(name)
	want := variant.String()
	if strings.Contains(lower, want) {
		return true
	}
	other := "release"
	if want == "release" {
		other = "debug"
	}
	return !strings.Contains(lower, other)
}

func isWindowsConsoleTemplate(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "windows") && strings.HasSuffix(lower, "_console.exe")
}

func isWindowsNormalTemplate(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "windows") && strings.HasSuffix(lower, ".exe") && !strings.Contains(lower, "_console")
}
