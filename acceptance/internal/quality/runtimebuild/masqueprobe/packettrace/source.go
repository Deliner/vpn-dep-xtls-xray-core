// Package packettrace supplies the same diagnostic-only code to local contract
// tests and source-verified packet boundaries. It never logs payloads.
package packettrace

import (
	"bytes"
	"embed"
	"fmt"
)

//go:embed header.go trace.go header_test.go trace_test.go datagram.go datagram_test.go queue.go queue_test.go
var sources embed.FS

// Sources returns compilable helpers and controls under the target package name.
func Sources(target string) (map[string][]byte, error) {
	if target != "connectip" && target != "wireguard" && target != "masque" {
		return nil, fmt.Errorf("unreviewed diagnostic package: %s", target)
	}
	entries, err := sources.ReadDir(".")
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		content, err := sources.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		declaration := []byte("package packettrace\n")
		if bytes.Count(content, declaration) != 1 {
			return nil, fmt.Errorf("ambiguous diagnostic helper package: %s", entry.Name())
		}
		result[entry.Name()] = bytes.Replace(content, declaration, []byte("package "+target+"\n"), 1)
	}
	return result, nil
}
