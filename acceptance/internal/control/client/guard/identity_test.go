package guard

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func TestReleaseNeverDeletesAReplacedOrAmbiguousTable(t *testing.T) {
	lease := Lease{Token: "nhop:" + strings.Repeat("a", 32), Handle: 17}
	for _, table := range []string{
		`{"family":"inet","name":"nhop_client","handle":18,"comment":"` + lease.Token + `"}`,
		`{"family":"inet","name":"nhop_client","handle":17,"comment":"foreign"}`,
		`{"family":"ip","name":"nhop_client","handle":17,"comment":"` + lease.Token + `"}`,
	} {
		deleted := false
		manager := &Manager{run: func(ctx context.Context, timeout time.Duration, args ...string) (process.Result, error) {
			if strings.Contains(strings.Join(args, " "), "delete") {
				deleted = true
			}
			return process.Result{Success: true, Output: []byte(fmt.Sprintf(`{"nftables":[{"table":%s}]}`, table))}, nil
		}}
		if err := manager.Release(t.Context(), lease); err == nil || deleted {
			t.Fatal("changed table was accepted or a destructive command was attempted")
		}
	}
}
