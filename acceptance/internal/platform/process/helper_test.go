//go:build darwin || linux

package process_test

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type background struct {
	PID     int
	Address string
}

func TestMain(m *testing.M) {
	if len(os.Args) >= 4 && os.Args[1] == "nhop-process-helper" {
		if os.Args[2] == "listener" {
			serveBackground(os.Args[3])
		} else {
			parentHelper(os.Args[2], os.Args[3])
		}
		return
	}
	os.Exit(m.Run())
}

func serveBackground(path string) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	raw, _ := json.Marshal(background{PID: os.Getpid(), Address: listener.Addr().String()})
	if os.WriteFile(path, raw, 0600) != nil {
		os.Exit(3)
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		connection.Close()
	}
}

func parentHelper(mode, directory string) {
	command := exec.Command(os.Args[0], "nhop-process-helper", "listener", filepath.Join(directory, "ready.json"))
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if command.Start() != nil {
		os.Exit(4)
	}
	for {
		if mode == "parent-exits" {
			if _, err := os.Stat(filepath.Join(directory, "finish")); err == nil {
				os.Exit(0)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}
