package repolint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"vpn-script/internal/quality/diagnostics"

	"vpn-script/internal/core/dag"
)

var taskImport = regexp.MustCompile(`^import ['"]([^'"]+)['"](?:\s*#.*)?$`)

func taskImports(root, relative string, seen map[string]bool, graph dag.Graph) error {
	if seen[relative] {
		return nil
	}
	seen[relative] = true
	contents, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "import") {
			continue
		}
		match := taskImport.FindStringSubmatch(line)
		if len(match) != 2 {
			return fmt.Errorf("%s: use required literal imports for the task catalog", relative)
		}
		next := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(relative), match[1])))
		if !strings.HasPrefix(next, "ops/tasks/") || !strings.HasSuffix(next, ".just") {
			return fmt.Errorf("%s: task definitions must live under ops/tasks", next)
		}
		graph.Add(relative, next)
		if err := taskImports(root, next, seen, graph); err != nil {
			return err
		}
	}
	return nil
}

func Tasks(root string) ([]diagnostics.Finding, error) {
	seen := map[string]bool{}
	graph := dag.Graph{}
	if err := taskImports(root, "justfile", seen, graph); err != nil {
		return []diagnostics.Finding{diagnostics.Issue(diagnostics.TaskCatalog, "%v", err)}, nil
	}
	if _, err := graph.Order(); err != nil {
		return []diagnostics.Finding{diagnostics.Issue(diagnostics.TaskCatalog, "task import %v", err)}, nil
	}
	var findings []diagnostics.Finding
	err := filepath.WalkDir(filepath.Join(root, "ops", "tasks"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".just") {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if !seen[filepath.ToSlash(relative)] {
				findings = append(findings, diagnostics.Issue(diagnostics.TaskCatalog, "%s: task file is unreachable from root justfile", relative))
			}
		}
		return nil
	})
	return findings, err
}
