package node

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestPreviousSourcesRejectRehashedChangesAndUnexpectedCompilerInputs(t *testing.T) {
	for _, damage := range []string{"", "source", "extra", "program", "archive", "provenance"} {
		t.Run(damage, func(t *testing.T) {
			previous, snapshot := previousFixture(t)
			switch damage {
			case "source":
				write(t, filepath.Join(previous, "source/main.go"), []byte("changed"))
				write(t, filepath.Join(previous, "source.sha256"), []byte(safeio.Hash([]byte("changed"))+"  ./main.go\n"))
			case "extra":
				write(t, filepath.Join(previous, "source/extra.go"), []byte("package extra"))
			case "program":
				write(t, filepath.Join(previous, "bin/nhop-node"), []byte("other program"))
			case "archive":
				write(t, filepath.Join(previous, "source.tar"), []byte("other archive"))
			case "provenance":
				write(t, filepath.Join(previous, "provenance.txt"), []byte("target=linux/arm64\n"))
			}
			if err := previousSources(previous, snapshot, "amd64"); (err == nil) != (damage == "") {
				t.Fatalf("previous source identity mismatch: %v", err)
			}
		})
	}
}

func previousFixture(t *testing.T) (string, string) {
	t.Helper()
	previous, snapshot := t.TempDir(), t.TempDir()
	raw := []byte("package example\n")
	commit := strings.Repeat("a", 40)
	var buffer bytes.Buffer
	archive := tar.NewWriter(&buffer)
	require(t, archive.WriteHeader(&tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": commit}}))
	require(t, archive.WriteHeader(&tar.Header{Name: "main.go", Mode: 0600, Size: int64(len(raw))}))
	_, err := archive.Write(raw)
	require(t, err)
	require(t, archive.Close())
	write(t, filepath.Join(previous, "source.tar"), buffer.Bytes())
	write(t, filepath.Join(previous, "source/main.go"), raw)
	write(t, filepath.Join(previous, "source.sha256"), []byte(safeio.Hash(raw)+"  ./main.go\n"))
	write(t, filepath.Join(previous, "bin/nhop-node"), []byte("program"))
	sha := safeio.Hash(buffer.Bytes())
	write(t, filepath.Join(previous, "provenance.txt"), []byte(fmt.Sprintf("project_commit=%s\narchive_sha256=%s\ntarget=linux/amd64\n", commit, sha)))
	policy := filepath.Join(snapshot, "ops/testing/acceptance/installer")
	write(t, filepath.Join(policy, "previous.tsv"), []byte(fmt.Sprintf("%s\t%s\t%d\n", commit, sha, buffer.Len())))
	write(t, filepath.Join(policy, "programs.tsv"), []byte(fmt.Sprintf("amd64\t%s\narm64\t%s\n", safeio.Hash([]byte("program")), safeio.Hash([]byte("program")))))
	return previous, snapshot
}

func TestGitSourceMetadataCannotOverrideArchiveMembers(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, damage := range []string{"", "commit", "path", "duplicate", "late", "missing", "runtime"} {
		t.Run(damage, func(t *testing.T) {
			raw := gitArchiveFixture(t, commit, damage)
			wanted := commit
			if damage == "runtime" {
				wanted = ""
			}
			files, err := tarHashes(bytes.NewReader(raw), "", wanted)
			if (err == nil) != (damage == "") {
				t.Fatalf("metadata policy mismatch: %s: %v", damage, err)
			}
			if err == nil && (len(files) != 1 || files["main.go"] != safeio.Hash([]byte("source"))) {
				t.Fatalf("source inventory changed: %v", files)
			}
		})
	}
}

func gitArchiveFixture(t *testing.T, commit, damage string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := tar.NewWriter(&buffer)
	header := &tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": commit}}
	if damage == "commit" {
		header.PAXRecords["comment"] = strings.Repeat("b", 40)
	}
	if damage == "path" {
		header.PAXRecords["path"] = "../outside"
	}
	if damage != "missing" && damage != "late" {
		require(t, archive.WriteHeader(header))
	}
	if damage == "duplicate" {
		require(t, archive.WriteHeader(header))
	}
	require(t, archive.WriteHeader(&tar.Header{Name: "main.go", Mode: 0600, Size: 6}))
	_, err := archive.Write([]byte("source"))
	require(t, err)
	if damage == "late" {
		require(t, archive.WriteHeader(header))
	}
	require(t, archive.Close())
	return buffer.Bytes()
}

func TestSourcePreparationRefusesUnpinnedInputsAndExistingOutputs(t *testing.T) {
	for _, damage := range []string{"", "changed", "existing"} {
		t.Run(damage, func(t *testing.T) {
			previous, snapshot := previousFixture(t)
			raw, err := safeio.Read(filepath.Join(previous, "source.tar"), 1<<20)
			require(t, err)
			if damage == "changed" {
				raw = []byte("untrusted source")
			}
			write(t, filepath.Join(snapshot, "previous-source.tar"), raw)
			target := filepath.Join(t.TempDir(), "copied.tar")
			if damage == "existing" {
				write(t, target, []byte("retained"))
			}
			script, err := filepath.Abs("../../../../../ops/testing/acceptance/installer/source.sh")
			require(t, err)
			command := exec.Command("bash", "-euo", "pipefail", "-c", `source "$1"; installer_previous_archive "$2" "$3"`, "source-test", script, snapshot, target)
			output, err := command.CombinedOutput()
			if (err == nil) != (damage == "") {
				t.Fatalf("archive preparation mismatch: %v %s", err, output)
			}
			if damage == "existing" {
				require(t, textFile(target, "retained"))
			}
		})
	}
}
