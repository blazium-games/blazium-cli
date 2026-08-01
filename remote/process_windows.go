//go:build windows

package remote

import (
	"os"
)

func processAlive(pid int) bool {
	// On Windows, FindProcess always succeeds for non-zero PIDs; OpenProcess via
	// os.FindProcess + Signal isn't reliable. Use a best-effort check by attempting
	// to open the process with PROCESS_QUERY_LIMITED_INFORMATION through os.StartProcess
	// is not available — fall back to checking whether we can signal via tasklist-like
	// approach: duplicate handle isn't exposed. Use kernel32 via syscall.
	return windowsProcessAlive(uint32(pid))
}

// keep os import for symmetry / future use
var _ = os.ErrNotExist
