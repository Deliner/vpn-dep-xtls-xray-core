package github

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMirrorCannotAdoptUnrelatedRepository(t *testing.T) {
	for _, mutation := range []string{"description", "private", "fork", "owner"} {
		t.Run(mutation, func(t *testing.T) {
			repo := Repository{ID: 1, FullName: "Deliner/vpn-dep-source", Description: "Source mirror of https://example.org/source; selected N-hop revisions."}
			switch mutation {
			case "description":
				repo.Description = "unrelated"
			case "private":
				repo.Private = true
			case "fork":
				repo.Fork = true
			case "owner":
				repo.FullName = "different/vpn-dep-source"
			}
			run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
				command := strings.Join(args, " ")
				if strings.Contains(command, "--method") {
					t.Fatal("unrelated mirror was mutated")
				}
				if strings.Contains(command, "user --jq") {
					return []byte("Deliner"), nil
				}
				return json.Marshal(repo)
			}
			if _, err := EnsureMirror(context.Background(), run, "vpn-dep-source", "https://example.org/source"); err == nil {
				t.Fatal("accepted unrelated repository")
			}
		})
	}
}
