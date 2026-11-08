package app

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"

	"vpn-script/internal/control/fleet/registry"
	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
)

func initialize(o options, output io.Writer) error {
	if err := newOutput(o.state); err != nil {
		return err
	}
	if err := registry.Initialize(o.state, o.fleet); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]string{"fleet": o.fleet, "state": o.state})
}

func assignment(o options, output io.Writer) error {
	if o.validity < 1 || o.validity > contract.MaxValiditySeconds || o.expected == math.MaxUint64 {
		return fmt.Errorf("invalid assignment validity or expected revision")
	}
	if err := newOutput(o.output); err != nil {
		return err
	}
	if err := protectedParent(o.manifest); err != nil {
		return err
	}
	if err := safeio.CheckAccess(o.manifest, 0600, os.Geteuid(), -1); err != nil {
		return err
	}
	raw, err := safeio.Read(o.manifest, contract.MaxManifestSize)
	if err != nil {
		return err
	}
	data, err := manifest.Decode(raw)
	if err != nil {
		return err
	}
	nonce, err := credential.NewToken()
	if err != nil {
		return err
	}
	request := wire.Publish{RequestID: credential.TokenHash(nonce), ExpectedRevision: o.expected, ValiditySeconds: o.validity, Manifest: raw}
	if err := safeio.NewJSON(o.output, request); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]string{"node": data.NodeID, "request": o.output, "request_id": request.RequestID})
}
