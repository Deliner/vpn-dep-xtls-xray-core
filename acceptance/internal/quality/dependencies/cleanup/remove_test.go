package cleanup

import (
	"context"
	"testing"

	"vpn-script/internal/quality/dependencies/scope"
)

func TestRemovalRequiresMatchingSessionIdentityAndPreservesProtectedIDs(t *testing.T) {
	for _, change := range []string{"id", "owner", "date", "private", "parent", "record", "protected-id", "protected-name", "read"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			keep := []scope.Fork{}
			switch change {
			case "id":
				f.repo.ID++
			case "owner":
				f.repo.Owner.ID++
			case "date":
				f.repo.CreatedAt = "2026-11-06T14:00:00Z"
			case "private":
				f.repo.Private = true
			case "parent":
				f.repo.Parent.ID++
			case "record":
				f.target.RecordSHA = "changed"
			case "protected-id":
				keep = []scope.Fork{{ID: f.target.ID, Name: "Deliner/vpn-dep-renamed"}}
			case "protected-name":
				keep = []scope.Fork{{ID: 999, Name: f.target.Name}}
			case "read":
				f.failure = "read"
			}
			if err := remove(context.Background(), f.run, f.target, keep); err == nil || f.deletions != 0 {
				t.Fatalf("unsafe deletion after %s: %v", change, err)
			}
		})
	}
}

func TestRemovalConfirmsAbsenceAndResumesWithoutDuplicateDelete(t *testing.T) {
	f := newFixture(t)
	f.failure = "postread"
	if err := remove(context.Background(), f.run, f.target, nil); err == nil || f.deletions != 1 {
		t.Fatalf("unconfirmed deletion was reported complete: %v", err)
	}
	f.failure = ""
	if err := remove(context.Background(), f.run, f.target, nil); err != nil || f.deletions != 1 {
		t.Fatalf("resume repeated deletion or lost absence: %v", err)
	}
}

func TestRemovalPermissionFailureDoesNotBecomeSuccess(t *testing.T) {
	f := newFixture(t)
	f.failure = "permission"
	if err := remove(context.Background(), f.run, f.target, nil); err == nil || f.deleted {
		t.Fatalf("permission failure was masked: %v", err)
	}
}
