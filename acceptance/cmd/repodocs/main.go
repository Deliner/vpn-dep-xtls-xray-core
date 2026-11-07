package main

import (
	"fmt"
	"os"

	"vpn-script/internal/quality/diagnostics"
	"vpn-script/internal/quality/documents"
)

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if len(os.Args) != 2 {
		return fmt.Errorf("use just docs-check or just research-inventory")
	}
	switch os.Args[1] {
	case "inventory":
		return documents.WriteInventory(root)
	case "check":
		findings, err := documents.Check(root)
		if err != nil {
			findings = append(findings, diagnostics.Issue(diagnostics.DocumentationLinks, "%v", err))
		}
		if err := diagnostics.Report(root, findings, os.Stderr); err != nil {
			return err
		}
		if len(findings) != 0 {
			return fmt.Errorf("%d documentation violations; use the repair skills shown with each error", len(findings))
		}
		fmt.Println("Documentation links and research inventory passed.")
		return nil
	default:
		return fmt.Errorf("unknown documentation command %q", os.Args[1])
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
