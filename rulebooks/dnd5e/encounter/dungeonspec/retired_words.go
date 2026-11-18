// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// retired_words.go is THE NOTED WORDS' OLD SPELLINGS (rpg-project#1005,
// ruled on rpg-project#987 and after): what an NPC has in mind about a
// person is a topic, asked with `noted` and given with `give`; "hold" stays
// physical (a prop in hand is `holds`); a state's post is `post`; a machine
// asks about anybody with `noted.anyone:`. Every old word is refused when
// content is loaded, with a sentence naming the word that replaced it —
// never read under its old meaning, so a file written before the rename
// cannot silently mean something else after it. Nothing on the wire and
// nothing in a saved run changed name.

// The refusal sentences, one per retired word. `recognized` is
// [retiredPredicateKeys]'; `topic:` in a guard is [topicGuardRetired].
const (
	recognizeIsGive = "`recognize` is now `give`: write give: { topic: <topic>, to: spoken_to, " +
		"about: party_in_sight|speaker, heard_by: carriers_who_witnessed }"
	holderIsTo       = "`holder` is now `to`: write give: { topic, to: spoken_to, about }"
	stateHoldsIsPost = "a state's `holds` is now `post`: write post: { door: <door id>, at, against, pass }"
)

// retiredAt is the refusal of an old word whose value starts at node.
func retiredAt(node *yaml.Node, sentence string) error {
	return fmt.Errorf("line %d: %s", node.Line, sentence)
}

// retiredRecognize is an outcome's retired `recognize:` ([recognizeIsGive]).
type retiredRecognize struct{}

// UnmarshalYAML refuses the old word, naming the new one.
func (*retiredRecognize) UnmarshalYAML(node *yaml.Node) error {
	return retiredAt(node, recognizeIsGive)
}

// retiredStateHolds is a state's retired `holds:` ([stateHoldsIsPost]).
type retiredStateHolds struct{}

// UnmarshalYAML refuses the old word, naming the new one.
func (*retiredStateHolds) UnmarshalYAML(node *yaml.Node) error {
	return retiredAt(node, stateHoldsIsPost)
}

// giveKeys is every key a give takes, in the order a refusal lists them.
var giveKeys = []string{"topic", "to", "about", "heard_by"}

// UnmarshalYAML reads a give, refusing a key it does not take by name and
// line — a give hangs under a row or an outcome whose own reader does not
// carry the decoder's strictness this far down — and the retired `holder`
// naming `to` ([holderIsTo]).
func (g *GiveSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i]
			switch {
			case key.Value == "holder":
				return retiredAt(key, holderIsTo)
			case !slices.Contains(giveKeys, key.Value):
				return fmt.Errorf("line %d: field %s not found in type dungeonspec.GiveSpec: a give takes %s",
					key.Line, key.Value, strings.Join(giveKeys, ", "))
			}
		}
	}
	type giveBody GiveSpec
	var body giveBody
	if err := value.Decode(&body); err != nil {
		return err
	}
	*g = GiveSpec(body)
	return nil
}
