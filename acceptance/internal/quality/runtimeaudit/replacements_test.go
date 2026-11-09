package runtimeaudit

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"sort"
	"time"

	"vpn-script/internal/platform/process"
)

func replacementQueries(info *debug.BuildInfo) ([]string, error) {
	unique := map[string]bool{}
	for _, dependency := range info.Deps {
		if dependency.Replace == nil || (dependency.Replace.Version != "" && dependency.Replace.Version != "(devel)") {
			continue
		}
		if dependency.Path == "" || dependency.Version == "" || dependency.Version == "(devel)" {
			return nil, fmt.Errorf("local replacement lacks a versioned upstream identity: %s", dependency.Path)
		}
		unique[dependency.Path+"@"+dependency.Version] = true
	}
	queries := make([]string, 0, len(unique))
	for query := range unique {
		queries = append(queries, query)
	}
	sort.Strings(queries)
	return queries, nil
}

func auditLocalReplacements(ctx context.Context, scanner, path string, report io.Writer) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	queries, err := replacementQueries(info)
	if err != nil || len(queries) == 0 {
		return err
	}
	return scanReplacementQueries(ctx, scanner, queries, report)
}

func scanReplacementQueries(ctx context.Context, scanner string, queries []string, report io.Writer) error {
	if _, err := fmt.Fprintln(report, "\ncoverage_limit=local replacements require conservative upstream module advisories; applicability needs review"); err != nil {
		return err
	}
	arguments := append([]string{scanner, "-mode=query", "-json"}, queries...)
	result, scanErr := process.Run(ctx, 2*time.Minute, arguments...)
	_, writeErr := report.Write(result.Output)
	var evidenceErr error
	if scanErr == nil {
		evidenceErr = verifyReplacementQuery(result.Output, queries)
	}
	err := errors.Join(scanErr, evidenceErr, writeErr)
	_, writeErr = fmt.Fprintf(report, "\nupstream_query_error=%v\n", err)
	return errors.Join(err, writeErr)
}
