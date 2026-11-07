package validate

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)
var userPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func Name(value string) error {
	if !namePattern.MatchString(value) {
		return fmt.Errorf("invalid lowercase identifier")
	}
	return nil
}

func Port(value int) error {
	if value < 1 || value > 65535 {
		return fmt.Errorf("port must be in 1..65535")
	}
	return nil
}

func User(value string) error {
	if !userPattern.MatchString(value) {
		return fmt.Errorf("invalid SSH user")
	}
	return nil
}

func Hash(value string) bool {
	return hashPattern.MatchString(value)
}

func Host(value string) error {
	if _, err := netip.ParseAddr(value); err == nil {
		return nil
	}
	if len(value) > 253 || !hostPattern.MatchString(value) {
		return fmt.Errorf("invalid IP address or hostname")
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("invalid hostname label")
		}
	}
	return nil
}
