package journal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestJournalSurvivesKilledWriterAtDurableBoundaries(t *testing.T) {
	for _, boundary := range []string{"pending", "committed", "cleared"} {
		t.Run(boundary, func(t *testing.T) {
			base := t.TempDir()
			killWriter(t, base, boundary)
			record, err := Load(base)
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "cleared" {
				if record != nil {
					t.Fatal("completed deletion lost after writer death")
				}
			} else if record == nil || record.Phase != boundary {
				t.Fatal("durable boundary lost after writer death")
			}
		})
	}
}

func killWriter(t *testing.T, base, boundary string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestJournalWriterProcess$")
	command.Env = append(os.Environ(), "NHOP_JOURNAL_WRITER="+base, "NHOP_JOURNAL_BOUNDARY="+boundary)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "durable\n" {
		t.Fatalf("writer did not reach the boundary: %q %v", line, err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("writer exited normally instead of being killed")
	}
}

func TestJournalWriterProcess(t *testing.T) {
	base := os.Getenv("NHOP_JOURNAL_WRITER")
	if base == "" {
		return
	}
	record := fixtureRecord(base)
	if err := Begin(base, record); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("NHOP_JOURNAL_BOUNDARY") != "pending" {
		var err error
		record, err = Commit(base, record)
		if err != nil {
			t.Fatal(err)
		}
	}
	if os.Getenv("NHOP_JOURNAL_BOUNDARY") == "cleared" {
		if err := Clear(base, record); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Fprint(os.Stdout, "durable\n")
	io.Copy(io.Discard, os.Stdin)
}
