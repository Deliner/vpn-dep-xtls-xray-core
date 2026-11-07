package topology

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"go.yaml.in/yaml/v3"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func Load(path string) (*model.Topology, error) {
	raw, err := safeio.Read(path, 1_000_000)
	if err != nil {
		return nil, err
	}
	var syntax yaml.Node
	if err := yaml.Unmarshal(raw, &syntax); err != nil {
		return nil, fmt.Errorf("invalid YAML")
	}
	if err := checkSyntax(&syntax); err != nil {
		return nil, err
	}
	var definition model.Definition
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&definition); err != nil {
		return nil, fmt.Errorf("invalid YAML fields, values or duplicate keys")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("expected exactly one YAML document")
	}
	return Build(definition)
}

func checkSyntax(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Tag == "!!null" {
		return fmt.Errorf("YAML aliases and null values are not supported")
	}
	if node.Kind == yaml.MappingNode {
		for index := 0; index < len(node.Content); index += 2 {
			key, value := node.Content[index], node.Content[index+1]
			if key.Tag != "!!str" {
				return fmt.Errorf("YAML keys must be strings")
			}
			if key.Value == "port" || key.Value == "socks_port" {
				number, err := strconv.Atoi(value.Value)
				if value.Tag != "!!int" || err != nil || validate.Port(number) != nil {
					return fmt.Errorf("invalid YAML port on line %d", value.Line)
				}
			}
		}
	}
	for _, child := range node.Content {
		if err := checkSyntax(child); err != nil {
			return err
		}
	}
	return nil
}

func Build(definition model.Definition) (*model.Topology, error) {
	if definition.Version != 1 || validate.Name(definition.Name) != nil {
		return nil, fmt.Errorf("expected topology version 1 and a valid name")
	}
	topology := &model.Topology{Definition: definition, Services: map[string]model.Endpoint{}, Successors: map[string]string{}}
	if err := nodes(topology); err != nil {
		return nil, err
	}
	if err := routes(topology); err != nil {
		return nil, err
	}
	graph := dag.Graph{}
	for node := range topology.Nodes {
		graph.Add(node, "")
	}
	for from, to := range topology.Successors {
		if to == "" {
			continue
		}
		left, right := topology.Services[from].Node, topology.Services[to].Node
		if left == right {
			return nil, fmt.Errorf("consecutive hops must be on different nodes")
		}
		graph.Add(left, right)
	}
	var err error
	topology.Order, err = graph.Order()
	return topology, err
}
