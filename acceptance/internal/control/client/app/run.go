package app

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/platform/safeio"
)

const usage = "nhop-client import --bundle DIR --route NAME --output PROFILE\nnhop-client plan --profile PROFILE --engine-uid UID --output NEW_DIR [--ipv6 tunnel|block]\nThese commands prepare private files; they do not activate networking.\n"

func Run(args []string, output io.Writer) error {
	if len(args) == 0 || args[0] == "--help" {
		_, err := fmt.Fprint(output, usage)
		return err
	}
	value, err := parse(args[0], args[1:], output)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if args[0] == "plan" {
		return plan(value, output)
	}
	imported, err := profile.Import(value.bundle, value.route)
	if err != nil {
		return err
	}
	if err := safeio.NewJSON(value.output, imported); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Imported %s/%s (%s, %d hops)\n", imported.Topology, imported.Route, imported.Metadata.Engine, imported.Metadata.HopCount)
	return err
}
