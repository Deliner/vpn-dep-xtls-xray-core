package app

import (
	"fmt"
	"io"

	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/platform/safeio"
)

func servicePlan(value options, output io.Writer) error {
	data, err := systemd.Unit(systemd.Options{Executable: value.binary, Profile: value.profile, Runtimes: value.runtimes, Network: value.network})
	if err != nil {
		return err
	}
	if err := safeio.Create(value.output, data, 0600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Prepared systemd unit: %s; manual start only, no boot protection or installation performed\n", value.output)
	return err
}
