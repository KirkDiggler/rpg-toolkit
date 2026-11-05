// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// sequenceTemplate reads the literal structural shell. Only values inside that
// shell are references; bindings cannot replace the program's structural maps.
type sequenceTemplate struct {
	body    jsontext.Value
	initial jsontext.Value
	tables  map[string]jsontext.Value
}

func readSequenceTemplate(raw jsontext.Value) (*sequenceTemplate, error) {
	body, err := sequenceObject(raw, "body")
	if err != nil {
		return nil, err
	}
	if len(body) != 2 || body["recipe"] == nil || body["table"] == nil {
		return nil, fmt.Errorf("body requires exactly recipe and table")
	}
	recipe, err := sequenceObject(body["recipe"], "body.recipe")
	if err != nil {
		return nil, err
	}
	if _, present := recipe["ref"]; present {
		return nil, fmt.Errorf("body.recipe.ref belongs to the outer definition")
	}
	states, err := sequenceObject(recipe["states"], "body.recipe.states")
	if err != nil {
		return nil, err
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("body.recipe.states is empty")
	}
	tables, err := sequenceObject(body["table"], "body.table")
	if err != nil {
		return nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(tables)) {
		if name == "" || strings.HasPrefix(name, "$") {
			return nil, fmt.Errorf("invalid local table name %q", name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(states)) {
		if name == "" || strings.HasPrefix(name, "$") {
			return nil, fmt.Errorf("invalid local state name %q", name)
		}
	}
	return &sequenceTemplate{body: raw, initial: recipe["initial"], tables: tables}, nil
}

func sequenceObject(raw jsontext.Value, path string) (map[string]jsontext.Value, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return nil, fmt.Errorf("%s requires an object", path)
	}
	var object map[string]jsontext.Value
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return object, nil
}

type sequenceResolver struct {
	item       jsontext.Value
	parameters map[string]jsontext.Value
	next       string
	done       map[string]string
}

// value never revisits an inserted value. A supplied literal that resembles a
// reference cannot acquire executable meaning at a second pass.
func (r sequenceResolver) value(raw jsontext.Value, path string) (jsontext.Value, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%s: missing value", path)
	}
	switch trimmed[0] {
	case '{':
		object, err := sequenceObject(raw, path)
		if err != nil {
			return nil, err
		}
		for _, key := range slices.Sorted(maps.Keys(object)) {
			if strings.HasPrefix(key, "$") {
				if len(object) != 1 {
					return nil, fmt.Errorf("%s: reference must have exactly one member", path)
				}
				return r.reference(key, object[key], path)
			}
		}
		for _, key := range slices.Sorted(maps.Keys(object)) {
			value, err := r.value(object[key], path+"."+key)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		return json.Marshal(object, json.Deterministic(true))
	case '[':
		var array []jsontext.Value
		if err := json.Unmarshal(raw, &array); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for i := range array {
			value, err := r.value(array[i], fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			array[i] = value
		}
		return json.Marshal(array)
	default:
		var value jsontext.Value
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return value, nil
	}
}

func (r sequenceResolver) reference(key string, raw jsontext.Value, path string) (jsontext.Value, error) {
	switch key {
	case "$item", "$next":
		var flag bool
		if err := json.Unmarshal(raw, &flag); err != nil || !flag {
			return nil, fmt.Errorf("%s: %s requires true", path, key)
		}
		if key == "$item" {
			return slices.Clone(r.item), nil
		}
		if r.next == "" {
			return nil, fmt.Errorf("%s: next cannot determine the initial state", path)
		}
		return json.Marshal(r.next)
	case "$param", "$done":
		var name string
		if err := json.Unmarshal(raw, &name); err != nil || name == "" {
			return nil, fmt.Errorf("%s: %s requires a name", path, key)
		}
		if key == "$param" {
			value, ok := r.parameters[name]
			if !ok {
				return nil, fmt.Errorf("%s: parameter %q is not declared and supplied", path, name)
			}
			return slices.Clone(value), nil
		}
		beat, ok := r.done[name]
		if !ok {
			return nil, fmt.Errorf("%s: completion table %q is unavailable", path, name)
		}
		return json.Marshal(beat)
	default:
		return nil, fmt.Errorf("%s: unknown reference %q", path, key)
	}
}
