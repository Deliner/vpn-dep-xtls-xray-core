package github

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vpn-script/internal/quality/dependencies/records"
)

func TestSharedNetworkRetainsDistinctUpstreamIdentity(t *testing.T) {
	root := &Repository{ID: 1, FullName: "golang/glog"}
	upstream := Repository{ID: 2, FullName: "kubernetes/klog", Fork: true, Source: root}
	fork := Repository{ID: 3, FullName: "Deliner/vpn-dep-golang-glog", Fork: true, Source: root}
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "POST") {
			t.Fatal("shared-network verification mutated GitHub")
		}
		return json.Marshal(fork)
	}
	record, err := sharedFork(context.Background(), run, upstream, fork, "Deliner/vpn-dep-kubernetes-klog")
	if err != nil || !record.SharedNetwork || record.Created || record.Upstream.ID != 2 || record.Fork.ID != 3 {
		t.Fatalf("identity mapping=%+v error=%v", record, err)
	}
}

func TestSharedNetworkRejectsDifferentOwnerOrSource(t *testing.T) {
	upstream := Repository{ID: 1}
	requested := Repository{ID: 2, FullName: "Deliner/vpn-dep-o-r"}
	for _, fork := range []Repository{
		{ID: 2, FullName: requested.FullName, Fork: true, Source: &Repository{ID: 99}},
		{ID: 2, FullName: "other/vpn-dep-o-r", Fork: true, Source: &upstream},
		{ID: 3, FullName: requested.FullName, Fork: true, Source: &upstream},
		{ID: 2, FullName: requested.FullName, Source: &upstream},
	} {
		if err := verifySharedNetwork(upstream, fork, requested); err == nil {
			t.Fatalf("accepted unrelated source %+v", fork)
		}
	}
	run := func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("queried unmanaged repository")
		return nil, nil
	}
	for _, name := range []string{"Other/vpn-dep-o-r", "Deliner/user-project"} {
		requested.FullName = name
		if _, err := sharedFork(context.Background(), run, upstream, requested, "Deliner/vpn-dep-o-r"); err == nil {
			t.Fatalf("accepted unmanaged target %s", name)
		}
	}
}

func TestSharedNetworkRetryOnlyRevalidatesAndDoesNotRequestAnotherFork(t *testing.T) {
	t.Chdir(t.TempDir())
	root := &Repository{ID: 1, FullName: "o/root"}
	upstream := Repository{ID: 2, FullName: "o/child", Source: root}
	fork := Repository{ID: 3, FullName: "Deliner/vpn-dep-o-root", Fork: true, Source: root}
	if err := records.Save("vpn-dep-o-child.json", ForkRecord{Upstream: upstream, Fork: fork, SharedNetwork: true}); err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		switch command {
		case "api --hostname github.com user --jq .login":
			return []byte("Deliner"), nil
		case "api --hostname github.com repos/o/child":
			return json.Marshal(upstream)
		case "api --hostname github.com repos/Deliner/vpn-dep-o-root":
			return json.Marshal(fork)
		default:
			t.Fatalf("unexpected request while revalidating existing alias: %s", command)
			return nil, nil
		}
	}
	result, err := Fork(context.Background(), run, "o/child", "Deliner")
	if err != nil || !result.SharedNetwork || result.Fork.ID != fork.ID {
		t.Fatalf("revalidation failed: %+v %v", result, err)
	}
}
