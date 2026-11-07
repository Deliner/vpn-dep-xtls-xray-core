package mirroring

import (
	"context"
	"strings"
	"testing"

	"vpn-script/internal/quality/dependencies/records"
)

func TestPublishPreservesExactAncestryAndDisablesActionsFirst(t *testing.T) {
	f := fixture(t)
	got, err := Publish(context.Background(), f.run, f.source.Name)
	if err != nil || !got.Published || !f.pushed || !f.actionsDisabled || got.Ancestors != 12 {
		t.Fatalf("incomplete publication: %+v, %v", got, err)
	}
	var saved Snapshot
	if _, err := records.Read("mirror-"+f.source.Name+".json", &saved); err != nil || !saved.Published || saved.Repository.ID != 42 {
		t.Fatalf("lost publication evidence: %+v, %v", saved, err)
	}
}

func TestMirrorRefusesShallowChangedSourcesAndMutableRemoteRefs(t *testing.T) {
	for _, failure := range []string{"shallow", "tree", "actions", "remote"} {
		t.Run(failure, func(t *testing.T) {
			f := fixture(t)
			switch failure {
			case "shallow":
				f.shallow = true
			case "tree":
				changePrepared(t, f.source)
			case "actions":
				f.denyActions = true
			case "remote":
				f.remote = strings.Repeat("c", 40) + "\t" + sourceRef(f.source)
			}
			if _, err := Publish(context.Background(), f.run, f.source.Name); err == nil || f.pushed {
				t.Fatalf("unsafe mirror publication after %s: %v", failure, err)
			}
		})
	}
}
