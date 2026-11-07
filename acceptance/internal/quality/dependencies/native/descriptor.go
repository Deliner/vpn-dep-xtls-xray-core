package native

import (
	"fmt"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func parseDescriptor(path string, raw []byte) (Descriptor, error) {
	result := Descriptor{Path: path, SHA256: safeio.Hash(raw)}
	const divider = "-------------------- DEPENDENCY DIVIDER --------------------"
	for _, section := range strings.Split(string(raw), divider) {
		fields, err := componentFields(section)
		if err != nil {
			return result, fmt.Errorf("%s: %w", path, err)
		}
		result.Components = append(result.Components, fields)
	}
	return result, nil
}

func componentFields(section string) (map[string]string, error) {
	fields := map[string]string{}
	keys := map[string]bool{"Name": true, "URL": true, "Version": true, "Revision": true,
		"License": true, "License File": true, "CPEPrefix": true, "Security Critical": true, "Shipped": true}
	for _, line := range strings.Split(section, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || !keys[key] {
			continue
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, fmt.Errorf("multiple %s declarations without Chromium dependency divider", key)
		}
		fields[key] = strings.TrimSpace(value)
	}
	return fields, nil
}

func descriptorPaths(raw []byte) ([]string, error) {
	var result []string
	for _, path := range strings.Split(string(raw), "\x00") {
		if !strings.HasSuffix(path, "/README.chromium") {
			continue
		}
		for _, part := range strings.Split(path, "/") {
			if part == "" || part == "." || part == ".." {
				return nil, fmt.Errorf("unexpected tracked descriptor path %q", path)
			}
		}
		result = append(result, path)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("native source contains no Chromium dependency descriptors")
	}
	return result, nil
}
