package modules

import (
	"fmt"

	"vpn-script/internal/platform/safeio"
)

func Roots() []Scope {
	return []Scope{{Name: "controller", Directory: "."}, {Name: "xray", Directory: "ops/runtime/xray"},
		{Name: "sing-box", Directory: "ops/runtime/sing-box"}}
}

func VerifyLocks(report Report) error {
	expected := map[string]string{}
	for _, scope := range Roots() {
		expected[scope.Name] = scope.Directory
	}
	if report.Schema != 1 || len(report.Scopes) != len(expected) {
		return fmt.Errorf("unexpected inventory schema or module roots")
	}
	for _, scope := range report.Scopes {
		directory, found := expected[scope.Name]
		if !found || scope.Directory != directory {
			return fmt.Errorf("unexpected or duplicate module root %s", scope.Name)
		}
		manifest, err := safeio.FileHash(directory + "/go.mod")
		if err != nil || manifest != scope.ManifestSHA {
			return fmt.Errorf("stale manifest inventory for %s: %v", scope.Name, err)
		}
		sums, err := safeio.FileHash(directory + "/go.sum")
		if err != nil || sums != scope.SumsSHA {
			return fmt.Errorf("stale checksum inventory for %s: %v", scope.Name, err)
		}
		delete(expected, scope.Name)
	}
	return nil
}
