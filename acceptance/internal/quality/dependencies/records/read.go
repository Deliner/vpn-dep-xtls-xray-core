package records

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"vpn-script/internal/platform/safeio"
)

// Read hashes exactly the bytes decoded, so concurrent replacement cannot pair
// one inventory's values with another inventory's file hash.
func Read(name string, value any) (string, error) {
	path, err := recordPath(name)
	if err != nil {
		return "", err
	}
	raw, err := safeio.Read(path, 10_000_000)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return "", fmt.Errorf("invalid %s: %w", name, err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", fmt.Errorf("trailing JSON in %s", name)
	}
	return safeio.Hash(raw), nil
}
