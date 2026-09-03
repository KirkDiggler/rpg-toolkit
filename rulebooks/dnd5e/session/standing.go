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

// Who is standing, answered where the sheets are.
//
// TWO WORDS, ON PURPOSE. The composition's capability is called Standing and
// asks who is DOWN; the vocabulary that leaves this seam says DOWNED, and its
// opposite is UP. That is Kirk's ruling (rpg-toolkit#1084): a bare "down" also
// reads as prone, and prone is a posture condition the rulebook tracks that
// this package never gates on. The composition's names are left alone because
// its beat kind is persisted in every stored world, so the translation happens
// here — at [ErrDowned], at [EventDowned], and in kindOf. Inside this file the
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
// COMPILE-TIME PROOF that the seam carries the migration bridge: the
// composition's constructors demand a Standing value that also assesses
// participation (encounter.StandingWithParticipation), and this package's one
// seam satisfies both from the same sheet lookup.
var _ encounter.StandingWithParticipation = standingSeam{}

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

// standingFor builds the dual Standing/Participation capability for one verb.
// Returning the concrete value keeps the richer answer available to session
// projections while assignments to encounter.Standing retain the same dynamic
// value for compatibility-shaped constructor fields.
func (m *Manager) standingFor(ctx context.Context, data *SessionData) standingSeam {
	return standingSeam{ctx: ctx, chars: m.characters, data: data}
}

// Standing reports which of the given members are down — downed, in the
// vocabulary this seam publishes — by selecting Down from the same rich
// provider assessment Assess returns. See the note at the top of this file.
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
// available guess is safe: reading an unreachable store as UP runs the fight on
// against sheets nobody can read, and reading it as DOWNED kills a character
// because a database blinked.
func (s standingSeam) Standing(members []encounter.MemberID) ([]encounter.MemberID, error) {
	snapshot, err := s.participation(members)
	if err != nil {
		return nil, err
	}
	var down []encounter.MemberID
	for _, member := range snapshot.assessment.Members {
		if member.Down {
			down = append(down, member.Member)
		}
	}
	return down, nil
}

// Assess returns the provider-derived scheduling/contact answer. Standing and
// Assess both delegate to participation, so neither can drift into a second
// hit-point or life-state rule.
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
// # No sheet, no death
//
// A member neither store answers for is in neither list, so nothing is asked
// about it and nothing is reported. That is an ordinary state rather than a
// defect: authored content placed straight into a world has no sheet until
// something spawns it, which is exactly what every tomb fixture's monsters are.
// Answering DOWNED instead would kill every authored monster in the toolkit the
// moment anybody looked at one; answering with an error would make those worlds
// unplayable. Neither is a rule this package gets to write.
func (s standingSeam) recordsFor(
	members []encounter.MemberID,
) (
	characters, monsters []resolution.Participant,
	worldNPCs map[string]bool,
	err error,
) {
	worldNPCs = worldNPCSet(s.data)
	for _, id := range members {
		name := string(id)

		// KindWorld identity wins before either combatant store. A host may own
		// a character with the same ID, and malformed/session-seeded data may
		// also retain an NPC sheet collision; neither turns the placed bystander
		// into a combatant.
		if worldNPCs[name] {
			continue
		}

		if sheet, ok := npcSheet(s.data, name); ok {
			monsters = append(monsters, resolution.Participant{Monster: sheet})
			continue
		}

		data, fetchErr := s.chars.GetCharacter(s.ctx, name)
		if fetchErr != nil {
			if errors.Is(fetchErr, ErrNotFound) {
				continue
			}

			return nil, nil, nil, fetchErr
		}
		if data == nil {
			return nil, nil, nil, fmt.Errorf(
				"character %q: GetCharacter reported success with no data: %w", name, ErrBadRepository)
		}

		characters = append(characters, resolution.Participant{Character: data})
	}

	return characters, monsters, worldNPCs, nil
}

// worldNPCSet indexes the session's explicit non-combatant identities. It
// reads no content and derives no rule; PlacedWorldNPC is already the
// session-owned statement that these member IDs are KindWorld bystanders.
func worldNPCSet(data *SessionData) map[string]bool {
	out := make(map[string]bool)
	if data == nil {
		return out
	}
	for _, placed := range data.WorldNPCs {
		out[placed.MemberID] = true
	}
	return out
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

// refuseIfDown refuses a verb whose ACTOR is DOWNED: at zero hit points, out
// of the fight. It answers [ErrDowned], which is where the word is explained.
//
// # Which verbs, and why only those
//
// The two where a downed member could still act: [Manager.Attack] and
// [Manager.Move]. Inside a fight the swing already stops without a gate,
// because the composition splices them out of the turn order and the seam
// above has nothing left to offer them (rpg-toolkit#1077). Free roam has no
// turn order, so the same member can still walk, and can still initiate —
// which is rpg-toolkit#845's shape reproduced on the new stack. The composition
// deliberately did not invent this refusal; it is ruled here, where the sheets
// are, and it is one refusal covering both clocks rather than a rule that only
// works when somebody happens to be in a fight.
//
// # And which are deliberately left open
//
// Not the reads: Where, View, Story, Status and Turn all answer about a downed
// member, because a downed member is still a member (ruled fork (a) on
// rpg-toolkit#959) and a client that could not ask where it fell could not
// render the moment it happened. Not recording an outcome ABOUT a downed member
// either — the killing stroke is itself a beat about somebody who is now down.
// Not a downed TARGET: swinging at one may be narratively silly, and refusing
// it is a different ruling that nobody has made.
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

	return fmt.Errorf("%s %q: %w", role, id, ErrDowned)
}
