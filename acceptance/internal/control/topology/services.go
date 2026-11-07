package topology

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
	"vpn-script/internal/core/validate"
)

func service(value *model.Service, ssh model.SSH) error {
	if value == nil || validate.Port(value.Port) != nil {
		return fmt.Errorf("service requires a valid port")
	}
	spec, _ := protocol.Lookup(value.Protocol)
	if strings.Contains(spec.Network, "tcp") && (value.Port == ssh.Port || slices.Contains(ssh.ClosedPorts, value.Port)) {
		return fmt.Errorf("service port conflicts with SSH or closed_ports")
	}
	if value.Listen == "" {
		value.Listen = "0.0.0.0"
	}
	if _, err := netip.ParseAddr(value.Listen); err != nil {
		return fmt.Errorf("listen must be an IP literal")
	}
	return serviceOptions(value)
}

func serviceOptions(value *model.Service) error {
	spec, ok := protocol.Lookup(value.Protocol)
	if !ok {
		return fmt.Errorf("unknown protocol %q; use just protocols", value.Protocol)
	}
	if spec.Status != "implemented" {
		return fmt.Errorf("%s is %s: %s", spec.ID, spec.Status, spec.Requirement)
	}
	if spec.Donor {
		return donorOptions(value)
	}
	if value.Target != "" {
		return fmt.Errorf("target applies only to REALITY/ShadowTLS")
	}
	if !spec.TLS && (value.ServerName != "" || value.TLS != (model.TLSFiles{})) {
		return fmt.Errorf("TLS fields do not apply to %s", value.Protocol)
	}
	if spec.TLS && validate.Host(value.ServerName) != nil {
		return fmt.Errorf("TLS requires server_name")
	}
	if (value.TLS.Certificate == "") != (value.TLS.Key == "") {
		return fmt.Errorf("provide both certificate_file and key_file")
	}
	if value.TLS.CA != "" && value.TLS.Certificate == "" {
		return fmt.Errorf("ca_file requires certificate_file")
	}
	web := strings.HasSuffix(value.Protocol, "-ws") || value.Protocol == "vless-xhttp"
	if !web && value.Path != "" {
		return fmt.Errorf("path applies only to WS/XHTTP")
	}
	if web && value.Path == "" {
		value.Path = "/vpn"
	}
	if web && (!strings.HasPrefix(value.Path, "/") || strings.ContainsAny(value.Path, "?#\r\n")) {
		return fmt.Errorf("path must be an absolute URL path without query/fragment")
	}
	return nil
}

func reality(service *model.Service) error {
	if err := validate.Host(service.ServerName); err != nil {
		return fmt.Errorf("REALITY requires a valid server_name")
	}
	if service.Target == "" {
		service.Target = net.JoinHostPort(service.ServerName, "443")
	}
	host, number, err := net.SplitHostPort(service.Target)
	if err != nil || validate.Host(host) != nil {
		return fmt.Errorf("REALITY target must be host:port")
	}
	parsed, err := strconv.Atoi(number)
	if err != nil || validate.Port(parsed) != nil {
		return fmt.Errorf("invalid REALITY target port")
	}
	return nil
}

func collision(left, right model.Service) bool {
	if left.Port != right.Port {
		return false
	}
	aSpec, _ := protocol.Lookup(left.Protocol)
	bSpec, _ := protocol.Lookup(right.Protocol)
	for _, network := range strings.Split(aSpec.Network, ",") {
		if !slices.Contains(strings.Split(bSpec.Network, ","), network) {
			continue
		}
		a := model.Listener{Address: left.Listen, Port: left.Port, Network: network}
		b := model.Listener{Address: right.Listen, Port: right.Port, Network: network}
		if validate.SocketsOverlap(a, b) {
			return true
		}
	}
	return false
}

func donorOptions(service *model.Service) error {
	if service.Path != "" || service.TLS.Certificate != "" || service.TLS.Key != "" {
		return fmt.Errorf("donor profiles do not accept path or server TLS certificate/key")
	}
	if service.Protocol == model.Reality && service.TLS.CA != "" {
		return fmt.Errorf("REALITY does not use ca_file")
	}
	return reality(service)
}
