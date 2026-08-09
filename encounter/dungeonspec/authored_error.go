package dungeonspec

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

type authoredValidationError struct {
	field   string
	code    string
	message string
}

func (e *authoredValidationError) Error() string { return e.message }

func authoredError(field, code, format string, args ...any) error {
	return &authoredValidationError{field: field, code: code, message: fmt.Sprintf(format, args...)}
}

func authoredWrap(field, code string, err error) error {
	return &authoredValidationError{field: field, code: code, message: err.Error()}
}

// validateYAMLShape walks the authored YAML node alongside the public grammar.
// It supplies exact structural paths before yaml.v3's prose-only KnownFields
// errors can erase nesting/index information.
func validateYAMLShape(node *yaml.Node, typ reflect.Type, path string) error {
	return validateYAMLShapeActive(node, typ, path, make(map[*yaml.Node]struct{}))
}

func validateYAMLShapeActive(
	node *yaml.Node, typ reflect.Type, path string, active map[*yaml.Node]struct{},
) error {
	if node == nil {
		return authoredError(path, "invalid_yaml", "decode dungeon spec: %s is missing", path)
	}
	if _, recursive := active[node]; recursive {
		return authoredError(path, "invalid_yaml", "decode dungeon spec: YAML alias cycle at %s", path)
	}
	active[node] = struct{}{}
	defer delete(active, node)
	if node.Kind == yaml.AliasNode {
		return validateYAMLShapeActive(node.Alias, typ, path, active)
	}
	if node.Tag == "!!null" {
		return nil
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		if node.Kind != yaml.MappingNode {
			return authoredError(path, "invalid_yaml", "decode dungeon spec: %s must be a mapping", path)
		}
		fields := yamlStructFields(typ)
		for index := 0; index < len(node.Content); index += 2 {
			key, value := node.Content[index], node.Content[index+1]
			if key.Value == "<<" {
				if err := validateYAMLMerge(value, typ, path, active); err != nil {
					return err
				}
				continue
			}
			fieldType, ok := fields[key.Value]
			child := key.Value
			if path != "spec" {
				child = path + "." + key.Value
			}
			if !ok {
				return authoredError(
					child, "unknown_field", "decode dungeon spec: line %d: %s is not a supported field", key.Line, child,
				)
			}
			if err := validateYAMLShapeActive(value, fieldType, child, active); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if node.Kind != yaml.SequenceNode {
			return authoredError(path, "invalid_yaml", "decode dungeon spec: %s must be a sequence", path)
		}
		for index, child := range node.Content {
			if err := validateYAMLShapeActive(child, typ.Elem(), fmt.Sprintf("%s[%d]", path, index), active); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array:
		if node.Kind != yaml.SequenceNode || len(node.Content) != typ.Len() {
			return authoredError(path, "invalid_yaml", "decode dungeon spec: %s must be a %d-item sequence", path, typ.Len())
		}
		for index, child := range node.Content {
			if err := validateYAMLShapeActive(child, typ.Elem(), fmt.Sprintf("%s[%d]", path, index), active); err != nil {
				return err
			}
		}
		return nil
	default:
		if node.Kind != yaml.ScalarNode {
			return authoredError(path, "invalid_yaml", "decode dungeon spec: %s has an invalid YAML shape", path)
		}
		target := reflect.New(typ).Interface()
		if err := node.Decode(target); err != nil {
			return authoredWrap(path, "invalid_yaml", fmt.Errorf("decode dungeon spec: %w", err))
		}
		return nil
	}
}

func validateYAMLMerge(
	node *yaml.Node, typ reflect.Type, path string, active map[*yaml.Node]struct{},
) error {
	if node == nil {
		return authoredError(path, "invalid_yaml", "decode dungeon spec: %s merge is missing", path)
	}
	if node.Kind == yaml.AliasNode || node.Kind == yaml.MappingNode {
		return validateYAMLShapeActive(node, typ, path, active)
	}
	if node.Kind == yaml.SequenceNode {
		if _, recursive := active[node]; recursive {
			return authoredError(path, "invalid_yaml", "decode dungeon spec: YAML alias cycle at %s", path)
		}
		active[node] = struct{}{}
		defer delete(active, node)
		for _, merged := range node.Content {
			if merged.Kind != yaml.AliasNode && merged.Kind != yaml.MappingNode {
				return authoredError(path, "invalid_yaml", "decode dungeon spec: %s merge must contain mappings", path)
			}
			if err := validateYAMLShapeActive(merged, typ, path, active); err != nil {
				return err
			}
		}
		return nil
	}
	return authoredError(path, "invalid_yaml", "decode dungeon spec: %s merge must be a mapping or alias sequence", path)
}

func yamlStructFields(typ reflect.Type) map[string]reflect.Type {
	out := make(map[string]reflect.Type, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
		tag := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if tag == "-" {
			continue
		}
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}
		out[tag] = field.Type
	}
	return out
}
