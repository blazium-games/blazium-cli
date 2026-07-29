package main

import (
	"fmt"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/editorinstall"
)

// Shims so templates.go / tests keep compiling after Hub CLI refactor.

func channelName(isNightly bool) string { return cdn.ChannelName(isNightly) }

func fetchRemoteJSON(url string) ([]byte, error) { return httpGet(url) }

func downloadFile(url, dest string) error { return cdn.DownloadFile(url, dest) }

func defaultTemplatesDest() string { return editorinstall.DefaultTemplatesDest() }

func templateShortVersion(version string) string {
	return editorinstall.TemplateShortVersion(version)
}

func InstallTemplatesFromTPZ(tpzPath, destRoot string) (string, error) {
	return editorinstall.InstallTemplatesFromTPZ(tpzPath, destRoot)
}

func InstallTemplateFiles(destRoot, engineVersion string, files []string) error {
	return editorinstall.InstallTemplateFiles(destRoot, engineVersion, files)
}

// httpGet is overridable in tests (templates_test.go).
var httpGet = cdn.HTTPGet

func logMessage(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func ResolveLatestNightly() (string, error) { return cdn.ResolveLatestNightly() }

func compareSemver(a, b string) int { return cdn.CompareSemver(a, b) }
