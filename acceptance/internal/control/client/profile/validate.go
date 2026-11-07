package profile

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	"vpn-script/internal/control/client"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func Validate(value Imported) error {
	m := value.Metadata
	if value.SchemaVersion != 1 || validate.Name(value.Topology) != nil || validate.Name(value.Route) != nil {
		return fmt.Errorf("invalid imported profile identity")
	}
	if m.HopCount < 1 || m.HopCount != len(m.Path) || !strings.HasPrefix(m.Path[0], m.EntryNode+".") || !strings.HasPrefix(m.Path[len(m.Path)-1], m.ExitNode+".") {
		return fmt.Errorf("invalid imported route path")
	}
	if err := validateEndpoint(value); err != nil {
		return err
	}
	hash, err := safeio.Digest(value.Config)
	if err != nil || !validate.Hash(m.ConfigSHA256) || hash != m.ConfigSHA256 {
		return fmt.Errorf("client configuration hash mismatch")
	}
	engine, err := client.Detect(value.Config)
	if err != nil || engine != m.Engine || m.RuntimeVersion != engines.Runtime(engine).Version {
		return fmt.Errorf("client runtime metadata mismatch")
	}
	return validateSOCKS(value)
}

func validateEndpoint(value Imported) error {
	e := value.Metadata.EntryEndpoint
	address, err := netip.ParseAddr(e.Address)
	if err != nil || address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() || address.Is4In6() {
		return fmt.Errorf("Linux client requires an explicit entry IP; render a profile with entry_endpoint metadata")
	}
	if e.Port < 1 || e.Port > 65535 || value.Metadata.SOCKSPort < 1024 || value.Metadata.SOCKSPort > 65535 {
		return fmt.Errorf("invalid entry or unprivileged SOCKS port")
	}
	switch e.Network {
	case "tcp", "udp", "tcp,udp":
		return nil
	default:
		return fmt.Errorf("invalid entry transport network")
	}
}

func validateSOCKS(value Imported) error {
	inbounds, _ := value.Config["inbounds"].([]any)
	if len(inbounds) != 1 {
		return fmt.Errorf("expected one loopback SOCKS inbound")
	}
	inbound, _ := inbounds[0].(map[string]any)
	portField, kindField := "port", "protocol"
	if value.Metadata.Engine == "sing-box" {
		portField, kindField = "listen_port", "type"
	}
	port, _ := inbound[portField].(json.Number)
	number, err := port.Int64()
	if err != nil || number != int64(value.Metadata.SOCKSPort) || inbound[kindField] != "socks" || inbound["listen"] != "127.0.0.1" {
		return fmt.Errorf("client SOCKS listener differs from private route metadata")
	}
	return nil
}
