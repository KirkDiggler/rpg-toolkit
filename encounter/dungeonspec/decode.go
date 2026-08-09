// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"

	"gopkg.in/yaml.v3"
)

// Decode strictly decodes a dungeon spec from YAML bytes. Unknown fields
// (typos, stale keys) fail loudly instead of being silently dropped — this
// is why yaml.NewDecoder with KnownFields(true) is used instead of the
// plain yaml.Unmarshal, which would drop them silently. A dungeon spec is
// exactly one YAML document: empty input and a stray second `---` document
// both fail loudly here rather than silently decoding to a zero-value spec
// or silently dropping everything after the first document.
func Decode(raw []byte) (*DungeonSpec, error) {
	shape, hasCanvas := dungeonRoomsShape(raw)
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, authoredWrap("spec", "invalid_yaml", fmt.Errorf("decode dungeon spec: %w", err))
	}
	if len(document.Content) == 0 {
		return nil, authoredError("spec", "invalid_yaml", "empty dungeon spec")
	}
	if hasCanvas && shape == roomsInvalid {
		return nil, authoredError("rooms", "invalid_yaml", "canvas mode rooms must be an explicit empty sequence (rooms: [])")
	}
	if err := validatePlacementOffsetNodes(document.Content[0]); err != nil {
		return nil, err
	}
	if err := validateYAMLShape(document.Content[0], reflect.TypeOf(DungeonSpec{}), "spec"); err != nil {
		return nil, err
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)

	var spec DungeonSpec
	if err := dec.Decode(&spec); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, authoredError("spec", "invalid_yaml", "empty dungeon spec")
		}
		if hasCanvas && shape == roomsInvalid {
			return nil, authoredError(
				"rooms", "invalid_yaml", "canvas mode rooms must be an explicit empty sequence (rooms: [])",
			)
		}
		return nil, authoredWrap("spec", "invalid_yaml", fmt.Errorf("decode dungeon spec: %w", err))
	}

	// A second call to Decode reads the next YAML document in the stream,
	// if any. yaml.v3 otherwise accepts multi-document input and silently
	// discards everything past the first document — the same silent-drop
	// class KnownFields already closes for unknown fields, closed here for
	// stray `---` documents.
	var extra DungeonSpec
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, authoredError("spec", "invalid_yaml", "multi-document YAML not supported: a dungeon spec is one document")
	}

	spec.roomsShape = shape
	return &spec, nil
}

// dungeonRoomsShape records the authored top-level rooms node form without
// weakening the strict typed decode that follows. yaml.v3 decodes omitted and
// null collections alike as nil slices, but canvas mode gives those forms
// different meaning from an explicit empty sequence.
func dungeonRoomsShape(raw []byte) (roomsShape, bool) {
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return roomsOmitted, false
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return roomsOmitted, false
	}

	shape := roomsOmitted
	hasCanvas := false
	mapping := document.Content[0]
	for i := 0; i < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		switch key.Value {
		case "canvas":
			hasCanvas = true
		case "rooms":
			switch value.Kind {
			case yaml.SequenceNode:
				shape = roomsSequence
			case yaml.ScalarNode:
				if value.Tag == "!!null" {
					shape = roomsNull
				} else {
					shape = roomsInvalid
				}
			default:
				shape = roomsInvalid
			}
		}
	}
	return shape, hasCanvas
}

// ValidationError identifies one invalid authored field by its canonical
// dungeonspec source path.
type ValidationError struct {
	Field   string
	Message string
}

// Error returns the source path followed by the validation failure.
func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

func newValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// validatePlacementOffsetNodes inspects placement offsets before generic shape
// decoding so explicit nulls and null components cannot collapse into omission
// or numeric zero.
func validatePlacementOffsetNodes(root *yaml.Node) error {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	if place := mappingNodeValue(root, "place"); place != nil {
		if err := validatePlaceOffsetSequence(place, "place"); err != nil {
			return err
		}
	}
	rooms := resolveYAMLAlias(mappingNodeValue(root, "rooms"))
	if rooms == nil || rooms.Kind != yaml.SequenceNode {
		return nil
	}
	for roomIndex, room := range rooms.Content {
		prefix := fmt.Sprintf("rooms[%d]", roomIndex)
		if place := mappingNodeValue(room, "place"); place != nil {
			if err := validatePlaceOffsetSequence(place, prefix+".place"); err != nil {
				return err
			}
		}
		boss := mappingNodeValue(room, "boss")
		if boss == nil {
			continue
		}
		if offset := mappingNodeValue(boss, "offset"); offset != nil {
			if err := validatePlacementOffsetNode(prefix+".boss.offset", offset); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePlaceOffsetSequence(sequence *yaml.Node, prefix string) error {
	sequence = resolveYAMLAlias(sequence)
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		return nil
	}
	for index, entry := range sequence.Content {
		if offset := mappingNodeValue(entry, "offset"); offset != nil {
			if err := validatePlacementOffsetNode(fmt.Sprintf("%s[%d].offset", prefix, index), offset); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePlacementOffsetNode(path string, node *yaml.Node) error {
	node = resolveYAMLAlias(node)
	if node == nil || node.Kind != yaml.SequenceNode {
		return newValidationError(path, "must be exactly three finite numeric [x,y,z] components")
	}
	if len(node.Content) != 3 {
		return newValidationError(path, fmt.Sprintf("must contain exactly three components (got %d)", len(node.Content)))
	}
	for index, component := range node.Content {
		component = resolveYAMLAlias(component)
		componentPath := fmt.Sprintf("%s[%d]", path, index)
		if component == nil || component.Kind != yaml.ScalarNode || component.Tag != "!!int" && component.Tag != "!!float" {
			return newValidationError(componentPath, "must be a finite number")
		}
		var value float64
		if err := component.Decode(&value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return newValidationError(componentPath, "must be a finite number")
		}
	}
	return nil
}

func mappingNodeValue(mapping *yaml.Node, name string) *yaml.Node {
	return mappingNodeValueActive(mapping, name, make(map[*yaml.Node]struct{}))
}

func mappingNodeValueActive(mapping *yaml.Node, name string, active map[*yaml.Node]struct{}) *yaml.Node {
	mapping = resolveYAMLAlias(mapping)
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	if _, recursive := active[mapping]; recursive {
		return nil // generic shape validation reports the alias cycle
	}
	active[mapping] = struct{}{}
	defer delete(active, mapping)
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == name {
			return mapping.Content[index+1]
		}
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != "<<" {
			continue
		}
		merged := resolveYAMLAlias(mapping.Content[index+1])
		if merged != nil && merged.Kind == yaml.SequenceNode {
			for _, candidate := range merged.Content {
				if value := mappingNodeValueActive(candidate, name, active); value != nil {
					return value
				}
			}
			continue
		}
		if value := mappingNodeValueActive(merged, name, active); value != nil {
			return value
		}
	}
	return nil
}

func resolveYAMLAlias(node *yaml.Node) *yaml.Node {
	seen := make(map[*yaml.Node]struct{})
	for node != nil && node.Kind == yaml.AliasNode {
		if _, recursive := seen[node]; recursive {
			return nil
		}
		seen[node] = struct{}{}
		node = node.Alias
	}
	return node
}
