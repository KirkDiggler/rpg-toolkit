// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// Who is standing, answered where the sheets are.
//
// The composition asks and the rulebook answers, and this file is the wire
// between them. It is the last piece of the death lane (rpg-toolkit#959): the
// composition holds no hit points and cannot ever hold any (law C1), so it
// takes a capability instead — member IDs in, member IDs out — and the session
// is the only layer that both knows what a sheet is and holds every one of
// them for the call in progress.
//
// NO RULE LIVES HERE. The whole of the arithmetic is [combat.IsDown], which is
// where a monster's Undead Fortitude (rpg-toolkit#977) will change the answer
// without a line of this file moving. What this file does is find sheets.

// standingSeam answers the composition's standing question out of the sheets
// this one verb holds.
//
// # It is per call, and it has to be
//
// The context rides on the struct, which is ordinarily a smell and is the right
// shape here: the capability's method takes no context, because the composition
// that calls it has none to give — it is running inside a verb it knows nothing
// about. So the verb's own context is captured when the seam is built, and the
// seam is built fresh for every verb (S1, S4). One never outlives the call that
// made it, and there is nothing here to go stale between two of them.
//
// # No cache, deliberately
//
// Every consult re-reads. The composition asks again rather than remembering
// (its [encounter.Standing] doc says why), and this side must not quietly undo
// that by remembering on its behalf: a swing writes a damaged sheet part-way
// through its own verb, so an answer cached at the top of the call would be
// answering about a world that no longer exists. The cost is a repository read
// per player per consult, and a walk consults once per step — real, measured in
// key-value gets, and cheaper than the class of bug a cache buys.
type standingSeam struct {
	// ctx is the verb's own. See the type's godoc.
	ctx context.Context

	// chars is the host's sheet store, for the members it owns.
	chars CharacterRepository

	// data is the session record, ALIASED rather than copied, so a sheet this
	// verb has already written back is what the next consult reads.
	//
	// Nil is legal and means "no session-scoped sheets", which is what
	// StartSession's validation load has: it is proving an authored blob can be
	// reconstituted, and no session record exists yet to hold anything.
	data *SessionData
}

// standingFor builds the capability for one verb.
func (m *Manager) standingFor(ctx context.Context, data *SessionData) encounter.Standing {
	return standingSeam{ctx: ctx, chars: m.characters, data: data}
}

// Standing reports which of the given members are down.
//
// ONLY ABOUT WHO WAS ASKED, and that is structural rather than a filter applied
// afterwards: the loop is over the question. It matters because the composition
// refuses an answer naming somebody who is not a member (ErrNotMember) rather
// than ignoring it, so a capability that replied out of its whole sheet store
// would abort every verb in the session — and the store really does hold
// strangers. A spawned monster's sheet stays in the session record after its
// member leaves, and [SessionData] is a blob a host can seed.
//
// An error aborts whatever verb was running, atomically (R5). A world that
// cannot find out who is standing does not half-act on a guess, and neither
// available guess is safe: reading an unreachable store as "standing" runs the
// fight on against sheets nobody can read, and reading it as "down" kills a
// character because a database blinked.
func (s standingSeam) Standing(members []encounter.MemberID) ([]encounter.MemberID, error) {
	var down []encounter.MemberID

	for _, id := range members {
		sheet, err := s.sheetOf(string(id))
		if err != nil {
			return nil, err
		}
		if sheet == nil {
			continue
		}
		if combat.IsDown(sheet) {
			down = append(down, id)
		}
	}

	return down, nil
}

// sheetOf finds one member's sheet, wherever the session keeps it, and reports
// (nil, nil) when there is none.
//
// The two stores are the two ways a member enters a session, and the split is
// by where the data comes from rather than by what the thing is — the same
// distinction [instantiate] draws. A monster is INSTANTIATED and its sheet is
// session-scoped, so it is in the session record. A character is LOADED and the
// host owns it, so it is behind the repository.
//
// The NPC list is asked first, and the order is not arbitrary: it is the only
// one that needs no knowledge of a member's KIND. This seam is handed bare IDs,
// by a composition that has not finished loading when the seam is built, so it
// cannot consult a roster — and a lookup that tried to would be asking the
// encounter a question while the encounter is asking it one.
//
// # No sheet, no death
//
// A member neither store answers for is reported STANDING. There is nothing to
// read hit points off, and that is an ordinary state rather than a defect:
// authored content placed straight into a world has no sheet until something
// spawns it, which is exactly what every tomb fixture's monsters are, and what
// [Manager.Attack] already refuses BY NAME when somebody swings at one
// (ErrNoSheet). Answering "down" instead would kill every authored monster in
// the toolkit the moment anybody looked at one; answering with an error would
// make those worlds unplayable. Neither is a rule this package gets to write.
//
// The two failure arms are the ones where the store answered and the answer was
// unusable, which is a different thing from having nothing to say.
func (s standingSeam) sheetOf(id string) (combat.Combatant, error) {
	if sheet, ok := npcSheet(s.data, id); ok {
		built, err := monster.Load(s.ctx, sheet)
		if err != nil {
			// A stored NPC sheet that will not reconstitute is corrupt SESSION
			// state — this record is the only thing that could have written it —
			// so it is reported as that rather than as a character problem or as
			// a broken world. The reason rides as TEXT, never as a chain: the
			// rulebook reports through rpgerr and S2 is not a promise this
			// package delegates (rpg-toolkit#1066).
			return nil, fmt.Errorf("npc %q: %w: %v", id, ErrInvalidSession, err)
		}

		return built, nil
	}

	data, err := s.chars.GetCharacter(s.ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf(
			"character %q: GetCharacter reported success with no data: %w", id, ErrBadRepository)
	}

	// THE LENIENT LOADER, and the choice is load-bearing.
	//
	// character.Load is the strict one: it refuses a sheet carrying a condition
	// this build cannot route, which is right where compileAttack uses it,
	// because a swing hands sheets BACK to be persisted and an effect dropped
	// on the way in is an effect deleted on the way out (rpg-toolkit#985).
	//
	// Nothing is written back from here. This sheet is reconstituted to be
	// asked one question and dropped, so strictness buys nothing and costs
	// everything: a player carrying a homebrew condition this build does not
	// know can join today (TestACorruptConditionIsDroppedRatherThanRejected
	// pins that), and refusing to read their hit points would abort every verb
	// in their session over a condition nobody is asking about.
	//
	// So the standing question reads a sheet exactly as leniently as the join
	// that admitted it. A bus is required by this loader and is thrown away
	// with the sheet; Cleanup is not called, for the reason at the top of
	// entities.go — its first act is to drop the conditions this seam is
	// deliberately tolerating.
	loaded, err := character.LoadFromData(s.ctx, data, newCallBus())
	if err != nil {
		return nil, fmt.Errorf("character %q: %w: %v", id, ErrBadCharacter, err)
	}

	return loaded, nil
}

// npcSheet finds a session-scoped sheet by member ID.
//
// Returns a pointer INTO the record rather than a copy, so a verb that has
// already written damage back sees it on the next consult.
func npcSheet(data *SessionData, id string) (*monster.Data, bool) {
	if data == nil {
		return nil, false
	}
	for i := range data.NPCs {
		if data.NPCs[i].ID == id {
			return &data.NPCs[i], true
		}
	}

	return nil, false
}

// refuseIfDown refuses a verb whose ACTOR is at zero hit points.
//
// # Which verbs, and why only those
//
// The two where a down member could still act: [Manager.Attack] and
// [Manager.Move]. Inside a fight the swing already stops without a gate,
// because the composition splices a body out of the turn order and the seam
// above has nothing left to offer it (rpg-toolkit#1077). Free roam has no turn
// order, so the same body can still walk, and can still initiate — which is
// rpg-toolkit#845's shape reproduced on the new stack. The composition
// deliberately did not invent this refusal; it is ruled here, where the sheets
// are, and it is one refusal covering both clocks rather than a rule that only
// works when somebody happens to be in a fight.
//
// # And which are deliberately left open
//
// Not the reads: Where, View, Story, Status and Turn all answer about a body,
// because a body is still a member (ruled fork (a) on rpg-toolkit#959) and a
// client that could not ask where its own corpse is could not render the moment
// it fell. Not recording an outcome ABOUT a down member either — the killing
// stroke is itself a beat about somebody who is now down. Not a down TARGET:
// swinging at a body may be narratively silly, and refusing it is a different
// ruling that nobody has made.
//
// It asks about ONE member, which is the whole question. The capability is
// roster-scoped by construction, so a single-ID question gets a single-ID
// answer and nothing else is loaded.
func refuseIfDown(scope *writeScope, role, id string) error {
	down, err := scope.standing.Standing([]encounter.MemberID{encounter.MemberID(id)})
	if err != nil {
		return err
	}
	if len(down) == 0 {
		return nil
	}

	return fmt.Errorf("%s %q: %w", role, id, ErrDown)
}
