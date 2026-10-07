// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// How fast everyone is, what they swing and how far they see, answered where
// the sheets are (rpg-project#538, "Sheet facts asked at use time").
//
// The composition holds no speed, sight, attack or targeting and cannot hold
// any (law C1, R6), so it asks through two capabilities — [encounter.Sheets]
// for speed, attacks and targeting, [encounter.Sight] for how far — and this
// file answers both from the sheets the verb holds: the host's character
// store for a player, the session's own stat blocks for a monster, and the
// session's recorded content for a world NPC. It is the same wire
// [standingSeam] and [equipmentSeam] are, built the same way.
//
// # One owner per fact, no copies
//
// Join, Spawn and PlaceNPC hand the composition none of these facts; it asks
// at the moment it paces a walk, budgets a driven turn, tests reach, builds a
// driver's view or refreshes a percept. A level gained, a weapon swapped or a
// stat block changed is read at the next ask without any verb owning a
// refresh. This package's own push budget (castmove.go) asks the same answer.

// sheetSeam answers the composition's sheet and sight questions out of the
// sheets this one verb holds.
//
// COMPILE-TIME PROOF that this package satisfies the composition's contract.
var _ encounter.Sheets = sheetSeam{}

// # It is per call, and it has to be
//
// The context rides on the struct for [standingSeam]'s reason: the
// capability's methods take no context because the composition calling them
// has none to give, so the verb's own is captured when the seam is built, and
// the seam is built fresh for every verb (S1, S4).
//
// # No cache, deliberately
//
// Every consult re-reads. A verb that equips a weapon or levels a character
// writes the sheet part-way through itself, and an answer remembered from the
// top of the call would describe a sheet that no longer exists — the exact
// copy rpg-project#538 retired from the composition, moved one layer up.
//
// THE SAME FIELDS AS [standingSeam], AND NONE OF ITS METHODS: a defined type
// over it, so the two capabilities share where the sheets are and which verb
// is asking while staying separate contracts answering separate questions.
type sheetSeam standingSeam

// sheetsBeside builds the sheet capability from the verb-scoped facts a
// [standingSeam] already carries, for [equipmentBeside]'s reason: the roster
// kind snapshot is the verb's one classification, and a second snapshot could
// disagree with it. The kinds map is shared, not copied, so a member [place]
// classifies mid-verb is known here before its own Join asks about it.
func sheetsBeside(s standingSeam) sheetSeam {
	return sheetSeam(s)
}

// Sheets reports each given member's speed, attacks and targeting, read off
// its sheet at this moment.
//
//   - A player answers from the resolution door's projection of its record:
//     walking speed from its race, and its attack from the main-hand fold
//     (an empty hand is an unarmed strike) — the same projection Join reads.
//     A player has no targeting strategy; empty is what its sheet says.
//   - A monster answers from its stat block: SpeedData.Walk, every executable
//     action definition in author order, and the targeting word the author's
//     placement wrote onto it at spawn.
//   - A world NPC is stationary and non-acting by construction (design N4):
//     its recorded content states no speed and no actions, so the true
//     answer from that content is zero speed and nothing to swing. Answered
//     only when its content is recorded.
//
// ONLY ABOUT WHO WAS ASKED: the loop is over the question, because the
// composition refuses a stranger in the answer (ErrNotMember).
//
// A member the verb holds no sheet for is refused, never answered with zero
// (zero is a real speed): [ErrNoCharacter] for a player whose record is not
// found, [ErrNoSheet] for a monster or world NPC the session never recorded.
// Any error aborts whatever verb was running, atomically (R5).
func (s sheetSeam) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		facts, err := s.factsOf(id)
		if err != nil {
			return nil, err
		}
		out[id] = facts
	}

	return out, nil
}

// factsOf reads one member's sheet facts.
func (s sheetSeam) factsOf(id encounter.MemberID) (encounter.SheetFacts, error) {
	held, err := s.sheetOf(id)
	if err != nil {
		return encounter.SheetFacts{}, err
	}

	switch {
	case held.character != nil:
		projected, projectErr := projectCharacter(s.ctx, string(id), held.character)
		if projectErr != nil {
			return encounter.SheetFacts{}, projectErr
		}
		actions, actionsErr := memberActionsFrom(projected.MainHand)
		if actionsErr != nil {
			return encounter.SheetFacts{}, fmt.Errorf("character %q: %w", id, actionsErr)
		}
		return encounter.SheetFacts{SpeedFeet: projected.Sheet.SpeedFeet, Actions: actions}, nil
	case held.monster != nil:
		return encounter.SheetFacts{
			SpeedFeet: held.monster.Speed.Walk,
			Actions:   memberActionsFromMonster(held.monster.Actions),
			Targeting: held.monster.Targeting.String(),
		}, nil
	default:
		return encounter.SheetFacts{}, nil
	}
}

// heldSheet is the one record a member's sheet facts are read from: exactly
// one of character or monster is set, or neither for a world NPC whose
// content was found.
type heldSheet struct {
	character *character.Data
	monster   *monster.Data
}

// sheetOf finds the sheet behind one member, by the verb's authoritative
// roster kind — never by guessing from whichever store happens to answer.
//
// It refuses, by name, a member it holds no sheet for: a player whose record
// the host's store does not have ([ErrNoCharacter]), and a monster or world
// NPC the session never recorded ([ErrNoSheet]). This is where it differs
// from [standingSeam.recordsFor], which tolerates a sheetless member because
// "is it down" has a true answer for a body nobody can read; "how fast is it"
// does not.
func (s sheetSeam) sheetOf(id encounter.MemberID) (heldSheet, error) {
	name := string(id)
	kind, ok := s.kinds[name]
	if !ok {
		return heldSheet{}, fmt.Errorf("member %q has no roster kind: %w", name, ErrInvalidSession)
	}

	switch kind {
	case encounter.KindWorld:
		if s.data == nil {
			return heldSheet{}, fmt.Errorf("world npc %q: %w", name, ErrNoSheet)
		}
		if _, err := findWorldNPCIndex(s.data, name); err != nil {
			return heldSheet{}, err
		}
		return heldSheet{}, nil
	case encounter.KindMonster:
		sheet, found := npcSheet(s.data, name)
		if !found {
			return heldSheet{}, fmt.Errorf("monster %q: %w", name, ErrNoSheet)
		}
		return heldSheet{monster: sheet}, nil
	case encounter.KindPlayer:
	default:
		return heldSheet{}, fmt.Errorf("member %q has unknown roster kind %q: %w", name, kind, ErrInvalidSession)
	}

	data, err := s.chars.GetCharacter(s.ctx, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return heldSheet{}, fmt.Errorf("character %q: %w", name, ErrNoCharacter)
		}
		return heldSheet{}, err
	}
	if data == nil {
		return heldSheet{}, fmt.Errorf(
			"character %q: GetCharacter reported success with no data: %w", name, ErrBadRepository)
	}
	if data.ID != name {
		return heldSheet{}, fmt.Errorf(
			"character %q: GetCharacter returned %q instead: %w", name, data.ID, ErrBadRepository)
	}

	return heldSheet{character: data}, nil
}
