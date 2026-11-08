package app

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/platform/safeio"
)

const usage = "nhop-client import --bundle DIR --route NAME --output PROFILE\nnhop-client plan --profile PROFILE --engine-uid UID --output NEW_DIR [--ipv6 tunnel|block]\nnhop-client start --profile PROFILE --runtimes INVENTORY [--ipv6 tunnel|block]\nnhop-client diagnostics\nnhop-client reconnect\nnhop-client resume\nnhop-client stop\nstart/reconnect/resume/stop require Linux root; import/plan never activate networking.\n"

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
	switch args[0] {
	case "plan":
		return plan(value, output)
	case "start", "stop", "diagnostics", "reconnect", "resume":
		return control(args[0], value, output)
	}
	return importProfile(value, output)
}

func importProfile(value options, output io.Writer) error {
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
