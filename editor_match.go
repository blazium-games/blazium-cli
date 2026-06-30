package main

import (
	"errors"
	"strings"
)

func findEditorMetadata(version, platform, arch string, isMono bool) (*EditorMetadata, error) {
	for i := range editorMetadata {
		if filenameMatchesEditor(editorMetadata[i].Filename, version, platform, arch, isMono) {
			return &editorMetadata[i], nil
		}
	}
	return nil, errors.New("no matching editor metadata found")
}

func filenameMatchesEditor(filename, version, platform, arch string, isMono bool) bool {
	name := strings.ToLower(filename)
	if !strings.Contains(name, strings.ToLower(version)) {
		return false
	}
	if !containsAny(name, platformTokens(platform)) {
		return false
	}
	if !containsAny(name, archTokens(arch)) {
		return false
	}
	isMonoFile := strings.Contains(name, ".mono")
	return isMono == isMonoFile
}

func platformTokens(platform string) []string {
	switch strings.ToLower(platform) {
	case "linux", "linuxbsd":
		return []string{"linux"}
	case "windows":
		return []string{"windows"}
	case "macos", "darwin":
		return []string{"macos", "darwin"}
	default:
		return []string{strings.ToLower(platform)}
	}
}

func archTokens(arch string) []string {
	switch strings.ToLower(arch) {
	case "x86_64", "amd64", "64bit":
		return []string{"x86_64", "64bit", "x64"}
	case "x86_32", "x86", "32bit", "i386":
		return []string{"x86_32", "32bit", "x86_32"}
	case "arm64", "aarch64":
		return []string{"arm64", "aarch64"}
	default:
		return []string{strings.ToLower(arch)}
	}
}

func containsAny(value string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}
