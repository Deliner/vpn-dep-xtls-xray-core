package sshutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
)

func TestControllerSSHDoesNotInheritForeignConfiguration(t *testing.T) {
	directory := t.TempDir()
	config := filepath.Join(directory, "foreign.conf")
	contents := "Host *\n Hostname changed.example.invalid\n ForwardAgent yes\n ControlMaster auto\n ControlPath /tmp/foreign-control\n GlobalKnownHostsFile /tmp/foreign-hosts\n IdentityFile /tmp/foreign-identity\n ProxyCommand false\n"
	if err := os.WriteFile(config, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	control := sshConfiguration(t, []string{"ssh", "-G", "-F", config, "requested.example.invalid"})
	if !strings.Contains(control, "\nhostname changed.example.invalid\n") || !strings.Contains(control, "\nforwardagent yes\n") {
		t.Fatal("foreign-config positive control did not apply its settings")
	}
	settings := model.SSH{Port: 22022, IdentityFile: filepath.Join(directory, "identity"), KnownHostsFile: filepath.Join(directory, "known_hosts")}
	if err := os.WriteFile(settings.IdentityFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	args := append([]string{"ssh", "-G", "-F", config}, Arguments(settings, "ssh")[1:]...)
	observed := sshConfiguration(t, append(args, "requested.example.invalid"))
	for key, value := range map[string]string{"hostname": "requested.example.invalid", "port": "22022", "batchmode": "yes", "forwardagent": "no", "globalknownhostsfile": "/dev/null", "clearallforwardings": "yes"} {
		if !strings.Contains(observed, "\n"+key+" "+value+"\n") {
			t.Errorf("unexpected effective %s; configuration:\n%s", key, observed)
		}
	}
	if strings.Contains(observed, "foreign-") || strings.Contains(observed, "\nproxycommand ") || strings.Count(observed, "\nidentityfile ") != 1 {
		t.Error("foreign identity, command or connection state reached the controller configuration")
	}
}

func sshConfiguration(t *testing.T, args []string) string {
	t.Helper()
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("OpenSSH client is required for its parser acceptance")
	}
	result, err := process.Run(t.Context(), 5*time.Second, args...)
	if err != nil || !result.Success {
		t.Fatalf("OpenSSH parser failed: %v; %s", err, result.Output)
	}
	return "\n" + string(result.Output)
}
