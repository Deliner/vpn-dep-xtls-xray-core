package node

import (
	"fmt"
	"path/filepath"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func verifyExecution(directory, reports string, profiles int) error {
	logs := map[string][]string{
		"installer-unit.log": {"TestUnsafeServiceSnapshotCannotStartTransaction", "TestInterruptedHostMutationsRecoverFromDiskWithoutInMemorySnapshot",
			"TestCommitMarkerDistinguishesAppliedAndCommittedTransactions", "TestRecoveryKeepsJournalWhenNewFirewallRuleCannotBeRemoved"},
		"journal-unit.log": {"TestJournalSurvivesKilledWriterAtDurableBoundaries", "TestJournalRejectsChangedUnsafeOrMalformedState"},
		"node-unit.log":    {"TestSignalsStopSupervisorSuccessfullyAndLetEnginesExit", "TestShutdownForcesEnginesThatIgnoreTermination", "TestEngineFailureRemainsAFailedSupervisorExit"},
		"payload.log":      {"TestInstalledNodeLifecycle"},
	}
	for name, tests := range logs {
		if err := testLog(filepath.Join(reports, name), tests); err != nil {
			return err
		}
	}
	raw, err := safeio.Read(filepath.Join(reports, "payload.log"), 10_000_000)
	if err != nil {
		return err
	}
	for _, phase := range phases {
		marker := fmt.Sprintf("%s: %d profiles, %d TCP/UDP streams, 25 barriers", phase, profiles, 2*profiles)
		if !strings.Contains(string(raw), marker) {
			return fmt.Errorf("traffic execution missing: %s", phase)
		}
	}
	race, err := safeio.Read(filepath.Join(directory, "race.log"), 10_000_000)
	if err != nil {
		return err
	}
	for _, name := range []string{"internal/node/installer", "internal/node/journal"} {
		if !strings.Contains(string(race), "ok  \tvpn-script/"+name+"\t") {
			return fmt.Errorf("native race execution missing: %s", name)
		}
	}
	if strings.Contains(string(race), "WARNING: DATA RACE") || strings.Contains(string(race), "\nFAIL") {
		return fmt.Errorf("native race verification failed")
	}
	return textFile(filepath.Join(reports, "status"), "passed")
}

func testLog(path string, tests []string) error {
	raw, err := safeio.Read(path, 10_000_000)
	if err != nil {
		return err
	}
	log := string(raw)
	if !strings.HasSuffix(log, "\nPASS\n") || strings.Contains(log, "--- FAIL:") || strings.Contains(log, "WARNING: DATA RACE") {
		return fmt.Errorf("unsuccessful native test log: %s", path)
	}
	for _, name := range tests {
		if !strings.Contains(log, "\n--- PASS: "+name+" (") {
			return fmt.Errorf("required test did not execute: %s", name)
		}
	}
	return nil
}
