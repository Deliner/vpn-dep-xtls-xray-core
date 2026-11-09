// Package installed independently binds service evidence to its tested inputs.
// It does not import the installer or execute any artifact being inspected.
package installed

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"vpn-script/internal/platform/safeio"
)

const base = "/var/lib/nhop-client"

var header = regexp.MustCompile(`^# nhop-client installation v1 ([0-9a-f]{32}) ([0-9a-f]{64})\n`)

// Verify checks the generation identity, every installed byte and runtime paths.
// The caller separately validates execution, namespace and recovery observations.
func Verify(directory, policy string) error {
	if policy != "tunnel" && policy != "block" {
		return fmt.Errorf("unknown installed client IPv6 policy")
	}
	unit, err := safeio.Read(filepath.Join(directory, "systemd-unit.txt"), 1<<20)
	if err != nil {
		return err
	}
	match := header.FindSubmatch(unit)
	if match == nil {
		return fmt.Errorf("installed unit has no exact ownership header")
	}
	generation := base + "/releases/" + string(match[2])
	hashes, err := inputHashes(directory)
	if err != nil {
		return err
	}
	identity := installationIdentity(hashes, policy)
	digest, err := safeio.Digest(identity)
	if err != nil || digest != string(match[2]) {
		return fmt.Errorf("installed identity does not bind the tested bytes and network policy: %v", err)
	}
	start := fmt.Sprintf(`ExecStart=:"%s/nhop-client" "service-run" "--profile" "%s/profile.json" "--runtimes" "%s/runtimes.json"`, generation, generation, generation)
	if !strings.Contains(string(unit), "\n"+start+" ") {
		return fmt.Errorf("installed unit does not execute its immutable generation")
	}
	if err := verifyRecords(directory, generation, string(match[1]), identity, hashes, safeio.Hash(unit)); err != nil {
		return err
	}
	return verifySnapshot(directory, generation, hashes)
}

func inputHashes(directory string) (map[string]string, error) {
	hashes := make(map[string]string)
	for _, name := range []string{"nhop-client", "xray", "sing-box", "libcronet.so", "profile.json"} {
		path := filepath.Join(directory, "bin", name)
		if name == "profile.json" {
			path = filepath.Join(directory, "installed-profile.json")
		}
		digest, err := safeio.FileHash(path)
		if err != nil {
			return nil, err
		}
		hashes[name] = digest
	}
	return hashes, nil
}

func installationIdentity(hashes map[string]string, policy string) map[string]any {
	return map[string]any{"schema_version": 1, "hashes": hashes, "network": map[string]any{
		"interface": "nhop0", "engine_uid": 0, "ipv6": policy,
		"dns_ip": "198.18.0.2", "dns_name": "dns.example.invalid", "mtu": 1280}}
}

func verifyRecords(directory, generation, token string, identity map[string]any, hashes map[string]string, unitHash string) error {
	inventory := map[string]any{"schema_version": 1}
	for key, name := range map[string]string{"xray": "xray", "sing_box": "sing-box", "cronet": "libcronet.so"} {
		inventory[key] = map[string]string{"path": generation + "/" + name, "sha256": hashes[name]}
	}
	records := map[string]any{
		"installed-owner.json":    map[string]any{"schema_version": 1, "token": token},
		"installed-release.json":  map[string]any{"identity": identity, "unit_sha256": unitHash},
		"installed-runtimes.json": inventory,
	}
	for name, expected := range records {
		if err := verifyJSON(filepath.Join(directory, name), expected); err != nil {
			return err
		}
	}
	return nil
}

func verifySnapshot(directory, generation string, hashes map[string]string) error {
	snapshot := map[string]string{base + "/install.lock": safeio.Hash(nil)}
	for name, digest := range hashes {
		snapshot[generation+"/"+name] = digest
	}
	paths := map[string]string{
		"installed-owner.json": base + "/owner.json", "systemd-unit.txt": generation + "/unit",
		"installed-release.json": generation + "/release.json", "installed-runtimes.json": generation + "/runtimes.json",
	}
	for name, target := range paths {
		digest, err := safeio.FileHash(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		snapshot[target] = digest
	}
	return verifyJSON(filepath.Join(directory, "installed-snapshot.json"), snapshot)
}

func verifyJSON(path string, expected any) error {
	var actual any
	if err := safeio.LoadJSON(path, &actual); err != nil {
		return err
	}
	left, err := safeio.Digest(actual)
	if err != nil {
		return err
	}
	right, err := safeio.Digest(expected)
	if err != nil || left != right {
		return fmt.Errorf("installed evidence binding mismatch: %s: %v", filepath.Base(path), err)
	}
	return nil
}
