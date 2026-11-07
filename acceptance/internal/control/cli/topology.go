package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/control/topology"
)

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func topologyAction(command string, options options, output io.Writer) error {
	definition, err := topology.Load(options.path)
	if err != nil {
		return err
	}
	switch command {
	case "validate":
		_, err = fmt.Fprintf(output, "Valid: %d nodes, %d services, %d routes\n", len(definition.Nodes), len(definition.Services), len(definition.Routes))
	case "plan":
		return writeJSON(output, bundle.Plan(definition))
	case "render":
		if err = bundle.Render(definition, options.output, options.installers); err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "Private deployment bundle: %s\n", options.output)
	}
	return err
}
