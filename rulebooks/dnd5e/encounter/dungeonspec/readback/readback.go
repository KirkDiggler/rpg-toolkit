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

	// The joins (rpg-project#971, design R30, R31): where two regions meet,
	// each said once, about Subject{Kind: "join"}. A join's state is not
	// restated: it is the door's, its holders' and the secret's, read through
	// the join's connections.
	KindJoinDoor       Kind = "place.join.door"
	KindJoinOpen       Kind = "place.join.open"
	KindJoinPassage    Kind = "place.join.passage"
	KindRegionUnjoined Kind = "place.region.unjoined"
	// A seam sight crosses and feet do not (a window): not a join, said so
	// that nothing is skipped silently, about Subject{Kind: "seam"}.
	KindSeamSight Kind = "place.seam.sight"
	// A door that joins no two regions, about the door: both its sides one
	// region's floor (within), or a side off the floor (edge).
	KindDoorWithin Kind = "place.door.within"
	KindDoorEdge   Kind = "place.door.edge"

	// The ways through a join (rpg-project#981, design R30), each said right
	// after its join, about Subject{Kind: "way"}: nothing bars it (free), a
	// lock approach, a line past holding posts, knowing or carrying what a
	// pass asks (known), the fight with the holders, a search or a told
	// record that reveals a secret, or a passage's carrier. Partial helps
	// past some holders and not all, and is no way on its own; none is a bar
	// nothing lifts, about Subject{Kind: "bar"}.
	KindWayFree    Kind = "place.way.free"
	KindWayLock    Kind = "place.way.lock"
	KindWayLine    Kind = "place.way.line"
	KindWayFight   Kind = "place.way.fight"
	KindWaySearch  Kind = "place.way.search"
	KindWayTold    Kind = "place.way.told"
	KindWayPassage Kind = "place.way.passage"
	KindWayKnown   Kind = "place.way.known"
	KindWayPartial Kind = "place.way.partial"
	KindWayNone    Kind = "place.way.none"

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

	// The record and secret hubs (rpg-project#973): a record's own
	// statement (what it reveals), who holds it, who tells it, and the ways
	// it lifts; a secret's own statement (what it hides), the records that
	// reveal it, how it is found (said once, from its own checks), the joins
	// it bars and the ways that need it. A way repeated under one of these
	// hubs is a pointer to the way ({way, <id>, is}) and never itself a way:
	// a WAY is a statement whose kind begins `place.way.`.
	KindRecordReveal Kind = "record.reveal"
	KindRecordHold   Kind = "record.hold"
	KindRecordTell   Kind = "record.tell"
	KindRecordLift   Kind = "record.lift"
	KindSecretHide   Kind = "secret.hide"
	KindSecretReveal Kind = "secret.reveal"
	KindSecretFind   Kind = "secret.find"
	KindSecretBar    Kind = "secret.bar"
	KindSecretNeed   Kind = "secret.need"

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
	// Value is a number the connection carries where its kind has one: the
	// difficulty of a check on a `method` connection; zero means none.
	Value int `json:"value,omitempty"`

	// Is, Op, Not and At belong to a [ConnCondition] connection only.

	// Is is the value a condition compares, as text: "failed", "true", "3".
	Is string `json:"is,omitempty"`
	// Op is the comparison where it is not equality ("gte"; "lte" for
	// `within` and `regions`, at most); the plain words for each are the
	// machine words table's ([encounter.MachineWordsTable.Ops]).
	Op string `json:"op,omitempty"`
	// Not says this leaf sits under an odd number of `not`s. It is a hint,
	// not the structure: At is what makes the structure exact. Under
	// `not: { all: [a, b] }` both a and b carry Not, at `when.not.all[0]`
	// and `when.not.all[1]`, and the step needs NOT (a and b), not
	// (NOT a) and (NOT b).
	Not bool `json:"not,omitempty"`
	// At is where the condition sits inside the step's `when`, as the author
	// wrote it: "when", "when.all[1]", "when.any[0].not". Several conditions
	// at one place are one node, all of them needed.
	At string `json:"at,omitempty"`
}

// The connections that say a behaviour machine's shape (rpg-project#994),
// so a reader draws the machine from connections alone. A state id is local
// to its machine, and the machine is named by the statement's PATH, never
// its subject: a routine's statements sit under
// `behaviorDefinitions.<routine>.`, and a creature's post or blocking under
// its binding, whose routine is the one its `behaviorState` names. A step
// repeated under a fact or topic hub keeps its shape connections while its
// subject is the fact or topic; its path still names the routine that owns
// the step.
const (
	// ConnState is a connection to one of a machine's states, scoped by the
	// routine the statement's path names.
	ConnState = "state"
	// ConnBeat is a connection to the beat a step wakes on, its word exactly
	// as authored ("time", "fact", "deed:x", "clock:world").
	ConnBeat = "beat"
	// ConnTable is a connection to one key of the routine's table of rows.
	ConnTable = "table"
	// ConnCondition is one condition a step's `when` reads (rpg-project#997),
	// role "asks": its id is the word as authored with its open part filled
	// ("heard.claim", "noted:admitted", "noted.anyone:admitted",
	// "fact:gate-cowed", "within", "rounds", "flag:lit"), from the machine words
	// table ([encounter.MachineWords]); Is, Op, Not and At say the rest, so
	// a reader rebuilds the condition from connections alone. A condition
	// that reads a thing in the file (a fact, a topic) also keeps that
	// thing's own connection, which the hubs group by.
	ConnCondition = "condition"

	// RoleStarts is the state a routine starts in, on the routine statement.
	RoleStarts = "starts"
	// RoleIs is the state a routine.state statement says.
	RoleIs = "is"
	// RoleFinal marks a state with no way out, beside its RoleIs.
	RoleFinal = "final"
	// RoleFrom is the state a step is heard in.
	RoleFrom = "from"
	// RoleTo is the state a step moves to. A step that names its own state
	// re-enters it (the engine treats it as a move) and still says `to`; a
	// step with no `to` stays and has none.
	RoleTo = "to"
	// RoleOn is the beat a step wakes on.
	RoleOn = "on"
	// RolePicks is a table key a step acts from, one per pick.
	RolePicks = "picks"
	// RoleRowOf is the table key a row belongs to.
	RoleRowOf = "row-of"
	// RoleWhile is a state a post holds, or a body blocks, in.
	RoleWhile = "while"
)

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
