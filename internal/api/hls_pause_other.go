//go:build !unix

package api

import "os"

// pauseProcess does nothing where processes can't be paused: the
// conversion simply runs to the end.
func pauseProcess(*os.Process, bool) {}
