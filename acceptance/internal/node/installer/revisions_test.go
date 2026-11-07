package installer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/release"
)

func TestLegacyStateUpgradesWithoutChangingConfigurationGeneration(t *testing.T) {
	i, _, data, old := legacyProgram(t)
	before, err := release.Inventory(old, data)
	require(t, err)
	state, err := i.state(data)
	require(t, err)
	if state.releaseID() != data.Generation {
		t.Fatal("legacy directory identity was lost")
	}
	prepared, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup())
	require(t, err)
	if !prepared.created || prepared.path == old {
		t.Fatal("new supervisor overwrote the previous release")
	}
	require(t, i.activatePrepared(data, prepared))
	state, err = i.state(data)
	require(t, err)
	if state.Generation != data.Generation || state.ReleaseID != filepath.Base(prepared.path) || state.PreviousGeneration != data.Generation || state.PreviousReleaseID != filepath.Base(old) {
		t.Fatalf("configuration/release identities mixed: %+v", state)
	}
	unchanged, err := i.unchanged(state, data)
	require(t, err)
	if !unchanged {
		t.Fatal("upgraded supervisor was not recognized on the next installation")
	}
	after, err := release.Inventory(old, data)
	require(t, err)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("upgrade modified the retained previous release")
	}
}

func TestFailedSameGenerationUpgradeRestoresStateAndPreservesReusedHistory(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "reused"}[reuse], func(t *testing.T) {
			i, backend, data, old := legacyProgram(t)
			before, err := os.ReadFile(filepath.Join(i.Paths.Base, "state.json"))
			require(t, err)
			prepared, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup())
			require(t, err)
			if reuse {
				prepared, err = publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup())
				require(t, err)
				if prepared.created {
					t.Fatal("existing release became owned by this attempt")
				}
			}
			backend.fail = "systemctl restart " + service
			if err := i.activatePrepared(data, prepared); err == nil {
				t.Fatal("failed activation was accepted")
			}
			current, err := i.current()
			require(t, err)
			after, err := os.ReadFile(filepath.Join(i.Paths.Base, "state.json"))
			require(t, err)
			if current != old || !bytes.Equal(before, after) || !backend.active {
				t.Fatal("same-generation rollback lost the committed state or active service")
			}
			_, err = os.Stat(prepared.path)
			if reuse && err != nil || !reuse && !os.IsNotExist(err) {
				t.Fatal("cleanup deleted history or retained its own failed candidate", err)
			}
		})
	}
}

func TestPublicationRejectsTamperingInsteadOfOverwritingExistingRelease(t *testing.T) {
	i, _, data, old := legacyProgram(t)
	prepared, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup())
	require(t, err)
	path := filepath.Join(prepared.path, "nhop-node")
	require(t, os.WriteFile(path, []byte("tampered"), 0755))
	if _, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup()); err == nil {
		t.Fatal("different content at an existing identity was accepted")
	}
	actual, err := os.ReadFile(path)
	require(t, err)
	if string(actual) != "tampered" {
		t.Fatal("existing release was silently overwritten")
	}
	current, err := i.current()
	require(t, err)
	if current != old {
		t.Fatal("rejected publication changed the active release")
	}
}

func TestServiceTemplateRevisionGetsItsOwnReleaseIdentity(t *testing.T) {
	i, _, data, old := legacyProgram(t)
	stage := candidateProgram(t, i, data)
	files, err := release.Inventory(stage, data)
	require(t, err)
	id, err := release.Identity(data.Generation, "previous service template", files)
	require(t, err)
	previous := filepath.Join(filepath.Dir(old), id)
	require(t, os.Rename(stage, previous))
	prepared, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup())
	require(t, err)
	if !prepared.created || prepared.path == previous {
		t.Fatal("service template revision reused a different service identity")
	}
	retained, err := release.Inventory(previous, data)
	require(t, err)
	if !reflect.DeepEqual(files, retained) {
		t.Fatal("service template revision changed the previous files")
	}
}

func TestStateRejectsAReleasePointerThatDisagreesWithCommittedRevision(t *testing.T) {
	i, _, data, _ := legacyProgram(t)
	prepared, err := publish(data, candidateProgram(t, i, data), filepath.Join(i.Paths.Base, "releases"), testGroup())
	require(t, err)
	require(t, i.switchTo(prepared.path))
	if _, err := i.state(data); err == nil {
		t.Fatal("interrupted same-generation update was treated as committed")
	}
}

func TestReleaseInventoryRejectsUnlistedOrNonRegularFiles(t *testing.T) {
	for _, name := range []string{"unexpected", "xray.json"} {
		t.Run(name, func(t *testing.T) {
			i, _, data, _ := legacyProgram(t)
			stage := candidateProgram(t, i, data)
			path := filepath.Join(stage, name)
			if name == "xray.json" {
				require(t, os.Remove(path))
			}
			require(t, os.Mkdir(path, 0700))
			if _, err := release.Inventory(stage, data); err == nil {
				t.Fatal("unexpected directory accepted in immutable release")
			}
		})
	}
	// Legacy schema still requires precisely its binary and configuration.
	_, _, _, directory := setup(t, false)
	require(t, os.Rename(filepath.Join(directory, "config.json"), filepath.Join(directory, "unlisted")))
	if _, err := release.Inventory(directory, model.NodeConfig{SchemaVersion: 1}); err == nil {
		t.Fatal("same-size but incomplete inventory accepted")
	}
}

func TestCommittedStateRejectsPermissionDriftWithoutChangingContents(t *testing.T) {
	for _, change := range []struct {
		path string
		mode os.FileMode
	}{
		{"xray", 0775}, {"config.json", 0644}, {".", 0770}, {"..", 0770}, {"../..", 0770},
	} {
		t.Run(change.path, func(t *testing.T) {
			i, _, data, directory := setup(t, false)
			require(t, i.activate(data, directory))
			before, err := release.Inventory(directory, data)
			require(t, err)
			_, err = i.state(data)
			require(t, err)
			require(t, os.Chmod(filepath.Join(directory, change.path), change.mode))
			if _, err := i.state(data); err == nil {
				t.Fatal("unsafe file permissions were trusted as committed state")
			}
			after, err := release.Inventory(directory, data)
			require(t, err)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("permission-only fixture changed release contents")
			}
		})
	}
}

func TestNewConfigurationCannotUseCorruptCommittedReleaseForRollback(t *testing.T) {
	i, backend, data, directory := setup(t, false)
	require(t, i.activate(data, directory))
	require(t, os.WriteFile(filepath.Join(directory, "xray"), []byte("corrupted old executable"), 0755))
	data.Generation = strings.Repeat("c", 64)
	backend.calls = nil
	if _, err := i.installLocked(data); err == nil || !strings.Contains(err.Error(), "committed release contents differ") {
		t.Fatal("new configuration accepted a corrupt rollback target", err)
	}
	for _, command := range backend.calls {
		if !strings.HasPrefix(command, "getent ") && command != "id -G "+account {
			t.Fatal("untrusted committed state caused service or download activity", backend.calls)
		}
	}
	current, err := i.current()
	require(t, err)
	if current != directory {
		t.Fatal("rejected update changed current")
	}
}

func TestPublicationRejectsPermissionDriftInsteadOfRepairingHistory(t *testing.T) {
	i, _, data, old := legacyProgram(t)
	prepared, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup())
	require(t, err)
	path := filepath.Join(prepared.path, "xray.json")
	require(t, os.Chmod(path, 0660))
	if _, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup()); err == nil {
		t.Fatal("existing unsafe release was reused")
	}
	info, err := os.Lstat(path)
	require(t, err)
	if info.Mode() != 0660 {
		t.Fatal("publication silently repaired historical permissions")
	}
	current, err := i.current()
	require(t, err)
	if current != old {
		t.Fatal("rejected publication changed the active release")
	}
}

func TestExistingServiceAccountCannotSupplyUnexpectedAuthority(t *testing.T) {
	for name, response := range map[string][2]string{
		"root-uid":             {"getent passwd " + account, fmt.Sprintf("%s:x:0:%d::/nonexistent:/usr/sbin/nologin\n", account, testGroup())},
		"root-primary-group":   {"getent passwd " + account, account + ":x:987:0::/nonexistent:/usr/sbin/nologin\n"},
		"extra-group":          {"id -G " + account, fmt.Sprintf("%d 0\n", testGroup())},
		"shared-service-group": {"getent group " + account, fmt.Sprintf("%s:x:%d:other-user\n", account, testGroup())},
		"interactive-shell":    {"getent passwd " + account, fmt.Sprintf("%s:x:987:%d::/nonexistent:/bin/sh\n", account, testGroup())},
	} {
		t.Run(name, func(t *testing.T) {
			i, backend, _, _ := setup(t, false)
			backend.responses = map[string]string{response[0]: response[1]}
			if _, err := i.serviceGroup(); err == nil {
				t.Fatal("unsafe existing service identity accepted")
			}
			for _, command := range backend.calls {
				if strings.HasPrefix(command, "useradd ") {
					t.Fatal("existing account was modified instead of rejected")
				}
			}
		})
	}
}
