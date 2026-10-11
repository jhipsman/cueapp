//go:build unix

package api

import (
	"os"
	"syscall"
)

// pauseProcess stops or starts a running process (an Apple TV conversion
// that's far enough ahead of the viewer).
func pauseProcess(p *os.Process, stop bool) {
	if p == nil {
		return
	}
	if stop {
		_ = p.Signal(syscall.SIGSTOP)
	} else {
		_ = p.Signal(syscall.SIGCONT)
	}
}
