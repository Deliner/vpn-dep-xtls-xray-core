package repolint

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"vpn-script/internal/quality/diagnostics"
	"vpn-script/internal/quality/repository"
)

const MaxFunctionLines = 40
const MaxFileLines = 300

func Source(root string) ([]diagnostics.Finding, error) {
	var findings []diagnostics.Finding
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && repository.Excluded(root, path) && path != root {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") {
			items, readErr := checkSource(path)
			findings = append(findings, items...)
			return readErr
		}
		return nil
	})
	return findings, err
}

func checkSource(path string) ([]diagnostics.Finding, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, raw, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	formatted, err := format.Source(raw)
	if err != nil {
		return nil, err
	}
	findings := fileLength(path, raw)
	if !bytes.Equal(raw, formatted) {
		findings = append(findings, diagnostics.Issue(diagnostics.Formatting, "%s: run just fmt", path))
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			start, end := fset.PositionFor(node.Pos(), false), fset.PositionFor(node.End(), false)
			if lines := end.Line - start.Line + 1; lines > MaxFunctionLines {
				findings = append(findings, diagnostics.Issue(diagnostics.FunctionSize, "%s: function has %d lines (maximum 40)", start, lines))
			}
		}
		return true
	})
	return findings, nil
}

func fileLength(path string, raw []byte) []diagnostics.Finding {
	lines := bytes.Count(raw, []byte{'\n'})
	if len(raw) != 0 && raw[len(raw)-1] != '\n' {
		lines++
	}
	if lines > MaxFileLines {
		return []diagnostics.Finding{diagnostics.Issue(diagnostics.FileSize, "%s: file has %d lines (maximum 300)", path, lines)}
	}
	return nil
}
