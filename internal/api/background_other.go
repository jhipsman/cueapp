//go:build !unix

package api

import "os"

// backgroundPriority does nothing where priorities can't be set.
func backgroundPriority(*os.Process) {}
