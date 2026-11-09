package netem

import (
	"fmt"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

// Verify checks complete evidence independently of the network test producer.
// A default-BBR delay failure is retained as a result; it cannot excuse a missing
// control, a broken harness, an unrelated phase failure, or any Reno failure.
func Verify(root, arch, commit, snapshot string) ([]Result, error) {
	if _, err := completeInventory(root, "evidence.sha256"); err != nil {
		return nil, err
	}
	status, err := safeio.Read(filepath.Join(root, "status"), 32)
	if err != nil || string(status) != "passed\n" && string(status) != "collected\n" {
		return nil, fmt.Errorf("incomplete native collection: %v", err)
	}
	if err := nativeIdentity(root, arch, commit, snapshot); err != nil {
		return nil, err
	}
	if err := verifyRuntime(root, arch, snapshot); err != nil {
		return nil, err
	}
	if err := textFile(filepath.Join(root, "plan.tsv"), "0\tdefault\n0\treno\n1\treno\n1\tdefault\n"); err != nil {
		return nil, err
	}
	var results []Result
	for round := range 2 {
		pair, err := verifyPair(root, round, snapshot)
		if err != nil {
			return nil, fmt.Errorf("round %d: %w", round, err)
		}
		results = append(results, pair...)
	}
	return results, nil
}

func verifyPair(root string, round int, snapshot string) ([]Result, error) {
	var pair []experiment
	for _, policy := range []string{"default", "reno"} {
		path := filepath.Join(root, "runs", fmt.Sprintf("%d-%s", round, policy), "experiment")
		value, err := readExperiment(path, policy, snapshot)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", policy, err)
		}
		if err := bindInputs(value, root); err != nil {
			return nil, err
		}
		value.result.Round = round
		pair = append(pair, value)
	}
	if err := compareInputs(pair[0], pair[1]); err != nil {
		return nil, err
	}
	return []Result{pair[0].result, pair[1].result}, nil
}
