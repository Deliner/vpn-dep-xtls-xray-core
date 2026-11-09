package installation

import (
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strconv"
)

// OwnedHost gates destructive acceptance; it never authorizes an ordinary host.
func OwnedHost() error {
	host, err := os.Hostname()
	if err != nil || runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return fmt.Errorf("node acceptance requires root on an owned Linux test host")
	}
	if os.Getenv("NHOP_SYSTEMD_ACCEPTANCE") == "private-lima" && host == "lima-payload" {
		return nil
	}
	if os.Getenv("NHOP_SYSTEMD_ACCEPTANCE") != "owned-native-ci" {
		return fmt.Errorf("node acceptance requires explicit owned-VM or native CI opt-in")
	}
	identity := map[string]string{}
	for _, key := range ciIdentityKeys {
		identity[key] = os.Getenv(key)
	}
	return nativeIdentity(identity, runtime.GOARCH)
}

var ciIdentityKeys = []string{"GITHUB_ACTIONS", "GITHUB_REPOSITORY", "GITHUB_REPOSITORY_ID", "GITHUB_REPOSITORY_OWNER_ID",
	"RUNNER_ENVIRONMENT", "GITHUB_EVENT_NAME", "GITHUB_REF", "NHOP_NODE_CI", "GITHUB_SHA", "GITHUB_RUN_ID"}

func nativeIdentity(identity map[string]string, arch string) error {
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported native node acceptance CPU")
	}
	expected := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "Deliner/vpn-dep-xtls-xray-core",
		"GITHUB_REPOSITORY_ID": "1449369505", "GITHUB_REPOSITORY_OWNER_ID": "26311467", "RUNNER_ENVIRONMENT": "github-hosted",
		"GITHUB_EVENT_NAME": "push", "GITHUB_REF": "refs/heads/nhop-acceptance", "NHOP_NODE_CI": "linux/" + arch}
	for key, value := range expected {
		if identity[key] != value {
			return fmt.Errorf("untrusted native node acceptance identity: %s", key)
		}
	}
	commit, err := hex.DecodeString(identity["GITHUB_SHA"])
	if err != nil || len(commit) != 20 {
		return fmt.Errorf("native acceptance requires an exact workflow commit")
	}
	run, err := strconv.ParseUint(identity["GITHUB_RUN_ID"], 10, 64)
	if err != nil || run == 0 {
		return fmt.Errorf("native acceptance requires a run ID")
	}
	return nil
}
