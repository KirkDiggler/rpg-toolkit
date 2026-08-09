// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
			return nil, authoredError("rooms", "invalid_yaml", "canvas mode rooms must be an explicit empty sequence (rooms: [])")
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
