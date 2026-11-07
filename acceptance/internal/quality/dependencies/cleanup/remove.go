package cleanup

import (
	"context"
	"fmt"
	"time"

	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
	"vpn-script/internal/quality/dependencies/scope"
)

func remove(ctx context.Context, run commands.Runner, target Target, keep []scope.Fork) error {
	if protected(target, keep) {
		return fmt.Errorf("refusing to delete protected repository %s", target.Name)
	}
	expected := target
	expected.CreatedAt = ""
	saved, err := readTarget(target.Record)
	if err != nil || saved != expected {
		return fmt.Errorf("session evidence changed for %s: %v", target.Name, err)
	}
	repo, found, err := repository(ctx, run, target.Name)
	if err != nil {
		return err
	}
	if found {
		if err := verifyIdentity(repo, target, keep); err != nil {
			return err
		}
		if err := verifySource(repo, target); err != nil {
			return err
		}
		if _, err := api(ctx, run, "--method", "DELETE", "repos/"+target.Name); err != nil {
			return err
		}
	}
	_, remains, err := repository(ctx, run, target.Name)
	if err != nil || remains {
		return fmt.Errorf("repository deletion is not confirmed for %s: %v", target.Name, err)
	}
	result := struct {
		Target     Target `json:"target"`
		Deleted    bool   `json:"deleted_via_api"`
		VerifiedAt string `json:"absence_verified_at"`
	}{target, found, time.Now().UTC().Format(time.RFC3339)}
	return records.Save(fmt.Sprintf("cleanup-absent-%d.json", target.ID), result)
}
