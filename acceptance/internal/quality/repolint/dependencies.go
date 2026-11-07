package repolint

import (
	"go/token"
	"path/filepath"
	"strings"
	"vpn-script/internal/quality/diagnostics"
	"vpn-script/internal/quality/repository"

	"golang.org/x/tools/go/packages"
	"vpn-script/internal/core/dag"
)

func localFile(root string, position token.Position) string {
	if position.Filename == "" {
		return ""
	}
	relative, err := filepath.Rel(root, position.Filename)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	if repository.Excluded(root, position.Filename) {
		return ""
	}
	return filepath.ToSlash(relative)
}

func fileGraph(root string, pkg *packages.Package) dag.Graph {
	graph := dag.Graph{}
	for identifier, object := range pkg.TypesInfo.Uses {
		if object.Pkg() != pkg.Types {
			continue
		}
		from := localFile(root, pkg.Fset.PositionFor(identifier.Pos(), false))
		to := localFile(root, pkg.Fset.PositionFor(object.Pos(), false))
		if from != "" && to != "" && from != to {
			graph.Add(from, to)
		}
	}
	return graph
}

func packageDirectory(root string, pkg *packages.Package) string {
	for _, file := range pkg.GoFiles {
		if relative := localFile(root, token.Position{Filename: file}); relative != "" {
			return filepath.ToSlash(filepath.Dir(relative))
		}
	}
	return ""
}

func importGraph(root string, loaded []*packages.Package) dag.Graph {
	graph := dag.Graph{}
	for _, pkg := range loaded {
		from := packageDirectory(root, pkg)
		if from == "" {
			continue
		}
		for _, imported := range pkg.Imports {
			to := packageDirectory(root, imported)
			if to != "" && from != to {
				graph.Add(from, to)
			}
		}
	}
	return graph
}

func checkDirectories(graph dag.Graph) []diagnostics.Finding {
	var findings []diagnostics.Finding
	for depth := 1; ; depth++ {
		grouped, deeper := directoryLevel(graph, depth)
		if _, err := grouped.Order(); err != nil {
			findings = append(findings, diagnostics.Issue(diagnostics.DirectoryDependencies, "directory level %d: %v", depth, err))
		}
		if !deeper {
			return findings
		}
	}
}

func directoryLevel(graph dag.Graph, depth int) (dag.Graph, bool) {
	grouped, deeper := dag.Graph{}, false
	for from, destinations := range graph {
		parts := strings.Split(from, "/")
		deeper = deeper || len(parts) > depth
		left := strings.Join(parts[:min(len(parts), depth)], "/")
		for to := range destinations {
			parts = strings.Split(to, "/")
			right := strings.Join(parts[:min(len(parts), depth)], "/")
			if left != right {
				grouped.Add(left, right)
			}
		}
	}
	return grouped, deeper
}
