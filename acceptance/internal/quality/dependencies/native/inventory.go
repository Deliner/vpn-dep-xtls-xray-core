package native

import (
	"context"
	"path/filepath"
	"strings"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/checkout"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

func Inventory(ctx context.Context, run commands.Runner, root, revision string) (Report, error) {
	if err := checkout.Verify(ctx, run, root, revision); err != nil {
		return Report{}, err
	}
	identity, err := checkout.ReadIdentity(ctx, run, root, revision)
	if err != nil {
		return Report{}, err
	}
	report := Report{Schema: 1, Repository: identity.Repository, Revision: revision, SourceTree: identity.SourceTree}
	version, err := safeio.Read(filepath.Join(root, "CHROMIUM_VERSION"), 256)
	if err != nil {
		return report, err
	}
	report.Chromium = strings.TrimSpace(string(version))
	report.Descriptors, err = collectDescriptors(ctx, run, root)
	if err != nil {
		return report, err
	}
	if err := checkout.Verify(ctx, run, root, revision); err != nil {
		return report, err
	}
	return report, records.Save("native-inventory.json", report)
}

func collectDescriptors(ctx context.Context, run commands.Runner, root string) ([]Descriptor, error) {
	raw, err := run(ctx, "git", "-C", root, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	paths, err := descriptorPaths(raw)
	if err != nil {
		return nil, err
	}
	var descriptors []Descriptor
	for _, path := range paths {
		raw, err := safeio.Read(filepath.Join(root, path), 128*1024)
		if err != nil {
			return nil, err
		}
		descriptor, err := parseDescriptor(path, raw)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}
