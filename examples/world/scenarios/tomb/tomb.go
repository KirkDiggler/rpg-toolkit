// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package tomb

import (
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// The fixed cast. Rooms and adventurers are always these; what a dungeon
// builder places per instance is the artifact, the captain, and the door's
// checks — see [Config].
const (
	// Party is the crew taking this on.
	Party journal.EntityID = "tomb-party"

	// Finch is the one of them who knows how to search and how to pick a
	// lock. Every contested check in this content is Finch's to make.
	Finch journal.EntityID = "finch"

	// Bram belongs to the party and does neither. He exists to prove a
	// negative: a party-mate who never searched and never fought still
	// learns nothing until somebody opens the door in front of him.
	Bram journal.EntityID = "bram"

	// Thane is the fighter who does the actual fighting. Combat resolution
	// is out of this spike's scope, the same as UC-1's camp: its outcome
	// enters as a declared fact.
	Thane journal.EntityID = "thane"

	// HiddenRoom holds the artifact, behind the door. Declared concealed —
	// see [declaration].
	HiddenRoom journal.EntityID = "hidden-room"

	// BossRoom holds the captain and the loot chest.
	BossRoom journal.EntityID = "boss-room"
)

// The declared vocabulary.
const (
	// BelongsTo is this world's membership relation.
	BelongsTo graph.Relation = "belongs-to"

	// LeadsTo is the passage from the boss room to the hidden room — a plain
	// structural edge, real from the moment the world exists. Declared
	// concealed: the kernel hides it from a stranger's structural view until
	// somebody finds it, pierces it for themselves, or opens the door for
	// good. See [declaration].
	LeadsTo graph.Relation = "leads-to"

	// Recovered is raised on the artifact once a knower gets through the
	// door. It is an ordinary completion flag, not a knowledge-scoping one:
	// everyone who witnesses the door opening sees it the same way.
	Recovered graph.Flag = "recovered"

	// Looted is raised on the boss room once its chest is taken. The fight's
	// own reward, independent of whether anyone ever finds the door.
	Looted graph.Flag = "looted"

	// KindFaction is the party, addressed as a unit.
	KindFaction graph.Kind = "faction"

	// KindPerson is somebody who can act or be acted on.
	KindPerson graph.Kind = "person"

	// KindMonster is the captain.
	KindMonster graph.Kind = "monster"

	// KindRoom is a place.
	KindRoom graph.Kind = "room"

	// KindItem is a thing that can be recovered.
	KindItem graph.Kind = "item"
)

// Fact kinds this content declares.
const (
	// FactLocationKnown is where the hidden room is, told two different ways
	// — see the [Defeat] and [Search] outcomes in [verbs].
	FactLocationKnown journal.Kind = "location-known"

	// FactSearchFailed is a search that came up empty. It reveals nothing:
	// no reducer folds it, so nobody's present changes because of it.
	FactSearchFailed journal.Kind = "search-failed"

	// FactDoorOpened is a knower succeeding the open check. This is the fact
	// that shares the door with whoever else is there — not the knowing.
	FactDoorOpened journal.Kind = "door-opened"

	// FactOpenFailed is an open attempt that did not land.
	FactOpenFailed journal.Kind = "open-failed"

	// FactLooted is the boss-room chest taken.
	FactLooted journal.Kind = "looted"
)

// The verbs.
const (
	// Search is the one door into knowledge this content has. Nothing here
	// evaluates on room entry or proximity — searching is a declared attempt
	// or it does not happen.
	Search world.VerbName = "search"

	// Open is the door's own check. Knowing where it is does not open it.
	Open world.VerbName = "open"

	// Defeat records the captain going down. Combat itself is out of scope;
	// this is the declared outcome, same as UC-1's camp.
	Defeat world.VerbName = "defeat"

	// Loot takes the boss-room chest — the fight's own reward, and no part
	// of recovering the artifact.
	Loot world.VerbName = "loot"
)

// QuestID names the single-run job: recover the artifact and make it out.
const QuestID = "recover-the-artifact"

// Check is one graded attempt the door asks its resolver about: an approach
// and a difficulty. This is deliberately the whole of what a dungeon builder
// supplies for it — witness policy and fact kind are the tomb's own to keep,
// because the two-writer story only holds if every instance of this content
// keeps that promise the same way.
type Check struct {
	// Approach is handed to the resolver untouched.
	Approach journal.Approach

	// Difficulty is handed to the resolver untouched.
	Difficulty int
}

// Config is the tomb's builder form: everything a dungeon builder places when
// it drops this scenario into a region. Every field is required and nothing
// is defaulted — see [New].
type Config struct {
	// Artifact is the item the hidden room holds and the quest is about.
	Artifact journal.EntityID

	// Captain is the monster guarding the hidden room. Defeating them is
	// what turns their knowledge into loot.
	Captain journal.EntityID

	// Find is the door's search check.
	Find Check

	// Open is the door's open check.
	Open Check
}

// ErrNoArtifact reports a config with nothing to recover.
var ErrNoArtifact = errors.New(
	"this tomb needs an artifact — the thing the hidden room holds and the whole quest is about")

// ErrNoCaptain reports a config with nobody guarding the hidden room.
var ErrNoCaptain = errors.New(
	"this tomb needs a captain — who is guarding the hidden room, and whose defeat is what turns their " +
		"knowledge into loot")

// ErrNoFindCheck reports a door with no search check.
var ErrNoFindCheck = errors.New(
	"this tomb's door needs a find check — an approach and a difficulty for searching it out without help")

// ErrNoOpenCheck reports a door with no open check.
var ErrNoOpenCheck = errors.New(
	"this tomb's door needs an open check — an approach and a difficulty for getting through it once " +
		"somebody knows where it is")

// New validates a config and returns the scenario it describes.
//
// Returns [ErrNoArtifact], [ErrNoCaptain], [ErrNoFindCheck], or
// [ErrNoOpenCheck] — each naming exactly the field a dungeon builder left
// blank. Nothing here is defaulted: a tomb with no captain is not "no fight",
// it is a form that was not finished.
func New(cfg Config) (world.Scenario, error) {
	if cfg.Artifact == "" {
		return world.Scenario{}, ErrNoArtifact
	}
	if cfg.Captain == "" {
		return world.Scenario{}, ErrNoCaptain
	}
	if cfg.Find.Approach == "" {
		return world.Scenario{}, ErrNoFindCheck
	}
	if cfg.Open.Approach == "" {
		return world.Scenario{}, ErrNoOpenCheck
	}

	return world.Scenario{
		Graph:  declaration(cfg),
		Verbs:  verbs(cfg),
		Quests: []quest.Template{contract(cfg)},
	}, nil
}

// declaration is the tomb as data: the cast, the door's structure, the two
// flags that are ordinary completion signals, and the concealment that
// keeps a stranger's structural view from showing the hidden room at all.
//
// The room, the artifact inside it, and the door edge are three separate
// declarations, concealed independently — there is no cascade from marking
// one concealed to the others, per the kernel's own second-instance law.
// All three are named together in the same [graph.Pierce] and [graph.Reveal]
// because in this content they always move together, not because the
// kernel requires it: a scenario that wanted the artifact to stay hidden
// after the door opened could reveal the door alone.
//
// There is no reducer for [FactLocationKnown] — that fact only pierces
// concealment (see the package doc for why that is not a graph flag); the
// kernel's own concealment machinery is what a stranger's view now answers
// with, not a hand-rolled journal query.
func declaration(cfg Config) graph.Config {
	return graph.Config{
		Membership: BelongsTo,
		Entities: []graph.Entity{
			{ID: Party, Kind: KindFaction},
			{ID: Finch, Kind: KindPerson},
			{ID: Bram, Kind: KindPerson},
			{ID: Thane, Kind: KindPerson},
			{ID: cfg.Captain, Kind: KindMonster},
			{ID: HiddenRoom, Kind: KindRoom, Concealed: true},
			{ID: BossRoom, Kind: KindRoom},
			{ID: cfg.Artifact, Kind: KindItem, Concealed: true},
		},
		Edges: []graph.Edge{
			{From: Finch, Rel: BelongsTo, To: Party},
			{From: Bram, Rel: BelongsTo, To: Party},
			{From: Thane, Rel: BelongsTo, To: Party},

			// The door, real from birth. Concealed until pierced or revealed
			// — see the Pierces and Reveals below.
			{From: BossRoom, Rel: LeadsTo, To: HiddenRoom, Concealed: true},
		},
		Reducers: []graph.Reducer{
			graph.Raise{On: FactDoorOpened, Flag: Recovered},
			graph.Raise{On: FactLooted, Flag: Looted},
		},
		Pierces: []graph.Pierce{{
			// Search's success and Defeat's Otherwise both write this kind —
			// the "two writers" story — so one pierce covers both.
			On:       FactLocationKnown,
			Entities: []journal.EntityID{HiddenRoom, cfg.Artifact},
			Edges:    []graph.Edge{{From: BossRoom, Rel: LeadsTo, To: HiddenRoom}},
		}},
		Reveals: []graph.Reveal{{
			On:       FactDoorOpened,
			Entities: []journal.EntityID{HiddenRoom, cfg.Artifact},
			Edges:    []graph.Edge{{From: BossRoom, Rel: LeadsTo, To: HiddenRoom}},
		}},
	}
}

// verbs is what anyone here may try. Search and Open are margin-banded,
// graded by the config's checks; Defeat and Loot are declared outcomes, the
// same as UC-1's combat-adjacent verbs.
func verbs(cfg Config) []world.Verb {
	return []world.Verb{
		{
			// The one door into knowledge. Success tells the searcher alone;
			// failure reveals nothing to anybody.
			Name:       Search,
			Approach:   cfg.Find.Approach,
			Difficulty: cfg.Find.Difficulty,
			Outcomes: []world.Band{
				{Emission: world.Emission{Kind: FactLocationKnown, Witness: world.WitnessNobody}},
			},
			Otherwise: world.Emission{Kind: FactSearchFailed, Witness: world.WitnessNobody},
		},
		{
			// Knowing is not entering. Success is the one moment this
			// content shares the door with whoever is standing there.
			Name:       Open,
			Approach:   cfg.Open.Approach,
			Difficulty: cfg.Open.Difficulty,
			Outcomes: []world.Band{
				{Emission: world.Emission{Kind: FactDoorOpened, Witness: world.WitnessBystanders}},
			},
			Otherwise: world.Emission{Kind: FactOpenFailed, Witness: world.WitnessNobody},
		},
		{
			// The captain's knowledge, turned to loot. Whoever was there
			// when he fell learns where the door is, the same fact a
			// successful search would have written.
			Name:      Defeat,
			Otherwise: world.Emission{Kind: FactLocationKnown, Witness: world.WitnessBystanders},
		},
		{
			// The boss room's own reward, and no part of the artifact quest.
			Name:      Loot,
			Otherwise: world.Emission{Kind: FactLooted, Witness: world.WitnessBystanders},
		},
	}
}

// contract is the single-run quest: recover the artifact and make it out.
// One subject, the party, claimed once — the shape UC-2's population
// machinery was always going to need a single-instance case of.
//
// The objective reads the truth view, not the party's own: whether the
// artifact was recovered is bookkeeping the world cannot be mistaken about,
// not something that depends on who happened to be standing there when the
// door opened. A party member's own present is exactly where knowledge of
// the door belongs instead — see [Knows].
func contract(cfg Config) quest.Template {
	return quest.Template{
		ID:       QuestID,
		Name:     "Recover the Artifact",
		Subjects: []journal.EntityID{Party},
		Objectives: []quest.Objective{{
			ID:        "artifact-recovered",
			Predicate: quest.Flagged{Flag: Recovered, Of: cfg.Artifact},
		}},
	}
}

// Knows reports whether the hidden room is visible in an observer's own
// present.
//
// This used to be a hand-rolled journal query — the kernel's structural view
// had no way to answer it before concealment existed (rpg-toolkit#1338's
// finding). It is a one-line wrapper over [graph.State.Visible] now: the
// room, the artifact inside it, and the door edge are all declared
// concealed, pierced by a successful search or the captain's own defeat, and
// revealed for good once a knower gets the door open. Kept as a named
// function rather than inlined at every call site because "knows the hidden
// room" reads as a question about this content, not about the kernel
// mechanism that answers it.
func Knows(w *world.World, observer journal.EntityID) bool {
	return w.View(observer).Visible(HiddenRoom)
}
