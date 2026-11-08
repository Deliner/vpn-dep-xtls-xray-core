package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func Load(path string) (model.NodeConfig, error) {
	raw, err := safeio.Read(path, model.MaxNodeConfigSize)
	if err != nil {
		return model.NodeConfig{}, err
	}
	return Decode(raw)
}

// Decode applies the same manifest policy to verified in-memory Fleet payloads
// and files loaded by the existing SSH installer.
func Decode(raw []byte) (model.NodeConfig, error) {
	if len(raw) == 0 || len(raw) > model.MaxNodeConfigSize {
		return model.NodeConfig{}, fmt.Errorf("invalid node manifest size")
	}
	var data model.NodeConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return model.NodeConfig{}, fmt.Errorf("invalid node manifest JSON")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return model.NodeConfig{}, fmt.Errorf("unexpected trailing node manifest JSON")
	}
	if err := Validate(data); err != nil {
		return model.NodeConfig{}, err
	}
	return data, nil
}
