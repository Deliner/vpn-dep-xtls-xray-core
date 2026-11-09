package node

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func currentSources(directory, snapshot string) error {
	files, err := manifest(filepath.Join(directory, "source.sha256"))
	if err != nil {
		return err
	}
	expected, err := compiledInputs(snapshot)
	if err != nil {
		return err
	}
	if len(files) != len(expected) {
		return fmt.Errorf("incomplete compiled source inventory")
	}
	for name := range files {
		if !expected[name] {
			return fmt.Errorf("unreviewed compiled input: %s", name)
		}
	}
	return verifyFiles(snapshot, files)
}

func compiledInputs(snapshot string) (map[string]bool, error) {
	expected := map[string]bool{"go.mod": true, "go.sum": true}
	for _, group := range []string{"cmd", "internal", "ops/testing/linux"} {
		err := filepath.WalkDir(filepath.Join(snapshot, group), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext == ".go" || ext == ".html" || ext == ".css" || group == "ops/testing/linux" && ext == ".sh" {
				name, err := filepath.Rel(snapshot, path)
				if err != nil {
					return err
				}
				expected[name] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return expected, nil
}

func previousSources(directory, snapshot, arch string) error {
	raw, err := safeio.Read(filepath.Join(snapshot, "ops/testing/acceptance/installer/previous.tsv"), 4096)
	if err != nil {
		return err
	}
	fields := policyFields(raw)
	if len(fields) != 3 {
		return fmt.Errorf("invalid previous source policy")
	}
	files, err := previousArchive(filepath.Join(directory, "source.tar"), fields[1], fields[2])
	if err != nil {
		return err
	}
	recorded, err := manifest(filepath.Join(directory, "source.sha256"))
	if err != nil || !reflect.DeepEqual(files, recorded) {
		return fmt.Errorf("previous sources differ from the pinned archive")
	}
	if err := verifyTree(filepath.Join(directory, "source"), files); err != nil {
		return err
	}
	identity, err := properties(filepath.Join(directory, "provenance.txt"))
	if err != nil {
		return err
	}
	if identity["project_commit"] != fields[0] || identity["archive_sha256"] != fields[1] || identity["target"] != "linux/"+arch {
		return fmt.Errorf("previous source provenance differs from policy")
	}
	return previousProgram(directory, snapshot, arch)
}

func previousArchive(path, expectedSHA, expectedSize string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil || fmt.Sprint(info.Size()) != expectedSize {
		return nil, fmt.Errorf("previous source archive size changed")
	}
	sha, err := safeio.FileHash(path)
	if err != nil || sha != expectedSHA {
		return nil, fmt.Errorf("previous source archive changed")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return tarHashes(file, "")
}

func policyFields(raw []byte) []string {
	var fields []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields = append(fields, strings.Fields(line)...)
	}
	return fields
}

func previousProgram(directory, snapshot, arch string) error {
	raw, err := safeio.Read(filepath.Join(snapshot, "ops/testing/acceptance/installer/programs.tsv"), 4096)
	if err != nil {
		return err
	}
	fields := policyFields(raw)
	if len(fields) != 4 || fields[0] != "amd64" || fields[2] != "arm64" {
		return fmt.Errorf("invalid previous program policy")
	}
	expected := fields[1]
	if arch == "arm64" {
		expected = fields[3]
	}
	return verifyFiles(filepath.Join(directory, "bin"), map[string]string{"nhop-node": expected})
}
