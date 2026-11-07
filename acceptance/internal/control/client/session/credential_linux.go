package session

import (
	"os/exec"
	"syscall"
)

func credential(command *exec.Cmd, uid, gid int, unprivileged bool) {
	command.SysProcAttr.Pdeathsig = syscall.SIGKILL
	if unprivileged {
		command.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}
	}
}
