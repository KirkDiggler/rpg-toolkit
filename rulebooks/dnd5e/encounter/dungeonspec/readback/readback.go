// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"gopkg.in/yaml.v3"
)

// ErrNotASite is returned for a document that is not a site document
// (version 3 and above): the read-back reads the site dialect only.
var ErrNotASite = errors.New("readback: the read-back reads site documents (version 3 and above)")

// Kind is the stable name of what a statement says. A consumer keys on it;
// the sentence beside it may be reworded.
type Kind string

// The kinds of statement, grouped by what they are about.
const (
	KindPlace       Kind = "place"
	KindRegions     Kind = "place.regions"
	KindGeometry    Kind = "place.geometry"
	KindStart       Kind = "place.start"
	KindSharing     Kind = "place.sharing"
	KindExit        Kind = "place.exit"
	KindEnding      Kind = "place.ending"
	KindDoor        Kind = "place.door"
	KindProp        Kind = "place.prop"
	KindIntel       Kind = "place.intel"
	KindConcealment Kind = "place.concealment"
	KindTemplate    Kind = "place.template"
	KindBundle      Kind = "place.bundle"
	KindFaction     Kind = "side.faction"
	KindDisposition Kind = "side.disposition"

	KindCreature    Kind = "creature"
	KindHolds       Kind = "creature.holds"
	KindArrives     Kind = "creature.arrives"
	KindRoutineUse  Kind = "creature.routine"
	KindBlocking    Kind = "creature.blocking"
	KindPost        Kind = "creature.post"
	KindPass        Kind = "creature.post.pass"
	KindBorrowLines Kind = "creature.lines"

	KindLine        Kind = "line"
	KindLineCheck   Kind = "line.check"
	KindLineRequire Kind = "line.requires"
	KindLineAdvance Kind = "line.advance"
	KindLineRow     Kind = "line.row"
	KindLineGrant   Kind = "line.grant"

	KindConversation Kind = "conversation"
	KindApproach     Kind = "conversation.approach"
	KindHailers      Kind = "conversation.hailers"
	KindEngineRule   Kind = "conversation.engine-rule"
	KindDoorHold     Kind = "place.door.holders"
	KindConvState    Kind = "conversation.state"
	KindConvStep     Kind = "conversation.transition"
	KindOutcome      Kind = "conversation.outcome"
	KindConvAsked    Kind = "conversation.asked"

	KindTopic      Kind = "topic"
	KindTopicGrant Kind = "topic.grant"
	KindTopicAsk   Kind = "topic.ask"
	KindFact       Kind = "fact"
	KindFactTeach  Kind = "fact.teach"
	KindFactAsk    Kind = "fact.ask"

	KindRoutine      Kind = "routine"
	KindRoutineState Kind = "routine.state"
	KindRoutineWake  Kind = "routine.wake"
	KindRoutineRow   Kind = "routine.row"
)

// EngineRule is the path of a statement that says how the engine runs what
// the file authors, rather than something the file sets: it has no place in
// the file.
const EngineRule = "(engine rule)"

// Subject is what a statement or a group is about: a kind of thing and its
// id in the file ("creature" "gate-1", "topic" "admitted").
type Subject struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Connection is one thing a statement reads or changes, and how: the edge a
// later slice draws. Role is a verb from the reader's side: "grants",
// "asks", "teaches", "wakes-on", "advances", "reads", "holds", "said-to",
// "uses", "door", "carries".
type Connection struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Role string `json:"role"`
}

// Statement is one plain sentence about one thing, with where it is set.
type Statement struct {
	Kind     Kind         `json:"kind"`
	About    Subject      `json:"about"`
	Sentence string       `json:"sentence"`
	Path     string       `json:"path"`
	Connects []Connection `json:"connects,omitempty"`

	fold foldInfo
}

// Group is the statements about one thing, in reading order.
type Group struct {
	Title      string      `json:"title"`
	About      Subject     `json:"about"`
	Statements []Statement `json:"statements"`
}

// Section is one heading of the read-back: the place, the creatures, the
// conversations, the topics, the facts, the routines.
type Section struct {
	Title  string  `json:"title"`
	Groups []Group `json:"groups"`
}

// Unsaid is one authored thing the read-back cannot yet say.
type Unsaid struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Why  string `json:"why"`
}

// ReadBack is a whole encounter said back.
type ReadBack struct {
	Key        string    `json:"key"`
	Name       string    `json:"name"`
	Sections   []Section `json:"sections"`
	NotYetSaid []Unsaid  `json:"not_yet_said"`
}

// Statements is every statement in reading order, across sections.
func (r *ReadBack) Statements() []Statement {
	var out []Statement
	for _, s := range r.Sections {
		for _, g := range s.Groups {
			out = append(out, g.Statements...)
		}
	}
	return out
}

// Of compiles raw with the dialect's loader and says it back.
//
// Errors: [ErrNotASite] for a document below version 3; the compiler's own
// refusal (a [*dungeonspec.ValidationError]) for one that does not compile.
func Of(raw []byte) (*ReadBack, error) {
	var version struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(raw, &version); err != nil || version.Version < 3 {
		return nil, ErrNotASite
	}
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	if err != nil {
		return nil, err
	}
	compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: decoded.Spec})
	if err != nil {
		return nil, err
	}
	// The compile may resolve parts of the spec in place (bundles); the
	// paths are read from a fresh decode so they are the author's.
	fresh, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: bytes.Clone(raw)})
	if err != nil {
		return nil, fmt.Errorf("readback: second decode: %w", err)
	}
	b := newBuilder(fresh.Spec, &compiled, raw)
	return b.build(), nil
}
