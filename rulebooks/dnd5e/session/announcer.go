// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// announcerSeam is this package's real encounter.Announcer, bound to the live
// scope.enc a write verb is already operating on.
//
// A STRUCT HOLDING A *writeScope RATHER THAN A CLOSURE OVER ONE, for the reason
// strikerSeam's own doc gives one file over: encounter.LoadEncounter needs the
// capability to construct the very *encounter.Encounter this seam's own
// Announce method later receives as a parameter, so scope is allocated with its
// data (and later enc) fields set in the order openForWrite needs, and this
// seam only ever reads scope AT CALL TIME.
type announcerSeam struct {
	m     *Manager
	scope *writeScope
}

// compile-time proof the seam satisfies what it is handed to.
var _ encounter.Announcer = announcerSeam{}

// Announce turns one clock advance's boundaries into an interaction.
//
// This is the sentence the whole slice is about. The composition notices a turn
// boundary and cannot publish one — it holds no bus (ADR-0038), and the clock
// beneath it returns milestones as values and never publishes. This package
// holds no bus either. resolution does, so a boundary becomes a resolution: the
// whole cast is attached (R3), the crossings are published in the order they
// happened, everything is torn down (R5), and whoever changed comes back dirty.
//
// The effects that wait for this are scoped to a turn — dodging lapsing at
// its owner's next turn, disengaging and raging ending at theirs, and
// sneak-attack-used clearing. Death Saves are not boundary side effects: the
// session offers and executes them only through the explicit declaration verb.
//
// NEVER ADOPTS A NEW *encounter.Encounter, exactly as strikerSeam does not:
// enc is the SAME live object the composition is mid-verb over — often mid-
// drive-loop — and swapping it out from inside a call enc itself made would
// orphan that loop. The world resolution returns is read for its dirty sheets
// alone and otherwise discarded, which is sound for the same reason it is
// sound there: conditions live on SHEETS, not on the encounter's own
// story-and-position state.
//
// No cost. A boundary has no declaring actor and nothing to pay: time is what
// caused it. resolution's nil Cost is a free action, which is what this is.
func (a announcerSeam) Announce(
	ctx context.Context, enc *encounter.Encounter, crossed []encounter.Boundary,
) error {
	machine, err := resolution.NewBoundary(&resolution.BoundaryInput{Crossed: crossed})
	if err != nil {
		return fmt.Errorf("announce: %w", err)
	}

	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("announce: %w", translate(err))
	}

	cast, err := a.boundaryCast(ctx, roster)
	if err != nil {
		return fmt.Errorf("announce: %w", err)
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	world := enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        world,
		Participants: cast,
		Initiative:   a.m.initiative,
		Standing:     a.scope.standing,
		Sight:        &sightSeam{members: append([]encounter.MemberData(nil), world.Members...)},
		TurnDriver:   a.m.turnDriver,
		// The concealment pair (rpg-toolkit#1378), bound to the same live
		// scope openForWrite and adopt bind — the one-seam consistency law:
		// a concealed world refuses to reconstruct without them, and
		// resolution carries them without consulting either, since no verb
		// runs inside an interaction.
		CheckResolver: checkSeam(a),
		Witness:       witnessSeam{scope: a.scope},
		Machine:       machine,
		Roller:        &diceSeam{roller: a.m.dice},
	})
	if err != nil {
		return fmt.Errorf("announce: %w", translateResolution(err))
	}

	return a.m.saveDirty(ctx, a.scope, out)
}

// boundaryCast gathers everyone in the fight, and TOLERATES a member the
// repository does not hold.
//
// R3 otherwise: everyone in, applicability is the effect's own predicate. A
// boundary is the case where that matters most — which member a turn belongs to
// is exactly the question each condition answers for itself, and deciding it out
// here would put a rule in the wiring.
//
// # Why this is not castFor
//
// castFor refuses a character the repository cannot produce, and that is right
// for a swing: a cast with a hole in it might be missing the very effect that
// would have changed the number. A clock advance is different in a way that
// matters — it is not anybody's declared action, it is TIME, and it happens to
// everyone at once. Refusing it would freeze the whole fight for every member
// because one character's sheet is missing.
//
// The stack already made that call and pinned it:
// TestUnreadableCharacterBlocksAttackAndMoveButNotEndTurn says an unreadable
// character blocks Attack and Move — there is nothing to compile a swing
// from — while EndTurn, "governed by the clock alone", stays available. Failing
// the announcement would quietly retract that.
//
// What it costs is honest and worth stating: an absent character's turn-scoped
// conditions do not expire on this boundary. That is a real gap, and it is
// bounded by the fact that the same character can neither attack nor move —
// they are already not participating. Nothing is DELETED by the omission:
// saveDirty writes back only what resolution hands over, so a sheet that never
// went in cannot come out empty.
//
// ONLY ErrNoCharacter is tolerated. A repository that fails some other way — or
// reports success with no data (ErrBadRepository) — is a broken repository
// rather than an absent member, and that still fails the verb rather than
// quietly shrinking the cast.
func (a announcerSeam) boundaryCast(
	ctx context.Context, roster []encounter.Member,
) ([]resolution.Participant, error) {
	npcs := map[string]*monster.Data{}
	for i := range a.scope.data.NPCs {
		npcs[a.scope.data.NPCs[i].ID] = &a.scope.data.NPCs[i]
	}

	cast := make([]resolution.Participant, 0, len(roster))
	for _, member := range roster {
		id := string(member.ID)
		if member.Kind == encounter.MemberKind(KindMonster) {
			sheet, ok := npcs[id]
			if !ok {
				continue // content with no stored sheet contributes nothing
			}
			cast = append(cast, resolution.Participant{Monster: sheet})
			continue
		}

		if member.Kind == encounter.MemberKind(KindWorld) {
			continue // placed world NPC — no sheet, contributes nothing to the cast
		}

		data, err := a.m.fetchCharacterData(ctx, "participant", id)
		if err != nil {
			if errors.Is(err, ErrNoCharacter) {
				continue
			}
			return nil, err
		}
		cast = append(cast, resolution.Participant{Character: data})
	}
	return cast, nil
}
