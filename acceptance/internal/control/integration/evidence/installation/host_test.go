package installation

import (
	"maps"
	"strings"
	"testing"
)

func TestNativeHostIdentityRejectsForeignOrIncompleteExecutionContext(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		identity := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "Deliner/vpn-dep-xtls-xray-core",
			"GITHUB_REPOSITORY_ID": "1449369505", "GITHUB_REPOSITORY_OWNER_ID": "26311467", "RUNNER_ENVIRONMENT": "github-hosted",
			"GITHUB_EVENT_NAME": "push", "GITHUB_REF": "refs/heads/nhop-acceptance", "NHOP_NODE_CI": "linux/" + arch,
			"GITHUB_SHA": strings.Repeat("a", 40), "GITHUB_RUN_ID": "1234"}
		if err := nativeIdentity(identity, arch); err != nil {
			t.Fatal(err)
		}
		for _, key := range ciIdentityKeys {
			changed := maps.Clone(identity)
			changed[key] = "foreign"
			if err := nativeIdentity(changed, arch); err == nil {
				t.Fatal("foreign CI identity accepted", key)
			}
			delete(changed, key)
			if err := nativeIdentity(changed, arch); err == nil {
				t.Fatal("missing CI identity accepted", key)
			}
		}
		if err := nativeIdentity(identity, "386"); err == nil {
			t.Fatal("non-native supported CPU gate bypassed")
		}
	}
}
