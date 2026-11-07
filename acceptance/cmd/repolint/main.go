package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"vpn-script/internal/quality/diagnostics"
)

func run() error {
	root := flag.String("root", ".", "module root")
	targets := flag.String("platforms", "linux/amd64,linux/arm64,darwin/arm64,darwin/amd64", "supported build targets")
	explain := flag.String("explain", "", "print a rule's repair skill without running checks")
	flag.Parse()
	absolute, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	if *explain != "" {
		return diagnostics.Explain(absolute, diagnostics.Rule(*explain), os.Stdout)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	findings := inspect(ctx, absolute, *targets)
	if err := diagnostics.Report(absolute, findings, os.Stderr); err != nil {
		return err
	}
	if len(findings) != 0 {
		return fmt.Errorf("%d violations; read the repair skills above, then run just check", len(findings))
	}
	fmt.Println("Lint passed: <=40 lines/function, <=300 lines/Go file, <=12 entries/directory, gofmt, acyclic dependencies and complete task catalog.")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
