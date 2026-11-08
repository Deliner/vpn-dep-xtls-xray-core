package manager

import (
	"fmt"
	"strings"
)

func Parse(data []byte, properties string) (map[string]string, error) {
	if len(data) > 65536 {
		return nil, fmt.Errorf("systemd state exceeds size limit")
	}
	value := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, item, found := strings.Cut(line, "=")
		if _, duplicate := value[key]; !found || duplicate {
			return nil, fmt.Errorf("invalid or duplicate systemd property")
		}
		value[key] = item
	}
	for _, key := range strings.Split(properties, ",") {
		if _, found := value[key]; !found {
			return nil, fmt.Errorf("missing systemd property: %s", key)
		}
	}
	if len(value) != len(strings.Split(properties, ",")) {
		return nil, fmt.Errorf("unexpected systemd property")
	}
	return value, nil
}
