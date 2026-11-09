package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/journal"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

type interrupted struct{}

func TestUnsafeServiceSnapshotCannotStartTransaction(t *testing.T) {
	for _, raw := range []string{
		"LoadState=not-found\nActiveState=inactive\nUnexpected=\n",
		"LoadState=not-found\nActiveState=inactive\n",
		"LoadState=loaded\nActiveState=active\nActiveState=inactive\nUnitFileState=enabled\n",
		"LoadState=loaded\nActiveState=activating\nUnitFileState=enabled\n",
		"LoadState=masked\nActiveState=inactive\nUnitFileState=masked\n",
	} {
		t.Run(strings.ReplaceAll(strings.TrimSpace(raw), "\n", "/"), func(t *testing.T) {
			i, b, data, next, before := recoveryFixture(t)
			b.responses = map[string]string{"systemctl show -p LoadState -p ActiveState -p UnitFileState " + service: raw}
			if err := i.activate(data, next); err == nil {
				t.Fatal("unsafe service state was accepted as a rollback snapshot")
			}
			delete(b.responses, "systemctl show -p LoadState -p ActiveState -p UnitFileState "+service)
			after, err := i.snapshot()
			require(t, err)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("invalid snapshot modified the managed deployment")
			}
			record, err := journal.Load(i.Paths.Base)
			require(t, err)
			if record != nil {
				t.Fatal("invalid snapshot started a durable transaction")
			}
		})
	}
}

func recoveryFixture(t *testing.T) (*Installer, *backend, model.NodeConfig, string, Snapshot) {
	t.Helper()
	i, b, data, initial := setup(t, false)
	require(t, i.activate(data, initial))
	old, err := i.snapshot()
	require(t, err)
	data.Generation = strings.Repeat("c", 64)
	return i, b, data, testRelease(t, i.Paths.Base, data.Generation), old
}

func interruptAt(t *testing.T, i *Installer, b *backend, data model.NodeConfig, next, command string) {
	t.Helper()
	i.Run = func(ctx context.Context, limit time.Duration, args ...string) (process.Result, error) {
		if strings.Join(args, " ") == command {
			panic(interrupted{})
		}
		return b.run(ctx, limit, args...)
	}
	defer func() {
		i.Run = b.run
		if _, ok := recover().(interrupted); !ok {
			t.Fatal("test did not reach the interruption boundary")
		}
	}()
	_ = i.activate(data, next)
}

func TestInterruptedHostMutationsRecoverFromDiskWithoutInMemorySnapshot(t *testing.T) {
	for _, command := range []string{"systemctl daemon-reload", "systemctl restart " + service,
		"ufw app update VPN-NHOP", "ufw allow VPN-NHOP", "systemctl enable " + service} {
		t.Run(command, func(t *testing.T) {
			i, b, data, next, old := recoveryFixture(t)
			data.Firewall = "ufw"
			interruptAt(t, i, b, data, next, command)
			restarted := New(context.Background())
			restarted.Paths, restarted.Run, restarted.Sleep = i.Paths, b.run, i.Sleep
			require(t, restarted.recoverTransaction(data))
			after, err := restarted.snapshot()
			require(t, err)
			if !reflect.DeepEqual(old, after) {
				t.Fatal("interrupted update did not restore the complete previous snapshot")
			}
			record, err := journal.Load(i.Paths.Base)
			require(t, err)
			if record != nil {
				t.Fatal("finished recovery retained an active transaction")
			}
			require(t, restarted.recoverTransaction(data))
			require(t, restarted.activate(data, next))
		})
	}
}

func transactionFixture(t *testing.T, committed bool) (*Installer, *backend, model.NodeConfig, journal.Record) {
	t.Helper()
	i, b, data, next, old := recoveryFixture(t)
	state, err := i.nextState(data, next, old)
	require(t, err)
	record := journal.Record{Schema: 1, Phase: "pending", TopologyID: data.TopologyID, NodeID: data.NodeID, Old: old,
		Next: Snapshot{Previous: next, Unit: []byte(unitFor(data)), State: state, Active: true, Enabled: true}}
	require(t, journal.Begin(i.Paths.Base, record))
	_, err = i.apply(data, next, record.Next)
	require(t, err)
	if committed {
		record, err = journal.Commit(i.Paths.Base, record)
		require(t, err)
	}
	return i, b, data, record
}

func TestCommitMarkerDistinguishesAppliedAndCommittedTransactions(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "committed"}[committed], func(t *testing.T) {
			i, b, data, record := transactionFixture(t, committed)
			b.calls = nil
			require(t, i.recoverTransaction(data))
			want := record.Old
			if committed {
				want = record.Next
				for _, call := range b.calls {
					if strings.HasPrefix(call, "systemctl ") {
						t.Fatal("committed cleanup unnecessarily changed service state", b.calls)
					}
				}
			}
			after, err := i.snapshot()
			require(t, err)
			if !reflect.DeepEqual(after, want) {
				t.Fatal("wrong transaction side survived recovery")
			}
		})
	}
}

func TestRecoveryRefusesUntrustedStateAndKeepsJournal(t *testing.T) {
	for _, fault := range []string{"old-binary", "foreign-unit", "foreign-state", "wrong-node", "stop-failed"} {
		t.Run(fault, func(t *testing.T) {
			i, b, data, record := transactionFixture(t, false)
			switch fault {
			case "old-binary":
				require(t, os.WriteFile(filepath.Join(record.Old.Previous, "xray"), []byte("changed"), 0755))
			case "foreign-unit":
				require(t, os.WriteFile(i.Paths.Unit, []byte("foreign unit"), 0644))
			case "foreign-state":
				require(t, os.WriteFile(filepath.Join(i.Paths.Base, "state.json"), []byte("foreign state"), 0600))
			case "wrong-node":
				data.NodeID = "another"
			case "stop-failed":
				b.fail = "systemctl stop " + service
			}
			before, err := os.ReadFile(filepath.Join(i.Paths.Base, journal.Name))
			require(t, err)
			if err := i.recoverTransaction(data); err == nil {
				t.Fatal("unsafe recovery succeeded")
			}
			after, err := os.ReadFile(filepath.Join(i.Paths.Base, journal.Name))
			require(t, err)
			if !bytes.Equal(before, after) {
				t.Fatal("failed recovery discarded or modified its durable record")
			}
		})
	}
}

func TestInitialInterruptedInstallationRestoresAbsence(t *testing.T) {
	i, b, data, next := setup(t, false)
	interruptAt(t, i, b, data, next, "systemctl restart "+service)
	require(t, i.recoverTransaction(data))
	for _, path := range []string{i.Paths.Unit, "current", "state.json", journal.Name} {
		if !filepath.IsAbs(path) {
			path = filepath.Join(i.Paths.Base, path)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("initial recovery left managed state", path, err)
		}
	}
	if b.active || b.enabled {
		t.Fatal("initial recovery left the new service active or enabled")
	}
}

func TestRecoveryKeepsJournalWhenNewFirewallRuleCannotBeRemoved(t *testing.T) {
	i, b, data, next, old := recoveryFixture(t)
	data.Firewall = "ufw"
	interruptAt(t, i, b, data, next, "systemctl enable "+service)
	before, err := journal.Load(i.Paths.Base)
	require(t, err)
	b.fail = "ufw --force delete allow VPN-NHOP"
	if err := i.recoverTransaction(data); err == nil {
		t.Fatal("firewall rollback failure was ignored")
	}
	after, err := journal.Load(i.Paths.Base)
	require(t, err)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed firewall recovery lost the transaction")
	}
	if _, err := os.Stat(i.Paths.Firewall); err != nil {
		t.Fatal("failed deletion discarded the rule's application profile", err)
	}
	require(t, i.recoverTransaction(data))
	restored, err := i.snapshot()
	require(t, err)
	if !reflect.DeepEqual(old, restored) {
		t.Fatal("retried firewall recovery did not restore the previous deployment")
	}
}

func TestCommittedRecoveryRejectsChangedFirewallProfile(t *testing.T) {
	i, _, data, next, old := recoveryFixture(t)
	data.Firewall = "ufw"
	state, err := i.nextState(data, next, old)
	require(t, err)
	record := journal.Record{Schema: 1, Phase: "pending", TopologyID: data.TopologyID, NodeID: data.NodeID,
		FirewallChanged: true, Old: old, Next: Snapshot{Previous: next, State: state,
			Unit: []byte(unitFor(data)), Firewall: []byte(UFWProfile(data)), Active: true, Enabled: true}}
	require(t, journal.Begin(i.Paths.Base, record))
	_, err = i.apply(data, next, record.Next)
	require(t, err)
	record, err = journal.Commit(i.Paths.Base, record)
	require(t, err)
	require(t, os.Remove(i.Paths.Firewall))
	if err := i.recoverTransaction(data); err == nil {
		t.Fatal("committed state accepted the previous firewall profile")
	}
	retained, err := journal.Load(i.Paths.Base)
	require(t, err)
	if retained == nil || !reflect.DeepEqual(record, *retained) {
		t.Fatal("changed committed deployment lost its journal")
	}
}

func TestRecoveryPreservesUnchangedGenerationWithDifferentSupervisor(t *testing.T) {
	i, b, data, old := legacyProgram(t)
	prepared, err := publish(data, candidateProgram(t, i, data), filepath.Dir(old), testGroup())
	require(t, err)
	before, err := os.ReadFile(filepath.Join(i.Paths.Base, "state.json"))
	require(t, err)
	interruptAt(t, i, b, data, prepared.path, "systemctl enable "+service)
	require(t, i.recoverTransaction(data))
	after, err := os.ReadFile(filepath.Join(i.Paths.Base, "state.json"))
	require(t, err)
	current, err := i.current()
	require(t, err)
	if current != old || !bytes.Equal(before, after) {
		t.Fatal("same-generation interrupted supervisor upgrade lost the old release")
	}
	require(t, safeio.CheckAccess(filepath.Join(i.Paths.Base, "state.json"), 0600, os.Geteuid(), -1))
}
