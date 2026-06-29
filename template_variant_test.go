package main

import "testing"

func TestTemplateVariantFromString(t *testing.T) {
	if TemplateVariantFromString("") != TemplateVariantDebug {
		t.Fatal("empty should default to debug")
	}
	if TemplateVariantFromString("release") != TemplateVariantRelease {
		t.Fatal("release")
	}
	if TemplateVariantFromString("debug") != TemplateVariantDebug {
		t.Fatal("debug")
	}
}

func TestTemplateVariantNames(t *testing.T) {
	if webTemplateName(TemplateVariantDebug) != "web_nothreads_debug.zip" {
		t.Fatal("web debug name")
	}
	if linuxTemplateName(TemplateVariantRelease) != "linux_release.x86_64" {
		t.Fatal("linux release name")
	}
	if windowsNormalTemplateName(TemplateVariantDebug) != "windows_debug_x86_64.exe" {
		t.Fatal("windows normal")
	}
	if windowsConsoleTemplateName(TemplateVariantDebug) != "windows_debug_x86_64_console.exe" {
		t.Fatal("windows console")
	}
}
