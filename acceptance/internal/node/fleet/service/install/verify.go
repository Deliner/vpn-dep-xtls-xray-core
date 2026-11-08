package install

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

// The verified unit is the only mutable commit point. Reapplying after a crash
// always reloads systemd; no second current pointer can disagree with its bytes.
func (s *store) current() (string, []byte, error) {
	if _, err := os.Lstat(s.unit); os.IsNotExist(err) {
		return "", nil, nil
	}
	if err := s.access(s.unit, 0644); err != nil {
		return "", nil, err
	}
	raw, err := safeio.Read(s.unit, 65536)
	if err != nil {
		return "", nil, err
	}
	line, _, found := strings.Cut(string(raw), "\n")
	id, owned := strings.CutPrefix(line, marker+s.token+" ")
	if !found || !owned || !validate.Hash(id) {
		return "", nil, fmt.Errorf("refusing foreign Fleet service unit")
	}
	_, expected, err := s.revision(id)
	if err != nil || !bytes.Equal(raw, expected) {
		return "", nil, fmt.Errorf("Fleet unit or referenced generation differs")
	}
	return id, raw, nil
}
