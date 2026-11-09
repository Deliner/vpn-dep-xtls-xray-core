package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/core/protocol"
	"vpn-script/internal/platform/safeio"
)

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path string, contents []byte) {
	t.Helper()
	require(t, os.MkdirAll(filepath.Dir(path), 0700))
	require(t, os.WriteFile(path, contents, 0600))
}

func fixtureReport() report {
	result := report{Schema: 1, Status: "passed", ColdRequests: 1,
		PreviousSHA: strings.Repeat("a", 64), InstallerSHA: strings.Repeat("b", 64)}
	for _, spec := range protocol.All() {
		if spec.Status == "implemented" {
			result.Profiles = append(result.Profiles, spec.ID)
		}
	}
	for _, name := range phases {
		phase := phase{Name: name, Profiles: len(result.Profiles), Required: 25, Barriers: 25,
			Streams: 2 * len(result.Profiles), TCPBytes: 32768, UDPBytes: 512, Routes: map[string]rounds{}}
		for _, profile := range result.Profiles {
			phase.Routes[profile] = rounds{TCP: 25, UDP: 25}
		}
		result.Phases = append(result.Phases, phase)
	}
	return result
}

func fixtureReportFile(t *testing.T, result report) string {
	t.Helper()
	raw, err := json.Marshal(result)
	require(t, err)
	var fields map[string]any
	require(t, json.Unmarshal(raw, &fields))
	for _, name := range flags {
		fields[name] = true
	}
	path := filepath.Join(t.TempDir(), "installed.json")
	require(t, safeio.JSON(path, fields))
	return path
}

func fixtureCrash(t *testing.T, mutate func(*transaction)) (string, report) {
	t.Helper()
	directory := t.TempDir()
	result := fixtureReport()
	result.Runtimes = map[string]runtimeRecord{"server/xray": {SHA: strings.Repeat("c", 64)}, "server/sing-box": {SHA: strings.Repeat("d", 64)}}
	result.Interruption = crash{InstallerPID: 123, ControlPID: 234, Signal: "SIGKILL", SubState: "start-pre",
		Previous: strings.Repeat("e", 64), Next: strings.Repeat("f", 64), Restored: true}
	old := fixtureSnapshot(t, result, result.Interruption.Previous)
	next := fixtureSnapshot(t, result, result.Interruption.Next)
	record := transaction{Schema: 1, Phase: "pending", Old: old, Next: next}
	mutate(&record)
	path := filepath.Join(directory, "interrupted-transaction.json")
	require(t, safeio.JSON(path, record))
	var err error
	result.Interruption.JournalSHA, err = safeio.FileHash(path)
	require(t, err)
	return directory, result
}

func fixtureSnapshot(t *testing.T, result report, release string) snapshot {
	t.Helper()
	state := map[string]any{"release_id": release, "files": map[string]string{"nhop-node": result.InstallerSHA,
		"xray": result.Runtimes["server/xray"].SHA, "sing-box": result.Runtimes["server/sing-box"].SHA}}
	raw, err := json.Marshal(state)
	require(t, err)
	return snapshot{Release: "/opt/vpn-nhop/releases/" + release, State: raw, Active: true, Enabled: true}
}

func fixtureInventory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	write(t, filepath.Join(directory, "value"), []byte("original"))
	write(t, filepath.Join(directory, "evidence.sha256"), []byte(fmt.Sprintf("%s  ./value\n", safeio.Hash([]byte("original")))))
	return directory
}
