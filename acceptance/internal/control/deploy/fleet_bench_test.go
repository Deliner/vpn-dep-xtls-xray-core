package deploy

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func BenchmarkDeployFleet(b *testing.B) {
	directory := fleetFixture(b, 24, 1)
	runner := func(ctx context.Context, _ time.Duration, args ...string) (process.Result, error) {
		return successfulRemote(ctx, 2*time.Millisecond, args...)
	}
	for _, jobs := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("jobs=%d", jobs), func(b *testing.B) {
			for b.Loop() {
				if err := RunConcurrent(context.Background(), directory, runner, io.Discard, jobs); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
