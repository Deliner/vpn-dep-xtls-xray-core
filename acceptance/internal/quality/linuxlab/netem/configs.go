package netem

import (
	"fmt"
	"path/filepath"
	"reflect"
)

func compareConfigs(first, second experiment) error {
	for _, role := range []string{"client", "server"} {
		a, err := normalizedConfig(filepath.Join(first.root, "reports", role+".json"), first.policy)
		if err != nil {
			return err
		}
		b, err := normalizedConfig(filepath.Join(second.root, "reports", role+".json"), second.policy)
		if err != nil || !reflect.DeepEqual(a, b) {
			return fmt.Errorf("%s config differs beyond congestion and test credentials: %v", role, err)
		}
	}
	return nil
}

func normalizedConfig(path, policy string) (map[string]any, error) {
	var config map[string]any
	if err := readJSON(path, &config); err != nil {
		return nil, err
	}
	node, stream, err := masqueStream(config)
	if err != nil {
		return nil, err
	}
	if policy == "reno" {
		want := map[string]any{"quicParams": map[string]any{"congestion": "reno"}}
		if !reflect.DeepEqual(stream["finalmask"], want) {
			return nil, fmt.Errorf("Reno config changes additional transport settings")
		}
		delete(stream, "finalmask")
	} else if _, exists := stream["finalmask"]; exists {
		return nil, fmt.Errorf("default control config has an override")
	}
	if err := normalizeCredentials(node, stream); err != nil {
		return nil, err
	}
	return config, nil
}

func masqueStream(config map[string]any) (map[string]any, map[string]any, error) {
	var found, stream map[string]any
	for _, key := range []string{"inbounds", "outbounds"} {
		nodes, ok := config[key].([]any)
		if !ok {
			return nil, nil, fmt.Errorf("missing %s config", key)
		}
		for _, value := range nodes {
			node, ok := value.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("invalid transport config")
			}
			if node["protocol"] != "masque" {
				continue
			}
			stream, ok = node["streamSettings"].(map[string]any)
			if found != nil || !ok {
				return nil, nil, fmt.Errorf("ambiguous MASQUE transport")
			}
			found = node
		}
	}
	if found == nil {
		return nil, nil, fmt.Errorf("missing MASQUE transport")
	}
	return found, stream, nil
}

func normalizeCredentials(node, stream map[string]any) error {
	masque, ok := stream["masqueSettings"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing MASQUE settings")
	}
	if _, ok := masque["pass"].(string); ok {
		masque["pass"] = "<synthetic-password>"
	}
	if settings, ok := node["settings"].(map[string]any); ok {
		if users, ok := settings["users"].([]any); ok {
			for _, value := range users {
				user, ok := value.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid MASQUE user")
				}
				user["pass"] = "<synthetic-password>"
			}
		}
	}
	tls, ok := stream["tlsSettings"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing TLS config")
	}
	certificates, ok := tls["certificates"].([]any)
	if !ok || len(certificates) != 1 {
		return fmt.Errorf("missing synthetic trust")
	}
	certificate, ok := certificates[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid synthetic certificate")
	}
	for _, field := range []string{"certificate", "key"} {
		if _, exists := certificate[field]; exists {
			certificate[field] = []any{"<synthetic-PEM>"}
		}
	}
	return nil
}

func matchingCredentials(root string) error {
	password, certificate, err := configCredentials(filepath.Join(root, "client.json"))
	if err != nil {
		return err
	}
	serverPassword, serverCertificate, err := configCredentials(filepath.Join(root, "server.json"))
	if err != nil || password != serverPassword || !reflect.DeepEqual(certificate, serverCertificate) {
		return fmt.Errorf("client and server credentials differ: %v", err)
	}
	return nil
}

func configCredentials(path string) (string, []string, error) {
	var config struct{ Inbounds, Outbounds []credentialNode }
	if err := readJSON(path, &config); err != nil {
		return "", nil, err
	}
	password, certificate := "", []string(nil)
	for _, node := range append(config.Inbounds, config.Outbounds...) {
		if node.Protocol != "masque" {
			continue
		}
		if certificate != nil || len(node.StreamSettings.TLSSettings.Certificates) != 1 {
			return "", nil, fmt.Errorf("ambiguous synthetic trust")
		}
		certificate = node.StreamSettings.TLSSettings.Certificates[0].Certificate
		password = node.StreamSettings.MasqueSettings.Pass
		if users := node.Settings.Users; len(users) != 0 {
			if len(users) != 1 || password != "" {
				return "", nil, fmt.Errorf("ambiguous synthetic password")
			}
			password = users[0].Pass
		}
	}
	if password == "" || len(certificate) == 0 {
		return "", nil, fmt.Errorf("missing synthetic credentials")
	}
	return password, certificate, nil
}
