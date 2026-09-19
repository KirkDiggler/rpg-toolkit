// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// AN UNKNOWN KEY IS NAMED AT ITS PATH, LIKE EVERY OTHER DEFECT
// (rpg-project#481, R2).
//
// `KnownFields(true)` catches the typo — that is what it is for — but what it
// says is a fact about Go: "line 129: field tempre not found in type
// dungeonspec.FactionSpec". Every other refusal in this package is a
// [FieldError] at the author's own address for the thing that is wrong
// (`factions[0].on.sunrise`, `place[7].blocks_los`, `regions[1].cells[0][3]`),
// and the World Builder draws refusals at those paths. A line number cannot be
// drawn on a canvas and a Go type name is not a sentence, so the one decode
// defect an author causes most often was the one defect they could not be
// shown.
//
// The translation is two lookups over what is already in hand. The offending
// key's LINE is in yaml.v3's own message and the file is still in the caller's
// bytes, so re-reading the source as a [yaml.Node] tree and walking it once
// gives every authored key its path; the TYPE is in the message too, so
// reflecting over the spec's shapes gives the keys that type does take. The
// grammar is untouched: the same files are refused, at the same keys, for the
// same reason.
//
// THE CUSTOM UNMARSHALERS COME ALONG FOR FREE. [PlaceSpec], [AnswerSpec],
// [PredicateSpec] and the rest bypass `KnownFields` and so write their own
// unknown-key refusal by hand, deliberately in yaml.v3's words so an author
// sees one refusal and not two ([PositionSpec.UnmarshalYAML]). Because they
// say it the same way, they are rewritten the same way, and their defects gain
// the same paths without any of them changing.

// The two halves of yaml.v3's unknown-key sentence, and of the hand-written
// ones that mimic it: "field <key> not found in type <type>".
const (
	unknownFieldPrefix = "field "
	unknownFieldInType = " not found in type "
)

// unknownKeyDefect rewrites one decode message into a defect at the offending
// key's YAML path, and reports whether the message was an unknown-key refusal
// at all.
//
// `line` is the "line N" the caller already split off the front of the
// message — the only place the key's position is recorded — and it stays as
// the path when the walk cannot place the key (an aliased node, or two keys of
// the same name authored on one line). A path that might be the wrong one is
// worse than no path, and the SENTENCE is rewritten either way, so an author
// never reads a Go type name even in the case the walk cannot address.
func unknownKeyDefect(line, message string, keys authoredKeys) (FieldError, bool) {
	if !strings.HasPrefix(message, unknownFieldPrefix) {
		return FieldError{}, false
	}
	rest := strings.TrimPrefix(message, unknownFieldPrefix)
	i := strings.Index(rest, unknownFieldInType)
	if i <= 0 {
		return FieldError{}, false
	}
	key, typeName := rest[:i], rest[i+len(unknownFieldInType):]
	if key == "" || typeName == "" {
		return FieldError{}, false
	}
	path := line
	if at, ok := keys.pathOf(line, key); ok {
		path = at
	}

	return FieldError{Path: path, Message: unknownKeySentence(key, typeName)}, true
}

// unknownKeySentence is the refusal in this package's own voice, in the form
// [validation.placeOn] uses for a trigger key that is not one: the word that
// is wrong, and the words that would have been right.
//
// The list is omitted rather than guessed for a shape whose keys are read by
// hand ([PlaceSpec.UnmarshalYAML] and the other custom unmarshalers), because
// a struct's fields are not that shape's grammar — [AnswerSpec] carries a
// `Line` the file never authors, and [TemperSpec] takes a word OR a mix. An
// author reading "they are ..." must be able to trust the list.
func unknownKeySentence(key, typeName string) string {
	allowed := authoredKeysOfType(typeName)
	if len(allowed) == 0 {
		return fmt.Sprintf("%q is not a key this build reads", key)
	}

	return fmt.Sprintf("%q is not a key this build reads: they are %s", key, strings.Join(allowed, ", "))
}

// authoredKeys is every mapping key in the source, by the line it was written
// on, each with the YAML path that addresses it.
//
// Keyed by LINE because that is the only address yaml.v3 hands back with a
// `KnownFields` refusal, and the key's own node is the one it reports
// (`d.terrors` names `n.Content[i]`, the key, not its value). The key is the
// parser's own "line N" spelling rather than the number, so the lookup is the
// string the message already carries and nothing has to be parsed back out.
type authoredKeys map[string][]authoredKey

// authoredKey is one key node: what it was called, and where it sits.
type authoredKey struct {
	name string
	path string
}

// pathOf answers the path of the key called `name` written on `line`, where
// `line` is yaml.v3's own "line N" spelling.
//
// TWO OF THE SAME NAME ON ONE LINE IS NO ANSWER. Flow style can author
// `{ a: { temper: 1 }, temper: 2 }` on a single line, and the two are
// different paths; picking either would draw the refusal on a key that is
// fine. The caller keeps the line in that case.
func (k authoredKeys) pathOf(line, name string) (string, bool) {
	found := ""
	for _, key := range k[line] {
		if key.name != name {
			continue
		}
		if found != "" {
			return "", false
		}
		found = key.path
	}

	return found, found != ""
}

// indexAuthoredKeys walks the source as a node tree and gives every mapping
// key its path.
//
// Source rather than spec, because the spec is exactly what failed to decode:
// the tree is what the author wrote, whether or not it fits any Go shape. A
// source that will not even parse indexes nothing, and the caller falls back
// to the line — there is no key to address in a file the parser never read.
func indexAuthoredKeys(raw []byte) authoredKeys {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil
	}
	keys := authoredKeys{}
	walkAuthoredKeys(&root, "", keys)

	return keys
}

// walkAuthoredKeys records each mapping key under the path it is reached by,
// in the spelling the rest of this package's paths use: `.` into a mapping,
// `[i]` into a sequence.
//
// An alias is not walked. Its anchor was indexed where it was AUTHORED, which
// is the line yaml.v3 reports for anything inside it, and walking the alias
// too would record a second path for the same line.
func walkAuthoredKeys(node *yaml.Node, path string, keys authoredKeys) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			walkAuthoredKeys(child, path, keys)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			name, value := node.Content[i], node.Content[i+1]
			at := name.Value
			if path != "" {
				at = path + "." + name.Value
			}
			line := fmt.Sprintf("line %d", name.Line)
			keys[line] = append(keys[line], authoredKey{name: name.Value, path: at})
			walkAuthoredKeys(value, at, keys)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			walkAuthoredKeys(child, fmt.Sprintf("%s[%d]", path, i), keys)
		}
	}
}

// yamlUnmarshaler is the interface a shape implements to read itself by hand.
var yamlUnmarshaler = reflect.TypeOf((*yaml.Unmarshaler)(nil)).Elem()

var (
	knownKeysOnce sync.Once
	knownKeys     map[string][]string
)

// authoredKeysOfType answers the keys a spec shape takes, by the Go type name
// yaml.v3 puts in its refusal ("dungeonspec.FactionSpec"), or nothing for a
// shape whose grammar this cannot read off the struct.
//
// Reflected off [Spec] rather than listed, so a key added to a shape is
// offered by the refusal the day it exists and a key removed stops being
// offered the day it goes. A list written here would be a second spelling of
// the grammar, and a second spelling drifts.
func authoredKeysOfType(typeName string) []string {
	knownKeysOnce.Do(func() {
		knownKeys = map[string][]string{}
		collectAuthoredKeys(reflect.TypeOf(Spec{}), map[reflect.Type]bool{}, knownKeys)
	})

	return knownKeys[typeName]
}

// collectAuthoredKeys records the yaml keys of every struct reachable from t,
// under that struct's own type name.
func collectAuthoredKeys(t reflect.Type, seen map[reflect.Type]bool, out map[string][]string) {
	// A shape is reached through whatever carries it: a list of factions, a
	// pointer to a start, a map of scenario bindings. Only the thing at the
	// bottom has fields.
	for t != nil && (t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice ||
		t.Kind() == reflect.Array || t.Kind() == reflect.Map) {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true

	// A shape that reads itself by hand is not described by its fields, so its
	// keys are collected from nothing — but what it CONTAINS is still walked,
	// because the shapes below it are read by the decoder as usual.
	byHand := reflect.PointerTo(t).Implements(yamlUnmarshaler)
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if name, authored := yamlKeyOf(field); authored && !byHand {
			keys = append(keys, name)
		}
		collectAuthoredKeys(field.Type, seen, out)
	}
	if byHand || len(keys) == 0 {
		return
	}
	sort.Strings(keys)
	out[t.String()] = keys
}

// yamlKeyOf is the key a field is authored under, and whether it is authored
// at all — yaml.v3's own rule: the tag's name, or the lowercased field name,
// and `-` means the decoder fills it rather than the file.
func yamlKeyOf(field reflect.StructField) (string, bool) {
	name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
	switch name {
	case "-":
		return "", false
	case "":
		return strings.ToLower(field.Name), true
	default:
		return name, true
	}
}
