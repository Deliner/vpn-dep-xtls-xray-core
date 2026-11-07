package sshutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func TestPrivateLabSSH(t *testing.T) {
	root := os.Getenv("NHOP_SSH_LAB_ROOT")
	if root == "" {
		t.Skip("use just lab-ssh for the owned VM acceptance")
	}
	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(root, filepath.Join(home, ".ssh-custom", "nhop-lab-")) || !strings.HasPrefix(filepath.Base(root), "controller-ssh-") {
		t.Fatal("SSH acceptance requires a private checkout-owned lab directory")
	}
	port, err := strconv.Atoi(os.Getenv("NHOP_SSH_LAB_PORT"))
	if err != nil || port < 1024 || port > 65535 {
		t.Fatal("invalid local lab SSH port")
	}
	for _, name := range []string{"plain", "with spaces", "literal %h", `quote "and" back\slash`} {
		t.Run(name, func(t *testing.T) {
			node := labSSHNode(t, root, name, port)
			output := labSSHCommand(t, node, "printf nhop-ssh-verified")
			if string(output) != "nhop-ssh-verified" {
				t.Fatalf("unexpected SSH result: %s", output)
			}
			labHandoff(t, node)
			labCopyRoundTrip(t, node, filepath.Dir(node.SSH.IdentityFile))
		})
	}
	t.Run("unknown-host", func(t *testing.T) { labUnknownHost(t, root, port) })
	t.Run("changed-host", func(t *testing.T) { labChangedHost(t, root, port) })
}

func labSSHNode(t *testing.T, root, name string, port int) model.DeployNode {
	t.Helper()
	directory := filepath.Join(root, name)
	if err := safeio.PrivateDir(directory); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"identity", "known_hosts"} {
		raw, err := safeio.Read(filepath.Join(root, file), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if err := safeio.Atomic(filepath.Join(directory, file), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	node := model.DeployNode{Address: "127.0.0.1", SSH: model.SSH{User: "nhop", Port: port,
		IdentityFile: filepath.Join(directory, "identity"), KnownHostsFile: filepath.Join(directory, "known_hosts")}}
	if err := validate.SSH(&node.SSH); err != nil {
		t.Fatal(err)
	}
	return node
}

func labSSHCommand(t *testing.T, node model.DeployNode, script string) []byte {
	t.Helper()
	args := append(Connect(node), script)
	result, err := process.Run(context.Background(), 10*time.Second, args...)
	if err != nil || !result.Success {
		t.Fatalf("SSH failed: %v; %s", err, result.Output)
	}
	return result.Output
}

func labCopyRoundTrip(t *testing.T, node model.DeployNode, directory string) {
	t.Helper()
	remote := strings.TrimSpace(string(labSSHCommand(t, node, "mktemp -d /tmp/nhop-ssh-XXXXXXXX")))
	if !regexp.MustCompile(`^/tmp/nhop-ssh-[A-Za-z0-9]{8}$`).MatchString(remote) {
		t.Fatal("unexpected private staging directory")
	}
	t.Cleanup(func() { labSSHCommand(t, node, Shell("rm", "-rf", "--", remote)) })
	payload := []byte("SSH and SFTP must use the same explicit identity and host trust\n")
	source, destination := filepath.Join(directory, "source payload"), filepath.Join(directory, "returned payload")
	if err := safeio.Atomic(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	remotePath := Destination(node, true) + ":" + remote + "/payload"
	for _, paths := range [][]string{{source, remotePath}, {remotePath, destination}} {
		result, err := process.Run(context.Background(), 10*time.Second, append(Arguments(node.SSH, "scp"), paths...)...)
		if err != nil || !result.Success {
			t.Fatalf("SCP failed: %v; %s", err, result.Output)
		}
	}
	returned, err := safeio.Read(destination, 1<<20)
	if err != nil || !bytes.Equal(returned, payload) {
		t.Fatal("SCP round trip changed payload", err)
	}
}

func labUnknownHost(t *testing.T, root string, port int) {
	node := labSSHNode(t, root, "unknown-host", port)
	if err := safeio.Atomic(node.SSH.KnownHostsFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	labRefuseHost(t, node, "Host key verification failed")
}

func labChangedHost(t *testing.T, root string, port int) {
	node := labSSHNode(t, root, "changed-host", port)
	key, err := process.Run(context.Background(), 5*time.Second, "ssh-keygen", "-y", "-f", node.SSH.IdentityFile)
	if err != nil || !key.Success {
		t.Fatal("could not read the disposable lab identity's public key", err)
	}
	wrongHost := fmt.Sprintf("[127.0.0.1]:%d %s\n", port, strings.TrimSpace(string(key.Output)))
	if err := safeio.Atomic(node.SSH.KnownHostsFile, []byte(wrongHost), 0600); err != nil {
		t.Fatal(err)
	}
	labRefuseHost(t, node, "REMOTE HOST IDENTIFICATION HAS CHANGED")
}

func labRefuseHost(t *testing.T, node model.DeployNode, diagnostic string) {
	t.Helper()
	before, err := safeio.Read(node.SSH.KnownHostsFile, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	marker := fmt.Sprintf("unexpected-%d", time.Now().UnixNano())
	result, err := process.Run(context.Background(), 10*time.Second, append(Connect(node), "printf "+marker)...)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 255 || result.Success || !strings.Contains(string(result.Output), diagnostic) || strings.Contains(string(result.Output), marker) {
		t.Fatalf("untrusted host did not fail before command execution: %v; %s", err, result.Output)
	}
	raw, err := safeio.Read(node.SSH.KnownHostsFile, 1<<20)
	if err != nil || !bytes.Equal(raw, before) {
		t.Fatal("untrusted host modified the configured trust store")
	}
}

func labHandoff(t *testing.T, node model.DeployNode) {
	t.Helper()
	command := Shell(append(Connect(node), "printf handoff-verified")...)
	result, err := process.Run(context.Background(), 10*time.Second, "sh", "-c", command)
	if err != nil || !result.Success || string(result.Output) != "handoff-verified" {
		t.Fatalf("shell handoff changed the SSH arguments: %v; %s", err, result.Output)
	}
}
