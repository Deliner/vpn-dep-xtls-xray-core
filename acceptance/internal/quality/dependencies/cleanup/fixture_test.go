package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"vpn-script/internal/quality/dependencies/github"
	"vpn-script/internal/quality/dependencies/records"
)

type fixture struct {
	repo      Repository
	target    Target
	failure   string
	deleted   bool
	deletions int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Chdir(t.TempDir())
	repo := Repository{ID: 20, Name: "Deliner/vpn-dep-fixture", Fork: true, CreatedAt: "2026-11-07T14:00:00Z"}
	repo.Owner.ID, repo.Owner.Login = accountID, "Deliner"
	repo.Parent = &struct{ ID int64 }{10}
	saved := github.ForkRecord{Fork: github.Repository{ID: repo.ID, FullName: repo.Name, Fork: true, Parent: &github.Repository{ID: 10}}}
	if err := records.Save("vpn-dep-fixture.json", saved); err != nil {
		t.Fatal(err)
	}
	target, err := readTarget("vpn-dep-fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	target.CreatedAt = repo.CreatedAt
	return &fixture{repo: repo, target: target}
}

func (f *fixture) run(_ context.Context, binary string, args ...string) ([]byte, error) {
	command := strings.Join(args, " ")
	if binary != "gh" {
		return nil, fmt.Errorf("unexpected cleanup executable")
	}
	if command == "api --hostname github.com repos/"+f.target.Name {
		if f.failure == "read" || f.failure == "postread" && f.deleted {
			return nil, errors.New("network failure")
		}
		if f.deleted {
			return []byte(`{"status":"404"}`), errors.New("not found")
		}
		return json.Marshal(f.repo)
	}
	if command == "api --hostname github.com --method DELETE repos/"+f.target.Name {
		f.deletions++
		if f.failure == "permission" {
			return nil, errors.New("delete_repo scope missing")
		}
		f.deleted = true
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected cleanup operation: %s", command)
}
