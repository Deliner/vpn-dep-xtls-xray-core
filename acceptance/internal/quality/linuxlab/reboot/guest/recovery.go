package guest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

// Recover always attempts owned cleanup, including when acceptance fails.
// The final receipt is published only after this explicit recovery has finished.
func Recover(ctx context.Context, config contract.Config) error {
	observation, cancelObservation := context.WithTimeout(ctx, 45*time.Second)
	defer cancelObservation()
	phase, err := Phase(config)
	if err != nil {
		return err
	}
	boot, bootError := BootID()
	result := contract.Recovery{Phase: phase.Name, BeforeBootID: phase.BeforeBootID, BootID: boot, Status: "failed", Observations: map[string]string{}}
	err = bootError
	if err == nil && phase.Name != "baseline" && boot == phase.BeforeBootID {
		err = fmt.Errorf("guest kernel did not reboot")
	}
	if err == nil {
		err = Observe(observation, config, phase, result.Observations)
	}
	if err != nil {
		result.Error = err.Error()
	}
	cancelObservation()
	cleanup, cancelCleanup := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelCleanup()
	cleanupError := Restore(cleanup, config, phase)
	if cleanupError != nil {
		result.CleanupError = cleanupError.Error()
	}
	if err == nil && cleanupError == nil {
		result.Status = "passed"
	}
	writeError := safeio.JSON(contract.Root+"/recovery-"+phase.Name+".json", result)
	return errors.Join(err, cleanupError, writeError)
}
