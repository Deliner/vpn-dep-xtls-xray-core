package repolint

import (
	"slices"
	"strings"
	"vpn-script/internal/quality/diagnostics"

	"golang.org/x/tools/go/packages"
)

func packageErrors(loaded []*packages.Package, target string) []diagnostics.Finding {
	seen := map[string]bool{}
	var findings []diagnostics.Finding
	packages.Visit(loaded, nil, func(pkg *packages.Package) {
		for _, problem := range pkg.Errors {
			message := problem.Error()
			if seen[message] {
				continue
			}
			seen[message] = true
			rule := diagnostics.StaticAnalysis
			if strings.Contains(message, "import cycle") {
				rule = diagnostics.PackageDependencies
			}
			findings = append(findings, diagnostics.Issue(rule, "%s: %s", target, message))
		}
	})
	slices.SortFunc(findings, func(a, b diagnostics.Finding) int { return strings.Compare(a.String(), b.String()) })
	return findings
}
