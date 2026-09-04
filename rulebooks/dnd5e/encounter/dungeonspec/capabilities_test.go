// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// The three capabilities every encounter requires, supplied explicitly.
//
// Stated out loud rather than defaulted, which is the whole point of them being
// required (rpg-toolkit#1033 — capabilities are supplied, never defaulted). This
// package's scenes are about GEOMETRY: what the compiler built, and whether a
// player can walk it. So each of these answers the least interesting thing it
// can, and says so — an unbounded sight range and a roster where nobody is down
// mean no assertion in tomb_test.go is secretly about light or hit points.
//
// They are copies of the encounter package's own test fixtures rather than
// shared ones, because test helpers do not cross a package boundary and
// exporting them to share here would put a test-only surface on a published
// module.

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// unlimitedSight is far enough that no scene here is bounded by it.
const unlimitedSight = 1_000_000

// everyoneSeesTheWholeMap installs a sight model with no distance term, so
// every sighting in these scenes is decided by geometry alone — walls, doors,
// props and void — which is exactly what the world-model wave was about.
type everyoneSeesTheWholeMap struct{}

func (everyoneSeesTheWholeMap) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
	}

	return out, nil
}

// everyoneStanding reports nobody down. The tomb's scenes never drop anybody,
// and a body that was silently treated as an enemy would form fights this file
// does not mean to be about.
type everyoneStanding struct{}

func (everyoneStanding) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

func (everyoneStanding) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		assessment.Members = append(assessment.Members, encounter.MemberParticipation{
			Member: id, Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait,
		})
	}
	return assessment, nil
}

// orderAsGiven returns the roster untouched. These scenes assert that a fight
// FORMED, never what order it formed in, and a shuffle would make the assertion
// depend on a roll nobody is testing.
type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

// passDriver ends every unplayed member's turn with no other effect. These
// scenes are about geometry, not monster behaviour, so an unplayed member
// simply passes rather than this package inventing an opinion about what one
// should do (rpg-toolkit#1162).
type passDriver struct{}

func (passDriver) Act(encounter.MonsterView) (encounter.TurnIntent, error) {
	return encounter.Pass{}, nil
}

// noAttacksExpected is this package's Striker capability. These scenes are
// about geometry, not combat, and passDriver never returns an Attack intent
// — so this is never actually called; it exists only because the
// capability is required (rpg-toolkit#1033, rpg-project#254).
type noAttacksExpected struct{}

func (noAttacksExpected) Strike(
	context.Context, *encounter.Encounter, encounter.MemberID, encounter.MemberID, core.Ref,
) error {
	return errors.New("dungeonspec_test: no scene here ever attacks")
}

// quietAnnouncer hears every boundary and does nothing. These scenes are about
// geometry, not about what a turn boundary means to a condition — but unlike
// noAttacksExpected it really is called, so it succeeds rather than refusing.
type quietAnnouncer struct{}

func (quietAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
}

// nothingIsEverFound resolves every find check as a failure. These scenes are
// about GEOMETRY, and a concealed dungeon needs a resolver at the door
// whether or not anything in the scene searches — so this one is supplied and
// says the least interesting thing it can, exactly as its three siblings
// above do. What a DC means is the rulebook's; a scene about walls has no
// business deciding it either way.
type nothingIsEverFound struct{}

func (nothingIsEverFound) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{Beaten: false}, nil
}

// nobodyPerceivesAnything answers that no member perceives any open concealed
// door — the counterpart to nothingIsEverFound, and required for the same
// reason. A concealed dungeon in this package is a compile to be inspected,
// not a run to be played, so the atlas these tests read is the one nobody has
// found anything in yet.
type nobodyPerceivesAnything struct{}

func (nobodyPerceivesAnything) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, nil
}
