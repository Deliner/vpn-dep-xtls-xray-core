package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type forkFixture struct {
	posts int
}

func (f *forkFixture) run(_ context.Context, binary string, args ...string) ([]byte, error) {
	command := strings.Join(args, " ")
	if binary != "gh" {
		return nil, fmt.Errorf("unexpected executable %s", binary)
	}
	if strings.Contains(command, "--method POST") {
		if command != "api --hostname github.com --method POST repos/o/r/forks -f name=vpn-dep-o-r -F default_branch_only=false" {
			return nil, fmt.Errorf("unexpected mutation: %s", command)
		}
		f.posts++
		return []byte(`{"id":20,"full_name":"Deliner/vpn-dep-o-r"}`), nil
	}
	if command != "api --hostname github.com repos/Deliner/vpn-dep-o-r" {
		return nil, fmt.Errorf("unexpected lookup: %s", command)
	}
	if f.posts == 0 {
		return []byte(`{"status":"404"}`), errors.New("HTTP 404")
	}
	return []byte(`{"id":20,"full_name":"Deliner/vpn-dep-o-r","fork":true,"parent":{"id":10}}`), nil
}
