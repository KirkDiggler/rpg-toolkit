// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// Who is standing, answered where the sheets are.
//
// TWO WORDS, ON PURPOSE. The composition's capability is called Standing and
// asks who is DOWN; the vocabulary that leaves this seam says DOWNED, and its
// opposite is UP. That is Kirk's ruling (rpg-toolkit#1084): a bare "down" also
// reads as prone, and prone is a posture condition the rulebook tracks that
// this package never gates on. The composition's names are left alone because
// its beat kind is persisted in every stored world, so the translation happens
// here — at [ErrDowned] and in kindOf. Inside this file the
// composition's word is used, because inside this file we are answering the
// composition's question.
//
// The composition asks and the rulebook answers, and this file is the wire
// between them. It is the last piece of the death lane (rpg-toolkit#959): the
// composition holds no hit points and cannot ever hold any (law C1), so it
// takes a capability instead — member IDs in, member IDs out — and the session
// is the only layer that both knows what a sheet is and holds every one of
// them for the call in progress.
//
// NO RULE LIVES HERE. Resolution returns the root provider's complete
// participation answer, derived from canonical standing/life-state facts. This
// file finds stored records; participation.go maps the typed answer without
// recomputing a hit-point threshold.

// standingSeam answers the composition's standing question out of the sheets
// this one verb holds.
//
// COMPILE-TIME PROOF that the seam is the composition's participation
// capability. It is the one answer to who is down: Down on the assessment.
var _ encounter.Participation = standingSeam{}

// # It is per call, and it has to be
//
// The context rides on the struct, which is ordinarily a smell and is the right
// shape here: the capability's method takes no context, because the composition
// that calls it has none to give — it is running inside a verb it knows nothing
// about. So the verb's own context is captured when the seam is built, and the
// seam is built fresh for every verb (S1, S4). One never outlives the call that
// made it, and there is nothing here to go stale between two of them.
//
// # Re-read on write, not on every consult
//
// The composition asks again rather than remembering (its
// [encounter.Standing] doc says why), and this side must not quietly undo that
// by remembering on its behalf: a swing writes a damaged sheet part-way
// through its own verb, so an answer cached at the top of the call would be
// answering about a world that no longer exists.
//
// A player is re-read at every consult: a repository read per player per
// consult. A MONSTER's answer is kept for the verb and forgotten the moment
// the verb writes that monster's sheet ([participationMemo], rpg-project#892):
// a hundred-strong garrison re-attached at every patrol step was the cost of
// the raise, and a step writes no sheet. The law the paragraph above states
// still holds — a written sheet is what the next consult reads — because the
// write is what forgets.
type standingSeam struct {
	// ctx is the verb's own. See the type's godoc.
	ctx context.Context

	// sheets is the verb's sheet store, for the members the host owns.
	sheets sheetStore

	// kinds is the authoritative encounter roster classification copied at the
	// load/setup boundary. One assessment reads this one snapshot and never
	// guesses kind from whichever storage record happens to answer.
	kinds map[string]encounter.MemberKind

	// data is the session record, ALIASED rather than copied, so a sheet this
	// verb has already written back is what the next consult reads.
	//
	// Nil is legal and means "no session-scoped sheets", which is what a load
	// with no session record behind it has: nothing exists yet to hold anything.
	data *SessionData

	// live is the encounter this verb loaded, filled in once the load that
	// needs these seams has returned (the chicken-and-egg [strikerSeam]
	// names). Shared by pointer, so every copy of this seam — the sheet seam
	// beside it included — reads the same one. Read only by
	// [sheetSeam.factsOf], for the one fact the session answers from the
	// encounter's own state: whether a member is sneaking (design R8).
	live *liveEncounter

	// memo is this verb's kept monster participation answers
	// ([participationMemo]), shared by pointer so every copy of the seam —
	// the sheet, equipment and conditions seams beside it included — keeps
	// and forgets the same answers. Nil keeps nothing.
	memo *participationMemo
}

// liveEncounter is the slot [standingSeam.live] points at: nil until the
// verb's load returns, then the encounter that load built, replaced when a
// verb adopts a world back from a resolution ([Manager.adopt]).
type liveEncounter struct {
	enc sneakingReader
}

// sneakingReader is the one question the sheet seam asks the live encounter.
// [encounter.Encounter] answers it; a test may answer it without building a
// world.
type sneakingReader interface {
	Sneaking(member encounter.MemberID) bool
}

// sneaking reports whether the verb's loaded encounter holds member sneaking.
// False before the load has returned: a world still being built has asked
// nobody to sneak yet.
func (l *liveEncounter) sneaking(member encounter.MemberID) bool {
	return l != nil && l.enc != nil && l.enc.Sneaking(member)
}

// standingFor builds the participation capability for one verb. Returning the
// concrete value keeps the richer answer available to session projections.
func (m *Manager) standingFor(
	ctx context.Context, data *SessionData, kinds map[string]encounter.MemberKind,
) standingSeam {
	copiedKinds := make(map[string]encounter.MemberKind, len(kinds))
	for id, kind := range kinds {
		copiedKinds[id] = kind
	}
	return standingSeam{ctx: ctx, sheets: m.sheetsFor(nil), data: data, kinds: copiedKinds, live: &liveEncounter{}, memo: newParticipationMemo()}
}

// Assess returns the provider-derived scheduling/contact answer, Down
// included. It is the only answer to who is down, so nothing here can drift
// into a second hit-point or life-state rule.
//
// ONLY ABOUT WHO WAS ASKED, and that is structural rather than a filter
// applied afterwards: the loop is over the question. The composition refuses
// an answer naming somebody who is not a member rather than ignoring it, and
// the store really does hold strangers. An error aborts whatever verb was
// running, atomically (R5): a world that cannot find out who is down does not
// half-act on a guess.
func (s standingSeam) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	snapshot, err := s.participation(members)
	if err != nil {
		return nil, err
	}
	return snapshot.assessment, nil
}

// recordsFor gathers the stored record behind each member, split by the store
// it came from.
//
// THE FETCH HALF STAYS HERE, and only the fetch half. Finding sheets is what
// this seam is for — it is the one layer that both knows what a sheet is and
// holds every one of them for the call in progress. Reconstituting them is not:
// that needs a bus, and a bus in this package is a fold waiting to happen.
//
// # A player with no sheet is refused
//
// A player the roster names and the host's store does not hold is
// [ErrNoCharacter], the store's one answer for an absent sheet
// (rpg-project#542): answering "conscious" for a body nobody can read would
// keep a fight running around it, and answering "down" would kill it because
// a database blinked. Exit and End do not consult standing, so a run with a
// missing sheet can still be left and closed.
//
// A sheetless MONSTER (authored, never spawned) is still in neither provider
// input list, and participation.go supplies its one Conscious compatibility
// fact; the sheet seam refuses it the first time the world asks how it moves.
func (s standingSeam) recordsFor(
	members []encounter.MemberID,
) (characters, monsters []resolution.Participant, err error) {
	for _, id := range members {
		name := string(id)
		kind, ok := s.kinds[name]
		if !ok {
			return nil, nil, fmt.Errorf(
				"participation member %q has no roster kind: %w", name, ErrInvalidSession)
		}

		switch kind {
		case encounter.KindWorld:
			// Explicitly neutral. Never consult either combatant store.
			continue
		case encounter.KindMonster:
			// A monster can only come from the session's NPC sheets. An authored
			// sheetless monster keeps the compatibility fallback projected later;
			// it never falls through to a same-ID host character.
			if sheet, found := npcSheet(s.data, name); found {
				monsters = append(monsters, resolution.Participant{Monster: sheet})
			}
			continue
		case encounter.KindPlayer:
			// A player can only come from the host character repository. A stale
			// same-ID monster sheet cannot change the authoritative roster kind.
		default:
			return nil, nil, fmt.Errorf(
				"participation member %q has unknown roster kind %q: %w",
				name, kind, ErrInvalidSession)
		}

		data, fetchErr := s.sheets.load(s.ctx, "character", name)
		if fetchErr != nil {
			return nil, nil, fetchErr
		}

		characters = append(characters, resolution.Participant{Character: data})
	}

	return characters, monsters, nil
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

// refuseIfDown refuses a normal action whose ACTOR is DOWNED: at zero hit
// points. It answers [ErrDowned], which is where the word is explained.
//
// # Which verbs, and why only those
//
// The two normal actions a downed member could otherwise take are
// [Manager.Attack] and [Manager.Move]. Downed no longer means absent from a
// fight's initiative: provider participation retains Dying with a waiting slot
// for an explicit Death Save and Stabilized with an auto-pass slot; only
// Dead/Defeated removes the slot. This gate therefore remains necessary on the
// turn clock as well as in free roam, where there is no turn order at all. The
// composition deliberately did not invent this refusal; it is ruled here,
// where the sheets are, and it is one refusal covering both clocks.
//
// # And which are deliberately left open
//
// DeathSave and EndTurn are the explicit ways a retained downed slot proceeds,
// so neither uses this normal-action gate. The reads — Where, View, Story,
// Status, and Turn — all answer about a downed member, because a downed member
// is still a member (ruled fork (a) on rpg-toolkit#959) and a client that could
// not ask where it fell could not render the moment it happened. Recording an
// outcome ABOUT a downed member remains legal too — the killing stroke is
// itself a beat about somebody who is now down. Dying and Stabilized targets
// remain provider-approved Attack candidates; Dead/Defeated targets do not.
//
// It asks about ONE member, which is the whole question. The capability is
// roster-scoped by construction, so a single-ID question gets a single-ID
// answer and nothing else is loaded.
func refuseIfDown(scope *writeScope, role, id string) error {
	down, err := standingSet(scope.standing, []encounter.MemberID{encounter.MemberID(id)})
	if err != nil {
		return err
	}
	if !down[id] {
		return nil
	}

	return fmt.Errorf("%s %q: %w", role, id, ErrDowned)
}
