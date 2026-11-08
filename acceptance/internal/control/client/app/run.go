package app

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/platform/safeio"
)

const usage = "nhop-client import --bundle DIR --route NAME --output PROFILE\nnhop-client plan --profile PROFILE --engine-uid UID --output NEW_DIR [--ipv6 tunnel|block]\nnhop-client start --profile PROFILE --runtimes INVENTORY [--ipv6 tunnel|block]\nnhop-client service-prepare\nnhop-client boot-protect\nnhop-client service-run --profile PROFILE --runtimes INVENTORY [--ipv6 tunnel|block]\nnhop-client service-plan --binary PATH --profile PROFILE --runtimes INVENTORY --output UNIT\nnhop-client service-install --profile PROFILE --runtimes INVENTORY [--ipv6 tunnel|block]\nnhop-client service-remove\nnhop-client service-enable [--network-manager UNIT]\nnhop-client service-disable\nnhop-client diagnostics\nnhop-client reconnect\nnhop-client resume\nnhop-client stop\nstart/reconnect/resume/boot-protect/service-prepare/service-run/service-install/service-remove/service-enable/service-disable/stop require Linux root; import/plan/service-plan never activate networking.\n"

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
	case "service-plan":
		return servicePlan(value, output)
	case "service-enable", "service-disable":
		return serviceBoot(args[0], value, output)
	case "service-install", "service-remove":
		return serviceInstall(args[0], value, output)
	case "start", "stop", "diagnostics", "reconnect", "resume", "service-run", "service-prepare", "boot-protect":
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
