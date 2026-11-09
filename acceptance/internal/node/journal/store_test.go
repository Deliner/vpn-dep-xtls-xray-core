package journal

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixtureRecord(base string) Record {
	return Record{Schema: 1, Phase: "pending", TopologyID: "test", NodeID: "entry",
		Next: Snapshot{Previous: filepath.Join(base, "releases", strings.Repeat("a", 64)),
			Unit: []byte("next unit"), State: []byte("next state"), Active: true, Enabled: true}}
}

func TestJournalNeverOverwritesPendingWorkAndCommitsExplicitly(t *testing.T) {
	base := t.TempDir()
	record := fixtureRecord(base)
	if err := Begin(base, record); err != nil {
		t.Fatal(err)
	}
	if err := Begin(base, record); err == nil {
		t.Fatal("pending transaction overwritten")
	}
	loaded, err := Load(base)
	if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, record) {
		t.Fatal("persisted intent changed", err)
	}
	committed, err := Commit(base, record)
	if err != nil || committed.Phase != "committed" {
		t.Fatal("commit marker not durable", err)
	}
	if err := Clear(base, record); err == nil {
		t.Fatal("stale pending record removed a committed transaction")
	}
	if err := Clear(base, committed); err != nil {
		t.Fatal(err)
	}
	if loaded, err := Load(base); err != nil || loaded != nil {
		t.Fatal("cleared journal survived", err)
	}
}

func TestJournalRejectsChangedUnsafeOrMalformedState(t *testing.T) {
	for _, fault := range []string{"mode", "symlink", "malformed", "changed", "traversal"} {
		t.Run(fault, func(t *testing.T) {
			base := t.TempDir()
			record := fixtureRecord(base)
			if err := Begin(base, record); err != nil {
				t.Fatal(err)
			}
			corrupt(t, base, record, fault)
			before, err := os.ReadFile(filepath.Join(base, Name))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Commit(base, record); err == nil {
				t.Fatal("changed journal accepted for commit")
			}
			if err := Clear(base, record); err == nil {
				t.Fatal("changed journal discarded")
			}
			after, err := os.ReadFile(filepath.Join(base, Name))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("rejected record changed", err)
			}
		})
	}
}

func corrupt(t *testing.T, base string, record Record, fault string) {
	t.Helper()
	path := filepath.Join(base, Name)
	var err error
	switch fault {
	case "mode":
		err = os.Chmod(path, 0644)
	case "symlink":
		if err = os.Rename(path, path+".saved"); err == nil {
			err = os.Symlink(path+".saved", path)
		}
	case "malformed":
		err = os.WriteFile(path, []byte("truncated"), 0600)
	case "changed", "traversal":
		loaded, loadErr := os.ReadFile(path)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		from, to := []byte(`"entry"`), []byte(`"another"`)
		if fault == "traversal" {
			from, to = []byte(record.Next.Previous), []byte("/tmp/foreign-release")
		}
		err = os.WriteFile(path, bytes.ReplaceAll(loaded, from, to), 0600)
	}
	if err != nil {
		t.Fatal(err)
	}
}
