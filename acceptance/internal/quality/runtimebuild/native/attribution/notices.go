package attribution

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

func notices(input io.Reader) ([]byte, error) {
	reader := tar.NewReader(input)
	files := map[string][]byte{}
	seen := map[string]bool{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(header.Name, "/")
		if !fs.ValidPath(name) || strings.Contains(name, "\\") || seen[name] {
			return nil, fmt.Errorf("unsafe or duplicate source path: %s", name)
		}
		seen[name] = true
		if header.Typeflag == tar.TypeDir || !noticeName(name) {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size > 2_000_000 {
			return nil, fmt.Errorf("invalid source notice: %s", name)
		}
		raw, err := io.ReadAll(io.LimitReader(reader, 2_000_001))
		if err != nil {
			return nil, err
		}
		files[name] = raw
	}
	if len(files["LICENSE"]) == 0 {
		return nil, fmt.Errorf("source archive lacks its root LICENSE")
	}
	if err := licenseReferences(files); err != nil {
		return nil, err
	}
	return render(files), nil
}

func noticeName(name string) bool {
	base := strings.ToUpper(path.Base(name))
	if base == "README.CHROMIUM" {
		return true
	}
	for _, prefix := range []string{"LICENSE", "COPYING", "NOTICE"} {
		if base == prefix || strings.HasPrefix(base, prefix+".") || strings.HasPrefix(base, prefix+"-") {
			return true
		}
	}
	return false
}

func licenseReferences(files map[string][]byte) error {
	for name, raw := range files {
		if path.Base(name) != "README.chromium" {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			ref, ok := strings.CutPrefix(line, "License File:")
			if !ok {
				continue
			}
			ref = strings.TrimSpace(ref)
			target := path.Join(path.Dir(name), ref)
			if strings.HasPrefix(ref, "//") {
				target = path.Join("src", strings.TrimPrefix(ref, "//"))
			}
			if len(files[target]) == 0 {
				return fmt.Errorf("missing referenced license %s from %s", target, name)
			}
		}
	}
	return nil
}

func render(files map[string][]byte) []byte {
	var output bytes.Buffer
	output.WriteString("Source-tree license and notice texts. Inclusion does not assert linkage.\nThe accompanying source.tar.gz preserves the complete reviewed native build input.\n\n")
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&output, "===== %s =====\n", name)
		output.Write(files[name])
		output.WriteString("\n\n")
	}
	return output.Bytes()
}
