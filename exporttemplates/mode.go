package exporttemplates

import "fmt"

// DownloadMode identifies how templates download selects artifacts.
type DownloadMode string

const (
	ModeFile     DownloadMode = "file"
	ModePlatform DownloadMode = "platform"
	ModeRuntime  DownloadMode = "runtime"
	ModeAll      DownloadMode = "all"
	ModeTPZ      DownloadMode = "tpz"
)

// ResolveDownloadMode ensures exactly one download mode is selected.
func ResolveDownloadMode(files []string, platform string, runtime, all, tpz bool) (DownloadMode, error) {
	var modes []DownloadMode
	if len(files) > 0 {
		modes = append(modes, ModeFile)
	}
	if platform != "" {
		modes = append(modes, ModePlatform)
	}
	if runtime {
		modes = append(modes, ModeRuntime)
	}
	if all {
		modes = append(modes, ModeAll)
	}
	if tpz {
		modes = append(modes, ModeTPZ)
	}
	if len(modes) == 0 {
		return "", fmt.Errorf("select exactly one mode: --file, --platform, --runtime, --all, or --tpz")
	}
	if len(modes) > 1 {
		return "", fmt.Errorf("modes are mutually exclusive; got %v", modes)
	}
	return modes[0], nil
}
