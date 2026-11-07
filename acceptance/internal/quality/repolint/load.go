package repolint

import (
	"context"
	"fmt"
	"os"
	"strings"
	"vpn-script/internal/quality/diagnostics"

	"golang.org/x/tools/go/packages"
	"vpn-script/internal/quality/repository"
)

func Dependencies(ctx context.Context, root, target string) ([]diagnostics.Finding, error) {
	platform := strings.Split(target, "/")
	if len(platform) != 2 {
		return nil, fmt.Errorf("invalid platform %q", target)
	}
	config := &packages.Config{Context: ctx, Dir: root, Tests: true,
		Mode: packages.LoadSyntax | packages.NeedDeps | packages.NeedModule,
		Env:  append(os.Environ(), "GOOS="+platform[0], "GOARCH="+platform[1], "CGO_ENABLED=0")}
	patterns, err := repository.Packages(root)
	if err != nil || len(patterns) == 0 {
		return nil, err
	}
	loaded, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, err
	}
	if problems := packageErrors(loaded, target); len(problems) != 0 {
		return problems, nil
	}
	var findings []diagnostics.Finding
	for _, pkg := range loaded {
		if pkg.TypesInfo == nil {
			continue
		}
		if _, err := fileGraph(root, pkg).Order(); err != nil {
			findings = append(findings, diagnostics.Issue(diagnostics.FileDependencies, "%s %s: file %v", target, pkg.ID, err))
		}
	}
	return append(findings, checkDirectories(importGraph(root, loaded))...), nil
}
