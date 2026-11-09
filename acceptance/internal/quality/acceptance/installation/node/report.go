package node

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"vpn-script/internal/core/protocol"
	"vpn-script/internal/core/validate"
)

var phases = []string{"initial", "installer-rolled-back", "installer-upgraded", "access-restored", "interrupted-restored", "updated", "rolled-back", "recovered"}
var flags = []string{"unchanged_pid", "rolled_back", "stopped_service_recovered", "old_ports_released", "clean_stop",
	"installer_upgraded", "installer_upgrade_rolled_back", "cache_miss_blocked", "cache_offline_lifecycle", "installed_access_protected",
	"service_identity_protected", "udp22_with_existing_tcp_ssh", "disjoint_foreign_tcp_preserved"}

func verifyReport(path string) (report, error) {
	var result report
	if err := readJSON(path, &result); err != nil {
		return result, err
	}
	var fields map[string]json.RawMessage
	if err := readJSON(path, &fields); err != nil {
		return result, err
	}
	for _, name := range flags {
		if string(fields[name]) != "true" {
			return result, fmt.Errorf("unproven installer contract: %s", name)
		}
	}
	if result.Schema != 1 || result.Status != "passed" || result.ColdRequests < 1 || result.WarmRequests != 0 ||
		!validate.Hash(result.PreviousSHA) || !validate.Hash(result.InstallerSHA) || result.PreviousSHA == result.InstallerSHA {
		return result, fmt.Errorf("invalid installer lifecycle identity or cache result")
	}
	return result, verifyTraffic(result)
}

func verifyTraffic(result report) error {
	profiles := []string{}
	for _, spec := range protocol.All() {
		if spec.Status == "implemented" {
			profiles = append(profiles, spec.ID)
		}
	}
	actual := slices.Clone(result.Profiles)
	slices.Sort(actual)
	if !slices.Equal(actual, profiles) || len(result.Phases) != len(phases) {
		return fmt.Errorf("installer did not exercise every profile and lifecycle phase")
	}
	for index, observed := range result.Phases {
		if observed.Name != phases[index] || observed.Profiles != len(profiles) || observed.Streams != 2*len(profiles) ||
			observed.Required != 25 || observed.Barriers != 25 || observed.TCPBytes != 32768 || observed.UDPBytes != 512 ||
			len(observed.Missing) != 0 || len(observed.Routes) != len(profiles) {
			return fmt.Errorf("incomplete concurrent installer phase: %s", observed.Name)
		}
		for _, name := range profiles {
			value, ok := observed.Routes[name]
			if !ok || value.TCP != 25 || value.UDP != 25 || value.Error != "" {
				return fmt.Errorf("incomplete %s traffic in %s", name, observed.Name)
			}
		}
	}
	return nil
}

func verifyCrash(directory string, result report) error {
	c := result.Interruption
	if c.InstallerPID <= 0 || c.ControlPID <= 0 || c.Signal != "SIGKILL" || c.SubState != "start-pre" || !c.Restored ||
		!validate.Hash(c.Previous) || !validate.Hash(c.Next) || c.Previous == c.Next || !validate.Hash(c.JournalSHA) {
		return fmt.Errorf("missing actual interrupted restart and recovery proof")
	}
	path := filepath.Join(directory, "interrupted-transaction.json")
	if err := verifyFiles(directory, map[string]string{"interrupted-transaction.json": c.JournalSHA}); err != nil {
		return err
	}
	var record transaction
	if err := readJSON(path, &record); err != nil {
		return err
	}
	if record.Schema != 1 || record.Phase != "pending" || !record.Old.Active || !record.Old.Enabled ||
		record.Old.Release != "/opt/vpn-nhop/releases/"+c.Previous || record.Next.Release != "/opt/vpn-nhop/releases/"+c.Next {
		return fmt.Errorf("journal does not bind the observed interruption")
	}
	for _, snapshot := range []snapshot{record.Old, record.Next} {
		var state struct {
			Files   map[string]string `json:"files"`
			Release string            `json:"release_id"`
		}
		if err := json.Unmarshal(snapshot.State, &state); err != nil {
			return err
		}
		if state.Release != filepath.Base(snapshot.Release) || state.Files["nhop-node"] != result.InstallerSHA {
			return fmt.Errorf("interrupted state names another supervisor or release")
		}
		for _, name := range []string{"xray", "sing-box"} {
			if state.Files[name] != result.Runtimes["server/"+name].SHA {
				return fmt.Errorf("journal names another %s", name)
			}
		}
	}
	return nil
}
