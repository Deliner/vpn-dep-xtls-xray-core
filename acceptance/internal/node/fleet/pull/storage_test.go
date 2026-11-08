package pull

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPullLockAndClosedRunnerRefuseConcurrentUse(t *testing.T) {
	f := newFixture(t)
	if other, err := Open(f.root); err == nil {
		other.Close()
		t.Fatal("second pull owner accepted")
	}
	require(t, f.runner.Close())
	if _, err := f.runner.Step(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed runner returned %v", err)
	}
	f.open(t)
	if _, err := f.runner.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMissingJournalCannotBeResetByBootstrap(t *testing.T) {
	f := newFixture(t)
	require(t, f.runner.Close())
	require(t, os.Remove(filepath.Join(f.root, "agent", "state.json")))
	if _, err := Open(f.root); err == nil {
		t.Fatal("missing anti-replay journal accepted")
	}
	if err := Initialize(f.root, f.bootstrap); err == nil {
		t.Fatal("bootstrap reset an existing agent")
	}
	if _, err := os.Stat(filepath.Join(f.root, "agent", "state.json")); !os.IsNotExist(err) {
		t.Fatal("missing journal was recreated")
	}
}

func TestInsecureConnectionIsRejectedWithoutRepair(t *testing.T) {
	f := newFixture(t)
	require(t, f.runner.Close())
	path := filepath.Join(f.root, "connection.json")
	require(t, os.Chmod(path, 0644))
	if _, err := Open(f.root); err == nil {
		t.Fatal("public credential file accepted")
	}
	info, err := os.Stat(path)
	require(t, err)
	if info.Mode().Perm() != 0644 {
		t.Fatal("unsafe credentials silently repaired")
	}
}
