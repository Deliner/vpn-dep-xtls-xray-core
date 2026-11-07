package deploy

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/sshutil"
)

var stagePattern = regexp.MustCompile(`^/var/tmp/vpn-nhop-[A-Za-z0-9]{8}$`)

func applyNode(ctx context.Context, run process.Runner, directory, id string, manifest model.Deployment) (err error) {
	node := manifest.Nodes[id]
	path, arch, err := stage(ctx, run, node)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cleanup(run, node, path)) }()
	return transfer(ctx, run, node, directory, id, path, manifest.Installers[arch])
}

func remote(ctx context.Context, run process.Runner, node model.DeployNode, script string, timeout time.Duration) (string, error) {
	arguments := append(sshutil.Connect(node), script)
	result, err := run(ctx, timeout, arguments...)
	if err != nil || !result.Success {
		return "", fmt.Errorf("SSH command failed for %s", node.Address)
	}
	return strings.TrimSpace(string(result.Output)), nil
}

func stage(ctx context.Context, run process.Runner, node model.DeployNode) (string, string, error) {
	arch, err := remote(ctx, run, node, "uname -m", 30*time.Second)
	if err != nil || (arch != "x86_64" && arch != "aarch64") {
		return "", "", fmt.Errorf("remote architecture probe failed or architecture unsupported")
	}
	output, err := remote(ctx, run, node, "umask 077; mktemp -d /var/tmp/vpn-nhop-XXXXXXXX", 30*time.Second)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(output, "\n")
	path := lines[len(lines)-1]
	if !stagePattern.MatchString(path) {
		return "", "", fmt.Errorf("unexpected remote staging directory")
	}
	return path, arch, nil
}

func transfer(ctx context.Context, run process.Runner, node model.DeployNode, directory, id, destination string, binary model.Binary) error {
	args := append(sshutil.Arguments(node.SSH, "scp"), filepath.Join(directory, binary.File),
		filepath.Join(directory, "nodes", id+".json"), sshutil.Destination(node, true)+":"+destination+"/")
	result, err := run(ctx, 2*time.Minute, args...)
	if err != nil || !result.Success {
		return fmt.Errorf("SCP transfer failed")
	}
	command := []string{destination + "/" + filepath.Base(binary.File), "--config", destination + "/" + id + ".json"}
	if node.SSH.User != "root" {
		command = append([]string{"sudo", "-n", "--"}, command...)
	}
	_, err = remote(ctx, run, node, sshutil.Shell(command...), 10*time.Minute)
	return err
}

func cleanup(run process.Runner, node model.DeployNode, path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := remote(ctx, run, node, sshutil.Shell("rm", "-rf", "--", path), 30*time.Second)
	return err
}
