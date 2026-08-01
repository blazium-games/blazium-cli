package exporttemplates

import (
	"os"
	"strings"
)

// Variant selects debug or release export template files.
type Variant string

const (
	VariantDebug   Variant = "debug"
	VariantRelease Variant = "release"
)

// VariantFromEnv reads BLAZIUM_TEMPLATE_VARIANT (default release).
func VariantFromEnv() Variant {
	return VariantFromString(os.Getenv("BLAZIUM_TEMPLATE_VARIANT"))
}

// VariantFromString parses a variant string; empty defaults to release.
func VariantFromString(v string) Variant {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return VariantDebug
	default:
		return VariantRelease
	}
}

func (v Variant) String() string {
	if v == VariantDebug {
		return "debug"
	}
	return "release"
}

func webTemplateName(v Variant) string {
	return "web_nothreads_" + v.String() + ".zip"
}

func linuxTemplateName(v Variant) string {
	return "linux_" + v.String() + ".x86_64"
}

func windowsNormalTemplateName(v Variant) string {
	return "windows_" + v.String() + "_x86_64.exe"
}

func windowsConsoleTemplateName(v Variant) string {
	return "windows_" + v.String() + "_x86_64_console.exe"
}

func matchesTemplateVariant(name string, variant Variant) bool {
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
