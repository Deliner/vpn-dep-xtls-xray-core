package cleanup

import (
	"fmt"
	"regexp"
	"time"

	"vpn-script/internal/quality/dependencies/scope"
)

var managedName = regexp.MustCompile(`^Deliner/vpn-dep-[a-z0-9._-]+$`)

func protected(target Target, keep []scope.Fork) bool {
	for _, fork := range keep {
		if fork.ID == target.ID || fork.Name == target.Name {
			return true
		}
	}
	return false
}

func verifyIdentity(repo Repository, target Target, keep []scope.Fork) error {
	if protected(target, keep) || target.ID <= 0 || !managedName.MatchString(target.Name) {
		return fmt.Errorf("refusing protected or unmanaged repository %s", target.Name)
	}
	if repo.ID != target.ID || repo.Name != target.Name || repo.Owner.Login != "Deliner" || repo.Owner.ID != accountID || repo.Private || repo.Fork != target.Fork {
		return fmt.Errorf("repository identity changed for %s", target.Name)
	}
	created, err := time.Parse(time.RFC3339, repo.CreatedAt)
	start, _ := time.Parse(time.RFC3339, sessionStart)
	stop, _ := time.Parse(time.RFC3339, sessionStop)
	if err != nil || created.Before(start) || created.After(stop) || target.CreatedAt != "" && target.CreatedAt != repo.CreatedAt {
		return fmt.Errorf("repository was not created during the interrupted session: %s", target.Name)
	}
	return nil
}

func verifySource(repo Repository, target Target) error {
	if target.Fork {
		if repo.Parent == nil || repo.Parent.ID != target.ParentID || target.ParentID <= 0 {
			return fmt.Errorf("fork parent changed for %s", target.Name)
		}
		if target.SourceID > 0 && (repo.Source == nil || repo.Source.ID != target.SourceID) {
			return fmt.Errorf("fork network changed for %s", target.Name)
		}
	} else if target.Description == "" || repo.Description != target.Description {
		return fmt.Errorf("source mirror description changed for %s", target.Name)
	}
	return nil
}
