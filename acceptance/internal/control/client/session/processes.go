package session

import (
	"os/exec"
	"sync"
)

type childExit struct {
	name string
	err  error
}

type children struct {
	commands []*exec.Cmd
	exits    chan childExit
	waiters  sync.WaitGroup
	once     sync.Once
}
