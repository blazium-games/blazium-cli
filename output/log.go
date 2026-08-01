package output

import (
	"fmt"
	"os"
)

// Quiet suppresses non-fatal warnings when true.
var Quiet bool

// Warnf prints a warning to stderr unless Quiet is set.
func Warnf(format string, args ...any) {
	if Quiet {
		return
	}
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

// Notef prints a non-fatal notice to stderr unless Quiet is set.
func Notef(format string, args ...any) {
	if Quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
