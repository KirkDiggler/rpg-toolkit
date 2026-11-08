// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// Sheets attach once per verb (rpg-project#879 S8, rpg-project#892).
//
// The composition asks who is standing many times inside one verb: after
// every step a patrol takes during a raise, and again in the walker's own
// checks. Each ask used to hand every monster sheet to resolution, which
// decoded and attached all of them on a fresh bus, answered, and tore the bus
// down: on the garrison keep, about a hundred attachments per assessment and
// more than a hundred assessments per Move. A step changes where a monster
// stands, never what its sheet says, so nearly all of that work answered a
// question whose answer had not moved.
//
// # What is kept, and for how long
//
// One monster's participation fact (its life state, read off its own sheet)
// and whether its sheet holds a compulsion, by member, for the verb that
// built the seam. A monster's answer depends on its own record and on nothing
// else in the cast, so an answer kept for one member is the same answer the
// whole-cast attach would give for it.
//
// The memo dies with the seam: [Manager.standingFor] builds a new one for
// every verb (S1, S4), so nothing kept here outlives the verb that kept it. A
// Move that answers before the world runs (rpg-project#879 R2) carries its
// scope, memo included, into its continuation; that is still the same verb,
// under the session guard it never gave back, so no other writer can change a
// sheet in between.
//
// # A sheet written is a sheet re-read
//
// Every monster sheet a verb writes goes through one door,
// [writeScope.replaceMonsterSheet], and that door forgets the member here.
// The next assessment attaches that sheet again and answers from what was
// written: a monster dropped mid-verb is down at the next ask. Nothing else
// forgets: a step is not a write.
//
// # Players are not kept
//
// A player's sheet is the host's record, read per ask through the verb's one
// sheet store ([sheetStore]); a verb-scoped character cache is the store's
// own deferred ruling (R10), not this file's. A party is a handful of sheets;
// the cost this answers is the garrison.
//
// # The bus stays in resolution
//
// This package holds no bus (TestNoBusLivesHere). What is reused is the
// ANSWER resolution gave for a sheet, not the attached sheet itself; the
// attach still happens on resolution's own transient bus, once per member
// per verb, plus once more for each write.
type participationMemo struct {
	kept map[string]monsterParticipation
}

// monsterParticipation is one monster's kept answer.
type monsterParticipation struct {
	fact      resolution.ParticipantParticipation
	compelled bool
}

// participationOf is resolution's participation entry, held in a variable so
// a test can count the sheets the seam hands it
// (CountParticipationAttachesForTest).
var participationOf = resolution.Participation

// newParticipationMemo returns an empty memo for one verb.
func newParticipationMemo() *participationMemo {
	return &participationMemo{kept: make(map[string]monsterParticipation)}
}

// monsters fills facts and compelled for every monster record given,
// answering from the memo where it holds the member and asking resolution,
// once, for the rest. A nil memo keeps nothing and asks for all of them.
//
// An error is not kept: the next ask tries again, exactly as it would have
// without a memo.
func (m *participationMemo) monsters(
	ctx context.Context,
	records []resolution.Participant,
	facts map[string]resolution.ParticipantParticipation,
	compelled map[string]bool,
) error {
	asked := records
	if m != nil {
		asked = make([]resolution.Participant, 0, len(records))
		for _, record := range records {
			id := record.Monster.ID
			if kept, ok := m.kept[id]; ok {
				facts[id] = kept.fact
				compelled[id] = kept.compelled
				continue
			}
			asked = append(asked, record)
		}
	}
	if len(asked) == 0 {
		return nil
	}

	out, err := participationOf(ctx, &resolution.ParticipationInput{Participants: asked})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	held := make(map[string]bool, len(asked))
	for _, record := range asked {
		held[record.Monster.ID] = holdsCompulsion(record.Monster.Conditions)
	}
	for _, member := range out.Members {
		facts[member.Member] = member
		compelled[member.Member] = held[member.Member]
		if m != nil {
			m.kept[member.Member] = monsterParticipation{fact: member, compelled: held[member.Member]}
		}
	}
	return nil
}

// forget drops a member's kept answer, so the next assessment attaches its
// sheet again. Called by the one door every monster sheet write goes through.
func (m *participationMemo) forget(id string) {
	if m == nil {
		return
	}
	delete(m.kept, id)
}
