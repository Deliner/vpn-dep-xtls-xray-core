package repolint_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"vpn-script/internal/quality/diagnostics"

	"vpn-script/internal/quality/repolint"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = "module lintfixture\n\ngo 1.27.3\n"
	for path, contents := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func dependencies(t *testing.T, root, platform string) ([]diagnostics.Finding, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return repolint.Dependencies(ctx, root, platform)
}
