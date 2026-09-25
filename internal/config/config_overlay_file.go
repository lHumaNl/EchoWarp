package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

const maxOverlayFileSize = 1 << 20

func readOverlayValues(path string) (map[string]yaml.Node, error) {
	if path == "" {
		return nil, nil
	}
	data, err := readBoundedConfig(path)
	if err != nil {
		return nil, err
	}
	node, err := parseOverlayDocument(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: invalid YAML/JSON document", path)
	}
	values, err := flattenOverlayDocument(node)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: invalid configuration mapping", path)
	}
	return values, nil
}

func readBoundedConfig(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	defer file.Close() //nolint:errcheck // Read-only file; read errors are handled below.
	data, err := io.ReadAll(io.LimitReader(file, maxOverlayFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if len(data) > maxOverlayFileSize {
		return nil, fmt.Errorf("config file too large (max %d bytes)", maxOverlayFileSize)
	}
	return data, nil
}

func parseOverlayDocument(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
		}
		return nil, errOverlayType
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errOverlayType
	}
	return document.Content[0], nil
}

func flattenOverlayDocument(node *yaml.Node) (map[string]yaml.Node, error) {
	values := make(map[string]yaml.Node)
	if err := node.Decode(&values); err != nil {
		return nil, err
	}
	for _, section := range []string{"audio", "opus", "network", "security", "security.tls", "connection", "logging", "performance"} {
		if child, ok := values[section]; ok {
			if err := flattenOverlaySection(values, section, &child); err != nil {
				return nil, err
			}
		}
	}
	return values, nil
}

func flattenOverlaySection(values map[string]yaml.Node, section string, node *yaml.Node) error {
	var children map[string]yaml.Node
	if node.Tag == "!!null" {
		return errOverlayType
	}
	if err := node.Decode(&children); err != nil {
		return err
	}
	for key, child := range children {
		values[section+"."+key] = child
	}
	return nil
}
