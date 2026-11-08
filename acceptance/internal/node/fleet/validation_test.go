package fleet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/platform/safeio"
)

func TestAgentValidatesSignedManifestBeforeJournalOrInstallation(t *testing.T) {
	for name, change := range map[string]func(*model.NodeConfig){
		"wrong-node":        func(data *model.NodeConfig) { data.NodeID = "exit" },
		"unreviewed-engine": func(data *model.NodeConfig) { data.Runtime.Version = "unreviewed" },
		"open-default":      func(data *model.NodeConfig) { data.Xray.Outbounds[0] = model.Object{"protocol": "freedom"} },
	} {
		t.Run(name, func(t *testing.T) {
			f := newAgentFixture(t)
			change(&f.data)
			f.data = regenerate(t, f.data)
			calls := 0
			f.agent.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) {
				calls++
				return installed(data), nil
			}
			before, err := os.ReadFile(filepath.Join(f.agent.directory, "state.json"))
			require(t, err)
			if _, err := f.agent.Apply(context.Background(), f.signed(t, 1)); err == nil || calls != 0 {
				t.Fatalf("invalid signed manifest reached installer: calls=%d err=%v", calls, err)
			}
			after, err := os.ReadFile(filepath.Join(f.agent.directory, "state.json"))
			require(t, err)
			if string(before) != string(after) {
				t.Fatal("rejected manifest changed the replay checkpoint")
			}
		})
	}
}

func TestAgentRejectsMissingOrDamagedJournalWithoutResetting(t *testing.T) {
	for _, damage := range []string{"missing", "corrupt", "public-mode", "symlink"} {
		t.Run(damage, func(t *testing.T) {
			f := newAgentFixture(t)
			_, err := f.agent.Apply(context.Background(), f.signed(t, 9))
			require(t, err)
			damageJournal(t, f.agent.directory, damage)
			calls := 0
			f.agent.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) {
				calls++
				return installed(data), nil
			}
			if err := f.agent.Initialize(); err == nil {
				t.Fatal("bootstrap silently repaired or reset an existing journal")
			}
			if _, err := f.agent.Apply(context.Background(), f.signed(t, 1)); err == nil || calls != 0 {
				t.Fatalf("damaged journal permitted rollback: calls=%d err=%v", calls, err)
			}
		})
	}
}

func damageJournal(t *testing.T, directory, damage string) {
	t.Helper()
	path := filepath.Join(directory, "state.json")
	switch damage {
	case "missing":
		require(t, os.Remove(path))
	case "corrupt":
		require(t, safeio.Atomic(path, []byte(`{"version":1}`), 0600))
	case "public-mode":
		require(t, os.Chmod(path, 0644))
	case "symlink":
		copy := filepath.Join(directory, "previous.json")
		require(t, os.Rename(path, copy))
		require(t, os.Symlink(copy, path))
	}
}

func TestAgentBindsJournalToProvisionedIdentityAndKey(t *testing.T) {
	for _, change := range []string{"fleet", "node", "key"} {
		f := newAgentFixture(t)
		switch change {
		case "fleet":
			f.agent.trust.FleetID = "another"
		case "node":
			f.agent.trust.NodeID = "exit"
		case "key":
			f.agent.trust.PublicKey[0] ^= 1
		}
		if err := f.agent.Initialize(); err == nil {
			t.Fatalf("journal reused after changing %s", change)
		}
	}
}
