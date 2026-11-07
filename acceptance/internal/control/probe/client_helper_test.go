package probe

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type probeChild struct {
	PID     int
	Address string
}

func TestMain(m *testing.M) {
	if len(os.Args) >= 4 && os.Args[1] == "nhop-probe-helper" {
		if os.Args[2] == "parent" {
			probeParent(os.Args[3])
		} else {
			probeListener(os.Args[3], os.Args[len(os.Args)-1], os.Args[2] == "listener")
		}
		return
	}
	os.Exit(m.Run())
}

func probeListener(directory, configPath string, configured bool) {
	port := 0
	if configured {
		raw, err := os.ReadFile(configPath)
		var config struct{ Inbounds []struct{ Port int } }
		if err != nil || json.Unmarshal(raw, &config) != nil || len(config.Inbounds) != 1 {
			os.Exit(3)
		}
		port = config.Inbounds[0].Port
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		os.Exit(4)
	}
	raw, _ := json.Marshal(probeChild{PID: os.Getpid(), Address: listener.Addr().String()})
	if os.WriteFile(filepath.Join(directory, "ready.json"), raw, 0600) != nil {
		os.Exit(5)
	}
	if configured {
		go probeExitSignal(directory)
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		connection.Close()
	}
}

func probeExitSignal(directory string) {
	for {
		if _, err := os.Stat(filepath.Join(directory, "exit")); err == nil {
			os.Exit(23)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func probeParent(directory string) {
	command := exec.Command(os.Args[0], "nhop-probe-helper", "background-child", directory)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if command.Start() != nil {
		os.Exit(6)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "exit")); err == nil {
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
