package pull

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/fleetapi"
)

// Run keeps the installed state during controller/network failures. Revocation
// ends polling; removing an installed VPN requires a separate explicit action.
func (r *Runner) Run(ctx context.Context, interval time.Duration, observe func(wire.Status, error)) error {
	if interval < time.Second || interval > time.Hour {
		return fmt.Errorf("Fleet polling interval must be between one second and one hour")
	}
	failures := 0
	for {
		status, err := r.Step(ctx)
		if observe != nil {
			observe(status, err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if terminal(err) {
			return err
		}
		if err != nil {
			failures = min(failures+1, 8)
		} else {
			failures = 0
		}
		timer := time.NewTimer(pollDelay(interval, failures))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func terminal(err error) bool {
	var response *fleetapi.HTTPError
	return errors.Is(err, ErrClosed) || errors.As(err, &response) && (response.Status == http.StatusUnauthorized || response.Status == http.StatusForbidden)
}

func pollDelay(interval time.Duration, failures int) time.Duration {
	delay := min(interval*time.Duration(1<<failures), max(interval, 15*time.Minute))
	spread := delay / 5
	return delay - spread + time.Duration(rand.Int64N(int64(2*spread)+1))
}
