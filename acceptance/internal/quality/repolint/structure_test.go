package repolint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/quality/repolint"
)

func TestTwelveEntriesBoundaryIncludesFilesAndDirectories(t *testing.T) {
	for _, count := range []int{12, 13} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			files := map[string]string{"docs/.hidden/config.yml": "hidden: true\n"}
			for index := 1; index < count; index++ {
				files[fmt.Sprintf("docs/topic-%d.md", index)] = "document\n"
			}
			root := fixture(t, files)
			findings, err := repolint.Directories(root)
			if err != nil || (len(findings) != 0) != (count > 12) {
				t.Fatalf("count %d: %v, %v", count, findings, err)
			}
		})
	}
}

func TestOnlyRootBuildAndGitAreExcluded(t *testing.T) {
	for _, directory := range []string{"build", ".git", ".github", "docs/build"} {
		files := map[string]string{}
		for index := range 13 {
			files[fmt.Sprintf("%s/item-%d.md", directory, index)] = "text\n"
		}
		root := fixture(t, files)
		findings, err := repolint.Directories(root)
		wantFailure := directory == ".github" || directory == "docs/build"
		if err != nil || (len(findings) != 0) != wantFailure {
			t.Fatalf("%s: %v, %v", directory, findings, err)
		}
	}
}

func TestRootEntriesAreChecked(t *testing.T) {
	files := map[string]string{}
	for index := range 12 {
		files[fmt.Sprintf("root-%d.md", index)] = "text\n"
	}
	findings, err := repolint.Directories(fixture(t, files))
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].String(), "13 immediate entries") {
		t.Fatalf("root limit bypassed: %v, %v", findings, err)
	}
}

func TestTaskCatalogRejectsOrphanAndMissingImport(t *testing.T) {
	root := fixture(t, map[string]string{"justfile": "import 'ops/tasks/dev.just'\n",
		"ops/tasks/dev.just": "test:\n    go test ./...\n", "ops/tasks/lost.just": "orphan:\n    true\n"})
	findings, err := repolint.Tasks(root)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].String(), "unreachable") {
		t.Fatalf("orphan escaped: %v, %v", findings, err)
	}
	if err := os.WriteFile(filepath.Join(root, "justfile"), []byte("import 'ops/tasks/missing.just'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	findings, err = repolint.Tasks(root)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].String(), "missing.just") {
		t.Fatalf("missing import escaped: %v, %v", findings, err)
	}
}

func TestTaskCatalogFollowsNestedImports(t *testing.T) {
	root := fixture(t, map[string]string{"justfile": "import 'ops/tasks/dev.just'\n",
		"ops/tasks/dev.just": "import 'testing/unit.just'\n", "ops/tasks/testing/unit.just": "test:\n    go test ./...\n"})
	findings, err := repolint.Tasks(root)
	if err != nil || len(findings) != 0 {
		t.Fatalf("valid catalog rejected: %v, %v", findings, err)
	}
}

func TestPackageLoaderDoesNotCompileGeneratedToolchains(t *testing.T) {
	root := fixture(t, map[string]string{"source.go": "package sample\nfunc Valid() {}\n",
		"build/toolchains/go/test/invalid.go": "this is deliberately not Go\n"})
	findings, err := dependencies(t, root, "linux/amd64")
	if err != nil || len(findings) != 0 {
		t.Fatalf("generated workspace leaked into packages: %v, %v", findings, err)
	}
}

func TestTaskImportCycleRejected(t *testing.T) {
	root := fixture(t, map[string]string{"justfile": "import 'ops/tasks/a.just'\n",
		"ops/tasks/a.just": "import 'b.just'\n", "ops/tasks/b.just": "import 'a.just'\n"})
	findings, err := repolint.Tasks(root)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].String(), "dependency cycle") {
		t.Fatalf("task import cycle escaped: %v, %v", findings, err)
	}
}
