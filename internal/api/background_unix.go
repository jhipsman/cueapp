//go:build unix

package api

import (
	"os"
	"syscall"
)

// backgroundPriority makes a running process the last in line for the
// processor (nice 19), so work nobody is waiting for (a preview clip)
// never slows what someone is watching.
func backgroundPriority(p *os.Process) {
	_ = syscall.Setpriority(syscall.PRIO_PROCESS, p.Pid, 19)
}
