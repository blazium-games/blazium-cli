package exporttemplates

import "testing"

func TestVariantFromString(t *testing.T) {
	if VariantFromString("") != VariantRelease {
		t.Fatal("empty should default to release")
	}
	if VariantFromString("release") != VariantRelease {
		t.Fatal("release")
	}
	if VariantFromString("debug") != VariantDebug {
		t.Fatal("debug")
	}
}

func TestVariantNames(t *testing.T) {
	if webTemplateName(VariantDebug) != "web_nothreads_debug.zip" {
		t.Fatal("web debug name")
	}
	if linuxTemplateName(VariantRelease) != "linux_release.x86_64" {
		t.Fatal("linux release name")
	}
	if windowsNormalTemplateName(VariantDebug) != "windows_debug_x86_64.exe" {
		t.Fatal("windows normal")
	}
	if windowsConsoleTemplateName(VariantDebug) != "windows_debug_x86_64_console.exe" {
		t.Fatal("windows console")
	}
}
