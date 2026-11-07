package probe

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func localClient(ctx context.Context, binary, directory string, config model.Object) (*probeClient, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	if err := clientPort(config, port); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "client.json")
	if err := safeio.JSON(path, config); err != nil {
		return nil, err
	}
	log, err := os.OpenFile(filepath.Join(directory, "client.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	engine, _ := client.Detect(config)
	local, err := launchClient(ctx, binary, engine, path, port, log)
	if err != nil {
		return nil, err
	}
	if err := waitClient(local); err != nil {
		local.stop()
		return nil, err
	}
	return local, nil
}

func clientPort(config model.Object, port int) error {
	inbounds, _ := config["inbounds"].([]any)
	if len(inbounds) != 1 {
		return fmt.Errorf("expected one SOCKS inbound")
	}
	inbound, _ := inbounds[0].(map[string]any)
	if inbound == nil {
		return fmt.Errorf("invalid SOCKS inbound")
	}
	engine, err := client.Detect(config)
	if err != nil {
		return err
	}
	if engine == "xray" {
		inbound["port"] = port
	} else {
		inbound["listen_port"] = port
	}
	return nil
}

func loadClient(path string, profile model.Profile) (model.Object, string, error) {
	var config model.Object
	if err := safeio.LoadJSON(path, &config); err != nil {
		return nil, "", err
	}
	var value any = config
	if profile.Engine == "" {
		var legacy model.Xray
		if err := safeio.LoadJSON(path, &legacy); err != nil {
			return nil, "", err
		}
		value = legacy
	}
	hash, err := safeio.Digest(value)
	if err != nil || hash != profile.ConfigSHA256 {
		return nil, "", fmt.Errorf("client configuration hash mismatch")
	}
	engine, err := client.Detect(config)
	if err != nil {
		return nil, "", err
	}
	if profile.Engine != "" && (profile.Engine != engine || profile.RuntimeVersion != engines.Runtime(engine).Version) {
		return nil, "", fmt.Errorf("client runtime metadata mismatch")
	}
	inbounds, _ := config["inbounds"].([]any)
	if len(inbounds) != 1 {
		return nil, "", fmt.Errorf("invalid client inbounds")
	}
	return config, engine, nil
}
