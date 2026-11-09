package runtimeaudit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

const queryConfiguration = `{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scan_mode":"query"}}` + "\n"
const queryProgress = `{"progress":{"message":"Looking up vulnerabilities in fixture.invalid/local at v1.2.3..."}}` + "\n"
const queryAdvisory = `{"osv":{"id":"GO-2099-0001"}}` + "\n"

func replacementBinary(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require(t, os.Mkdir(filepath.Join(root, "dep"), 0700))
	files := map[string]string{
		"go.mod":       "module fixture.invalid/main\n\ngo 1.27.3\n\nrequire fixture.invalid/local v1.2.3\nreplace fixture.invalid/local => ./dep\n",
		"main.go":      "package main\n\nimport (\n\"fmt\"\n\"fixture.invalid/local\"\n)\n\nfunc main() {\nfmt.Println(local.Value())\n}\n",
		"dep/go.mod":   "module fixture.invalid/local\n\ngo 1.27.3\n",
		"dep/value.go": "package local\n\nfunc Value() string {\nreturn \"local module fixture\"\n}\n",
	}
	for name, raw := range files {
		require(t, os.WriteFile(filepath.Join(root, name), []byte(raw), 0600))
	}
	binary := filepath.Join(root, "fixture")
	result, err := process.Run(context.Background(), time.Minute, "go", "-C", root, "build", "-trimpath", "-buildvcs=false", "-o", binary, ".")
	if err != nil {
		t.Fatalf("build replacement fixture: %v %s", err, result.Output)
	}
	return binary
}
