package planning

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"

	"vpn-script/internal/platform/safeio"
)

func privateBytes(path string) ([]byte, error) {
	if err := safeio.ProtectedParent(path, os.Geteuid()); err != nil {
		return nil, err
	}
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return nil, err
	}
	return safeio.Read(path, 10_000_000)
}

func readDraft(path string) (draft, error) {
	raw, err := privateBytes(path)
	if err != nil {
		return draft{}, err
	}
	var tree yaml.Node
	if err := yaml.Unmarshal(raw, &tree); err != nil {
		return draft{}, fmt.Errorf("invalid rollout YAML")
	}
	if err := plainYAML(&tree); err != nil {
		return draft{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var value draft
	if err := decoder.Decode(&value); err != nil || decoder.Decode(new(any)) != io.EOF {
		return draft{}, fmt.Errorf("rollout YAML contains unknown, invalid or trailing data")
	}
	return value, nil
}

func plainYAML(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Anchor != "" || node.Tag == "!!null" || node.Tag == "!!merge" {
		return fmt.Errorf("rollout YAML cannot contain aliases, anchors, nulls or merges")
	}
	for _, child := range node.Content {
		if err := plainYAML(child); err != nil {
			return err
		}
	}
	return nil
}
