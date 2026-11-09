package netem

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func readExperiment(root, policy, snapshot string) (experiment, error) {
	value := experiment{root: root, policy: policy}
	var err error
	value.inputs, err = experimentInputs(root, policy, snapshot)
	if err != nil {
		return value, err
	}
	reports := filepath.Join(root, "reports")
	if _, err := completeInventory(reports, "evidence.sha256"); err != nil {
		return value, err
	}
	if err := experimentIsolation(reports); err != nil {
		return value, err
	}
	value.result, err = verifyPhases(reports, policy)
	if err != nil {
		return value, err
	}
	value.result.Packets, err = packetCapture(reports)
	if err != nil {
		return value, err
	}
	return value, experimentOutcome(reports, value.result)
}

func experimentInputs(root, policy, snapshot string) (map[string]string, error) {
	manifest, err := readManifest(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		return nil, err
	}
	congestion := "\n"
	if policy == "reno" {
		congestion = "reno\n"
	}
	for name, want := range map[string]string{"cycles": "8\n", "workers": "8\n", "packets": "32\n", "congestion": congestion,
		"execution-context": "owned-native-ci\n", "runtime-kind": "runtime-candidate\n"} {
		if err := textFile(filepath.Join(root, name), want); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"guest.sh", "namespace.sh", "context.sh"} {
		if err := equalFiles(filepath.Join(root, name), filepath.Join(snapshot, "ops/testing/diagnostics/netem", name)); err != nil {
			return nil, err
		}
	}
	files, err := regularFiles(root)
	if err != nil {
		return nil, err
	}
	for name := range files {
		if name == "SHA256SUMS" || strings.HasPrefix(name, "reports/") {
			continue
		}
		actual, err := safeio.FileHash(filepath.Join(root, name))
		if err != nil || manifest[name] != actual {
			return nil, fmt.Errorf("unrecorded experiment input: %s", name)
		}
	}
	return manifest, expectedInputs(manifest)
}

func expectedInputs(manifest map[string]string) error {
	names := []string{"cycles", "workers", "packets", "congestion", "execution-context", "runtime-kind", "guest.sh", "namespace.sh", "context.sh",
		"network.test", "source.txt", "bin/xray", "runtime.inventory.sha256", "runtime.binary.sha256", "provenance.txt"}
	if len(manifest) != len(names) {
		return fmt.Errorf("unexpected experiment input inventory")
	}
	for _, name := range names {
		if manifest[name] == "" {
			return fmt.Errorf("missing experiment input: %s", name)
		}
	}
	return nil
}

func experimentIsolation(root string) error {
	identity, err := properties(filepath.Join(root, "isolation.txt"))
	if err != nil {
		return err
	}
	for _, name := range []string{"net", "mnt", "pid"} {
		pattern := regexp.MustCompile("^" + name + `:\[[1-9][0-9]*\]$`)
		if !pattern.MatchString(identity[name]) || !pattern.MatchString(identity["host_"+name]) || identity[name] == identity["host_"+name] {
			return fmt.Errorf("missing private %s namespace", name)
		}
	}
	var attribution map[string]any
	if err := readJSON(filepath.Join(root, "queue-attribution.json"), &attribution); err != nil {
		return err
	}
	want := map[string]any{"available": false, "reason": "runtime-candidate has no diagnostic queue observer"}
	if !reflect.DeepEqual(attribution, want) {
		return fmt.Errorf("ordinary candidate invented queue observations")
	}
	firewall, err := safeio.Read(filepath.Join(root, "h3-only-firewall.txt"), 1<<20)
	if err != nil || !regexp.MustCompile(`tcp dport 24443 counter packets [0-9]+ bytes [0-9]+ reject`).Match(firewall) {
		return fmt.Errorf("missing H2 fallback refusal: %v", err)
	}
	return nil
}

func experimentOutcome(root string, result Result) error {
	status, code, terminal := "passed\n", "0\n", "--- PASS: TestMASQUENetworkFaults ("
	if result.DelayFailures != 0 {
		status, code, terminal = "failed\n", "1\n", "--- FAIL: TestMASQUENetworkFaults ("
	}
	if err := textFile(filepath.Join(root, "status"), status); err != nil {
		return err
	}
	if err := textFile(filepath.Join(root, "../../exit-code"), code); err != nil {
		return err
	}
	raw, err := safeio.Read(filepath.Join(root, "experiment.log"), 16<<20)
	if err != nil {
		return err
	}
	log := string(raw)
	if strings.Count(log, terminal) != 1 || strings.Count(log, "loss-free phase did not complete:") != result.DelayFailures ||
		strings.Contains(log, "panic:") || strings.Contains(log, "WARNING: DATA RACE") || strings.Contains(log, "unexpected flow result") {
		return fmt.Errorf("experiment outcome differs from observed phases")
	}
	return processEvidence(root)
}

func processEvidence(root string) error {
	for _, role := range []string{"client", "server"} {
		paths, err := filepath.Glob(filepath.Join(root, role+"-*.log"))
		if err != nil || len(paths) != 9 {
			return fmt.Errorf("missing initial process or eight %s restarts", role)
		}
		for _, path := range paths {
			raw, err := safeio.Read(path, 32<<20)
			if err != nil || strings.Contains(string(raw), "panic:") || strings.Contains(string(raw), "NHOP_") ||
				!strings.Contains(string(raw), "nhop-fork-masque-sack-1 (go1.27.3 linux/") {
				return fmt.Errorf("invalid ordinary process log: %s: %v", path, err)
			}
		}
	}
	return nil
}

func compareInputs(first, second experiment) error {
	if len(first.inputs) != len(second.inputs) {
		return fmt.Errorf("congestion pair input sets differ")
	}
	for name, digest := range first.inputs {
		if name != "congestion" && second.inputs[name] != digest {
			return fmt.Errorf("congestion pair changed %s", name)
		}
	}
	if first.inputs["congestion"] == second.inputs["congestion"] {
		return fmt.Errorf("congestion pair has no distinct control")
	}
	return compareConfigs(first, second)
}

func bindInputs(value experiment, root string) error {
	for name, expected := range map[string]string{"network.test": "network.test", "source.txt": "source.txt", "bin/xray": "runtime/bin/xray",
		"provenance.txt": "runtime/provenance.txt", "runtime.inventory.sha256": "runtime/SHA256SUMS"} {
		if err := equalFiles(filepath.Join(value.root, name), filepath.Join(root, expected)); err != nil {
			return err
		}
	}
	if err := textFile(filepath.Join(value.root, "runtime.binary.sha256"), value.inputs["bin/xray"]+"  bin/xray\n"); err != nil {
		return err
	}
	identity, err := safeio.Read(filepath.Join(value.root, "reports/identity.txt"), 1<<20)
	if err != nil {
		return err
	}
	policy := value.policy
	if policy == "default" {
		policy = ""
	}
	for _, expected := range []string{"execution_context=owned-native-ci\n", "runtime_kind=runtime-candidate\n", "loss_seed=none\n", "congestion=" + policy + "\n",
		value.inputs["network.test"] + "  network.test\n", value.inputs["bin/xray"] + "  bin/xray\n"} {
		if !strings.Contains(string(identity), expected) {
			return fmt.Errorf("execution identity omits %s", strconv.Quote(expected))
		}
	}
	return matchingCredentials(filepath.Join(value.root, "reports"))
}
