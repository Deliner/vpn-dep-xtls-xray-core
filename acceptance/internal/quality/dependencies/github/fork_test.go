package github

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestForkNameRejectsUntrustedPaths(t *testing.T) {
	for _, input := range []string{"owner/repo/extra", "../repo", "owner/../repo", "https://github.com/o/r", "o/r?x=y", "o/r\n", "Deliner/repo"} {
		if _, err := forkName(input, "Deliner"); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	name, err := forkName("XTLS/Xray-core", "Deliner")
	if err != nil || name != "vpn-dep-xtls-xray-core" {
		t.Fatalf("name=%q error=%v", name, err)
	}
}

func TestForkRefusesUnrelatedExistingRepository(t *testing.T) {
	upstream := Repository{ID: 10, FullName: "upstream/engine"}
	for _, fork := range []Repository{
		{ID: 20, FullName: "Deliner/engine", Parent: &upstream},
		{ID: 20, FullName: "Deliner/engine", Fork: true},
		{ID: 20, FullName: "Deliner/engine", Fork: true, Parent: &Repository{ID: 99}},
		{ID: 20, FullName: "other/engine", Fork: true, Parent: &upstream},
	} {
		if err := verifyFork(upstream, fork, "Deliner/engine"); err == nil {
			t.Fatalf("accepted unrelated fork %+v", fork)
		}
	}
}

func TestRepositoryErrorsNeverAuthorizeCreation(t *testing.T) {
	for _, response := range []string{`{"status":"403"}`, `{"status":"500"}`, `not JSON`} {
		calls := 0
		run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls++
			if strings.Contains(strings.Join(args, " "), "POST") {
				t.Fatal("created fork after read failure")
			}
			return []byte(response), errors.New("request failed")
		}
		_, err := ensureFork(context.Background(), run, Repository{ID: 1, FullName: "o/r"}, "Deliner/r")
		if err == nil || calls != 1 {
			t.Fatalf("response=%q calls=%d error=%v", response, calls, err)
		}
	}
}

func TestCreateForkVerifiesReturnedParentAndIsIdempotent(t *testing.T) {
	fixture := &forkFixture{}
	upstream := Repository{ID: 10, FullName: "o/r"}
	for attempt := 0; attempt < 2; attempt++ {
		record, err := ensureFork(context.Background(), fixture.run, upstream, "Deliner/vpn-dep-o-r")
		if err != nil || record.Created != (attempt == 0) {
			t.Fatalf("attempt %d: %+v, %v", attempt, record, err)
		}
	}
	if fixture.posts != 1 {
		t.Fatalf("POST calls=%d", fixture.posts)
	}
}

func TestForkRequiresSelectedPersonalAccount(t *testing.T) {
	calls := 0
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls++
		if strings.Join(args, " ") != "api --hostname github.com user --jq .login" {
			t.Fatal(args)
		}
		return []byte("different-account\n"), nil
	}
	if _, err := Fork(context.Background(), run, "XTLS/Xray-core", "Deliner"); err == nil || calls != 1 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}

func TestRepositoryAcceptsCanonicalUpstreamRename(t *testing.T) {
	run := func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"id":333248791,"full_name":"HyNetworks/quic-go"}`), nil
	}
	repo, found, err := repository(context.Background(), run, "apernet/quic-go")
	if err != nil || !found || repo.FullName != "HyNetworks/quic-go" || repo.ID != 333248791 {
		t.Fatalf("canonical upstream identity lost: %+v, %v", repo, err)
	}
	if err := verifyFork(repo, Repository{Fork: true, FullName: "Other/repo", Parent: &repo}, "Deliner/repo"); err == nil {
		t.Fatal("rename allowance escaped destination ownership check")
	}
}
