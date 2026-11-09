package runtimeaudit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"vpn-script/internal/platform/process"
)

func TestReplacementQueryRejectsFindingsAndIncompleteOutput(t *testing.T) {
	complete := queryConfiguration + queryProgress
	for name, raw := range map[string]string{
		"clean": complete, "advisory": complete + queryAdvisory, "missing-config": queryProgress,
		"missing-query": queryConfiguration, "empty": "", "truncated-json": complete + "{",
		"duplicate-config": queryConfiguration + complete, "duplicate-query": complete + queryProgress,
		"wrong-mode":      strings.Replace(complete, `"query"`, `"binary"`, 1),
		"wrong-module":    strings.Replace(complete, "fixture.invalid/local", "unknown.invalid/module", 1),
		"unknown-message": complete + `{"new_kind":{}}`, "null-advisory": complete + `{"osv":null}`,
		"output-limit": complete + strings.Repeat(" ", process.OutputLimit-len(complete)),
	} {
		t.Run(name, func(t *testing.T) {
			err := verifyReplacementQuery([]byte(raw), []string{"fixture.invalid/local@v1.2.3"})
			if (err == nil) != (name == "clean") {
				t.Fatal(name, err)
			}
		})
	}
}

func TestReplacementQueryRequiresEveryModuleAndKnownOriginalVersion(t *testing.T) {
	info := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "normal.invalid/module", Version: "v1.0.0"},
		{Path: "forked.invalid/module", Version: "v1.0.0", Replace: &debug.Module{Path: "owned.invalid/module", Version: "v1.1.0"}},
		{Path: "fixture.invalid/local", Version: "v1.2.3", Replace: &debug.Module{Path: "./dep"}},
	}}
	queries, err := replacementQueries(info)
	if err != nil || len(queries) != 1 || queries[0] != "fixture.invalid/local@v1.2.3" {
		t.Fatal(queries, err)
	}
	queries = append(queries, "second.invalid/module@v1.0.0")
	if err := verifyReplacementQuery([]byte(queryConfiguration+queryProgress), queries); err == nil {
		t.Fatal("partial lookup accepted")
	}
	info.Deps[2].Version = "(devel)"
	if _, err := replacementQueries(info); err == nil {
		t.Fatal("unversioned upstream accepted")
	}
}

func TestBinaryAuditIncludesLocalReplacementAdvisories(t *testing.T) {
	binary := replacementBinary(t)
	for _, finding := range []bool{false, true} {
		t.Run(fmt.Sprint(finding), func(t *testing.T) {
			directory := t.TempDir()
			scanner := filepath.Join(directory, "scanner")
			reply := queryConfiguration + queryProgress
			if finding {
				reply += queryAdvisory
			}
			script := "#!/bin/sh\nif [ \"$1 $2\" = '-mode binary' ]; then echo 'binary scan completed'; exit 0; fi\n" +
				"[ \"$1 $2 $3\" = '-mode=query -json fixture.invalid/local@v1.2.3' ] || exit 9\ncat <<'JSON'\n" + reply + "JSON\n"
			require(t, os.WriteFile(scanner, []byte(script), 0700))
			audit := runtimeAudit{scanner: scanner, directory: directory}
			err := audit.scan(binary, "sing-box", "aarch64")
			if (err == nil) == finding {
				t.Fatal("lost upstream advisory", err)
			}
			raw, err := os.ReadFile(audit.report("sing-box", "aarch64"))
			require(t, err)
			for _, marker := range []string{"binary scan completed", "upstream_query_error=", "fixture.invalid/local"} {
				if !strings.Contains(string(raw), marker) {
					t.Fatal("missing audit evidence", marker)
				}
			}
		})
	}
}

func TestReplacementScannerFailureIsPreserved(t *testing.T) {
	scanner := filepath.Join(t.TempDir(), "scanner")
	require(t, os.WriteFile(scanner, []byte("#!/bin/sh\necho 'query transport failed'\nexit 7\n"), 0700))
	var output bytes.Buffer
	err := scanReplacementQueries(context.Background(), scanner, []string{"fixture.invalid/local@v1.2.3"}, &output)
	var failure *exec.ExitError
	if !errors.As(err, &failure) || failure.ExitCode() != 7 || !strings.Contains(output.String(), "query transport failed") {
		t.Fatal("lost query failure", err, output.String())
	}
}
