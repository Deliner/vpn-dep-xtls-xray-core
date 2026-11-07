package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

func Inventory(ctx context.Context, run commands.Runner) (Report, error) {
	report := Report{Schema: 1}
	for _, root := range Roots() {
		scope, err := collectScope(ctx, run, root)
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", root.Name, err)
		}
		report.Scopes = append(report.Scopes, scope)
	}
	if err := records.Save("inventory.json", report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func collectScope(ctx context.Context, run commands.Runner, scope Scope) (Scope, error) {
	manifest := filepath.Join(scope.Directory, "go.mod")
	sums := filepath.Join(scope.Directory, "go.sum")
	before, err := safeio.FileHash(manifest)
	if err != nil {
		return scope, err
	}
	beforeSums, err := safeio.FileHash(sums)
	if err != nil {
		return scope, err
	}
	scope.Modules, err = scopeModules(ctx, run, scope.Directory)
	if err != nil {
		return scope, err
	}
	scope.ManifestSHA, err = safeio.FileHash(manifest)
	if err != nil {
		return scope, err
	}
	scope.SumsSHA, err = safeio.FileHash(sums)
	if err == nil && (before != scope.ManifestSHA || beforeSums != scope.SumsSHA) {
		err = fmt.Errorf("module locks changed while collecting inventory")
	}
	return scope, err
}

func scopeModules(ctx context.Context, run commands.Runner, directory string) ([]Module, error) {
	raw, err := run(ctx, "go", "-C", directory, "list", "-mod=readonly", "-m", "-json", "all")
	if err != nil {
		return nil, err
	}
	modules, err := decodeModules(raw)
	if err != nil {
		return nil, err
	}
	if err := resolveOrigins(ctx, run, directory, modules); err != nil {
		return nil, err
	}
	return modules, nil
}

func decodeModules(raw []byte) ([]Module, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var modules []Module
	seen := map[string]bool{}
	for {
		var module Module
		if err := decoder.Decode(&module); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if module.Error != nil || module.Path == "" || seen[module.Path] {
			return nil, fmt.Errorf("invalid or duplicate module %q: %v", module.Path, module.Error)
		}
		if !module.Main && (module.Version == "" || module.Replace != nil && module.Replace.Version == "") {
			return nil, fmt.Errorf("unversioned dependency %s", module.Path)
		}
		seen[module.Path] = true
		modules = append(modules, module)
	}
	if len(modules) == 0 || !modules[0].Main {
		return nil, fmt.Errorf("module inventory has no main module")
	}
	return modules, nil
}
