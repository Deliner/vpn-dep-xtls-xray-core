package singrelease

import (
	"fmt"
	"strings"
	"vpn-script/internal/quality/runtimerelease"
)

func requireLines(raw []byte, lines []string) error {
	text := "\n" + string(raw)
	for _, line := range lines {
		key, _, _ := strings.Cut(line, "=")
		if strings.Count(text, "\n"+key+"=") != 1 || !strings.Contains(text, "\n"+line+"\n") {
			return fmt.Errorf("reviewed identity differs: %s", key)
		}
	}
	return nil
}

func exact(root, name, value string) error {
	raw, err := runtimerelease.ReadRegular(root, name, 4096)
	if err != nil {
		return err
	}
	if string(raw) != value {
		return fmt.Errorf("unexpected %s", name)
	}
	return nil
}
