package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpn-script/internal/core/model"
)

func resolve(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) || filepath.Dir(path) == path {
		return "", err
	}
	parent, err := resolve(filepath.Dir(path))
	return filepath.Join(parent, filepath.Base(path)), err
}

func SSHPath(value string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(value, "~/") {
		value = filepath.Join(home, value[2:])
	}
	root, err := resolve(filepath.Join(home, ".ssh-custom"))
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	path, err = resolve(path)
	if err != nil || strings.ContainsAny(value, "\n\r") || !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return "", fmt.Errorf("SSH files must be under ~/.ssh-custom")
	}
	if strings.ContainsAny(path, "\x00\n\r") || strings.Contains(path, "${") {
		return "", fmt.Errorf("SSH paths must be literal: control characters and ${...} expansion are unsupported")
	}
	return path, nil
}

func SSH(ssh *model.SSH) error {
	if ssh.Port == 0 {
		ssh.Port = 22222
	}
	if err := Port(ssh.Port); err != nil {
		return err
	}
	if err := User(ssh.User); err != nil {
		return err
	}
	if err := closedPorts(ssh); err != nil {
		return err
	}
	var err error
	ssh.IdentityFile, err = SSHPath(ssh.IdentityFile)
	if err != nil {
		return err
	}
	ssh.KnownHostsFile, err = SSHPath(ssh.KnownHostsFile)
	return err
}

func closedPorts(ssh *model.SSH) error {
	if ssh.ClosedPorts == nil {
		ssh.ClosedPorts = []int{}
		if ssh.Port != 22 {
			ssh.ClosedPorts = append(ssh.ClosedPorts, 22)
		}
	}
	seen := map[int]bool{ssh.Port: true}
	for _, number := range ssh.ClosedPorts {
		if err := Port(number); err != nil {
			return err
		}
		if seen[number] {
			return fmt.Errorf("closed_ports: duplicate or active SSH port")
		}
		seen[number] = true
	}
	return nil
}
