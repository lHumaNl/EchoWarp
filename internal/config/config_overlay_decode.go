package config

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var errOverlayType = errors.New("invalid configuration value type")

func decodeOverlayEnv(target reflect.Value, value string) error {
	if target.Kind() != reflect.Slice {
		return decodeOverlayField(target, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
	if value == "" {
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
		return nil
	}
	if target.Type().Elem().Kind() == reflect.String && !strings.HasPrefix(strings.TrimSpace(value), "[") {
		return decodeOverlayStringList(target, value)
	}
	node, err := parseOverlayDocument([]byte(value))
	if err != nil {
		return err
	}
	return decodeOverlayField(target, node)
}

func decodeOverlayStringList(target reflect.Value, value string) error {
	if value == "" {
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
		return nil
	}
	items := strings.Split(value, ",")
	for i := range items {
		items[i] = strings.TrimSpace(items[i])
	}
	target.Set(reflect.ValueOf(items))
	return nil
}

func decodeOverlayField(target reflect.Value, node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return decodeOverlayField(target, node.Alias)
	}
	if target.Kind() == reflect.Slice && target.Type().Elem() == reflect.TypeFor[DeviceEntry]() {
		return decodeOverlayDevices(target, node)
	}
	if node.Tag == "!!null" {
		target.SetZero()
		return nil
	}
	if target.Kind() == reflect.Slice {
		return decodeOverlaySlice(target, node)
	}
	return decodeOverlayScalar(target, node)
}

func decodeOverlayScalar(target reflect.Value, node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || (node.Tag == "!!float" && target.Kind() != reflect.String) {
		return errOverlayType
	}
	if target.Kind() == reflect.Pointer {
		target.Set(reflect.New(target.Type().Elem()))
		return decodeOverlayScalar(target.Elem(), node)
	}
	if node.Tag == "!!str" && target.Kind() != reflect.String {
		return decodeOverlayScalarString(target, node.Value)
	}
	return node.Decode(target.Addr().Interface())
}

func decodeOverlaySlice(target reflect.Value, node *yaml.Node) error {
	if node.Tag == "!!str" && target.Type().Elem().Kind() == reflect.String {
		return decodeOverlayStringList(target, node.Value)
	}
	return node.Decode(target.Addr().Interface())
}

func decodeOverlayScalarString(target reflect.Value, value string) error {
	switch target.Kind() {
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		target.SetBool(parsed)
		return err
	case reflect.Int:
		parsed, err := strconv.ParseInt(value, 10, target.Type().Bits())
		target.SetInt(parsed)
		return err
	case reflect.Uint32:
		parsed, err := strconv.ParseUint(value, 10, target.Type().Bits())
		target.SetUint(parsed)
		return err
	default:
		return errOverlayType
	}
}

func decodeOverlayDevices(target reflect.Value, node *yaml.Node) error {
	if node.Tag == "!!null" {
		target.SetZero()
		return nil
	}
	var entries []yaml.Node
	if err := node.Decode(&entries); err != nil {
		return err
	}
	devices := make([]DeviceEntry, len(entries))
	for i := range entries {
		devices[i].Volume = 1 // Legacy default applies only when volume is absent.
		if err := entries[i].Decode(&devices[i]); err != nil {
			return err
		}
	}
	target.Set(reflect.ValueOf(devices))
	return nil
}
