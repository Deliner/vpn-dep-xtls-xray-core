package main

import (
	"context"
	"strings"
	"vpn-script/internal/quality/diagnostics"

	"vpn-script/internal/quality/repolint"
)

func inspect(ctx context.Context, root, targets string) []diagnostics.Finding {
	var findings []diagnostics.Finding
	for _, check := range []func(string) ([]diagnostics.Finding, error){repolint.Directories, repolint.Source, repolint.Tasks} {
		items, err := check(root)
		findings = append(findings, items...)
		if err != nil {
			findings = append(findings, diagnostics.Issue(diagnostics.StaticAnalysis, "%v", err))
		}
	}
	for _, target := range strings.Split(targets, ",") {
		items, err := repolint.Dependencies(ctx, root, target)
		findings = append(findings, items...)
		if err != nil {
			findings = append(findings, diagnostics.Issue(diagnostics.StaticAnalysis, "%v", err))
		}
	}
	return findings
}
