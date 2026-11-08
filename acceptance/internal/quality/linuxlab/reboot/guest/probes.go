package guest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func Traffic(ctx context.Context, config contract.Config, expected string) (map[string]string, error) {
	observations := map[string]string{}
	address := config.PeerIP
	if expected == config.PeerIP {
		address = contract.OriginIP
	}
	httpPeer, err := httpProbe(ctx, config, address)
	if err != nil {
		return observations, err
	}
	observations["http_exit"] = httpPeer
	udpPeer, err := udpProbe(config, address)
	if err != nil {
		return observations, err
	}
	observations["udp_exit"] = udpPeer
	if httpPeer != expected || udpPeer != expected {
		return observations, fmt.Errorf("unexpected HTTP/UDP exit: %v", observations)
	}
	return observations, nil
}

func httpProbe(ctx context.Context, config contract.Config, address string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, "GET", "http://"+address+":18080/origin?token="+config.Token+"&payload=reboot", nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var origin contract.Origin
	err = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&origin)
	if err == nil && (response.StatusCode != 200 || origin.Payload != "reboot") {
		err = fmt.Errorf("HTTP origin did not echo the probe")
	}
	return origin.Peer, err
}

func udpProbe(config contract.Config, address string) (string, error) {
	connection, err := net.DialTimeout("udp4", net.JoinHostPort(address, "18081"), time.Second)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return "", err
	}
	payload := config.Token + ":reboot"
	if _, err := connection.Write([]byte(payload)); err != nil {
		return "", err
	}
	var buffer [4096]byte
	size, err := connection.Read(buffer[:])
	if err != nil {
		return "", err
	}
	var origin contract.Origin
	err = json.Unmarshal(buffer[:size], &origin)
	if err == nil && origin.Payload != payload {
		err = fmt.Errorf("UDP origin did not echo the probe")
	}
	return origin.Peer, err
}

func SystemDNS(parent context.Context, config contract.Config) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	name := fmt.Sprintf("boot-%d.nhop.test", time.Now().UnixNano())
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", name)
	if err != nil {
		return "", err
	}
	if len(addresses) != 1 || addresses[0].String() != config.PeerIP {
		return "", fmt.Errorf("unexpected system DNS response: %v", addresses)
	}
	return name + "=" + addresses[0].String(), nil
}
