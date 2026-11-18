// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// retired_words.go is THE HOLDING WORDS' OLD SPELLINGS (rpg-project#1005,
// ruled on rpg-project#987): what an NPC holds about a person is asked with
// `holds` and given with `give`; carrying a thing is `carries`; a state's
// post is `post`; a machine asks about anybody with `holds.anyone:`. Every
// old word is refused when content is loaded, with a sentence naming the
// word that replaced it — never read under its old meaning, so a file
// written before the rename cannot silently mean something else after it.
// Nothing on the wire and nothing in a saved run changed name.

// The refusal sentences, one per retired word. `recognized` is
// [retiredPredicateKeys]'; `topic:` in a guard is [topicGuardRetired].
const (
	recognizeIsGive = "`recognize` is now `give`: write give: { holding: <holding>, to: spoken_to, " +
		"about: party_in_sight|speaker, heard_by: carriers_who_witnessed }"
	topicIsHolding   = "`topic` is now `holding`: write give: { holding: <holding>, to, about }"
	holderIsTo       = "`holder` is now `to`: write give: { holding, to: spoken_to, about }"
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

// retiredTopic is a giving's retired `topic:` ([topicIsHolding]).
type retiredTopic struct{}

// UnmarshalYAML refuses the old word, naming the new one.
func (*retiredTopic) UnmarshalYAML(node *yaml.Node) error { return retiredAt(node, topicIsHolding) }

// retiredHolder is a giving's retired `holder:` ([holderIsTo]).
type retiredHolder struct{}

// UnmarshalYAML refuses the old word, naming the new one.
func (*retiredHolder) UnmarshalYAML(node *yaml.Node) error { return retiredAt(node, holderIsTo) }

// retiredStateHolds is a state's retired `holds:` ([stateHoldsIsPost]).
type retiredStateHolds struct{}

// UnmarshalYAML refuses the old word, naming the new one.
func (*retiredStateHolds) UnmarshalYAML(node *yaml.Node) error {
	return retiredAt(node, stateHoldsIsPost)
}
