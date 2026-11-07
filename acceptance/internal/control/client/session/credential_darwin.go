package session

import "os/exec"

// Start refuses macOS before preparing files or launching children.
func credential(command *exec.Cmd, uid, gid int, unprivileged bool) {}
