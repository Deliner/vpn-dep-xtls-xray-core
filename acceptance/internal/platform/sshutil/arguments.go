package sshutil

import (
	"strconv"
	"strings"

	"vpn-script/internal/core/model"
)

func Arguments(settings model.SSH, program string) []string {
	portFlag := "-p"
	if program == "scp" {
		portFlag = "-P"
	}
	return []string{program, portFlag, strconv.Itoa(settings.Port), "-F", "none",
		"-o", "IdentityFile=" + configPath(settings.IdentityFile),
		"-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes",
		"-o", "ConnectTimeout=10", "-o", "UserKnownHostsFile=" + configPath(settings.KnownHostsFile),
		"-o", "GlobalKnownHostsFile=/dev/null", "-o", "ForwardAgent=no",
		"-o", "ClearAllForwardings=yes", "-o", "ControlPath=none"}
}

// OpenSSH parses -o values and later expands percent tokens. Shell quoting alone
// cannot preserve a literal path at either of those boundaries.
func configPath(value string) string {
	return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "%", "%%").Replace(value) + "\""
}

func Destination(node model.DeployNode, scp bool) string {
	address := node.Address
	if scp && strings.Contains(address, ":") {
		address = "[" + address + "]"
	}
	return node.SSH.User + "@" + address
}

func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func Shell(arguments ...string) string {
	parts := make([]string, len(arguments))
	for index, value := range arguments {
		parts[index] = Quote(value)
	}
	return strings.Join(parts, " ")
}

func Connect(node model.DeployNode) []string {
	return append(Arguments(node.SSH, "ssh"), Destination(node, false))
}
