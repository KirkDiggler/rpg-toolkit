// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestTheRosterSaysWhoIsConcentrating is R11, end to end through the read verb
// the table actually calls.
//
// # Who this row is for
//
// Not the caster: a caster reads its own concentrating condition off its own
// status. Not the creature carrying the spell's effect either: every child
// condition names its source spell and the caster who cast it in its own blob.
// The row answers the question NOBODY ELSE can — whether a member whose sheet
// they do not hold is holding a spell together. Without it, the break beat
// slice three adds arrives about a state the table never saw.
//
// # One bool, and the rest stays on the sheet
//
// The character answers with a view carrying the spell, its name and how many
// effects it owns, and only the bool crosses. Naming the spell on the roster
// would publish the caster's hand to the room, which is a different decision
// than "somebody can see this member is busy".
func TestTheRosterSaysWhoIsConcentrating(t *testing.T) {
	fixture := newRosterFixture(t)
	concentrateOn(t, fixture, "alice", "dnd5e:spells:true-strike", "True Strike")

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.NoError(t, err)

	byID := map[string]session.PublicMember{}
	for _, member := range out.Members {
		byID[member.ID] = member
	}

	require.True(t, byID["alice"].Concentrating,
		"the bard holding True Strike together is concentrating, and the table can see it")
	require.False(t, byID["bob"].Concentrating,
		"a player holding nothing is not, and the flag is per member rather than per roster")
	require.False(t, byID["skel-1"].Concentrating,
		"a monster is false by construction: nothing in this build casts a concentration "+
			"spell from a monster sheet, and the row is filled from a character's own answer")
}

// TestTheRosterFlagFollowsTheSheetRatherThanTheCall pins the direction the
// value travels.
//
// The roster reloads the character on every call, so a concentration that ends
// between two reads must show as ended on the second — the flag is a fact
// about the sheet at read time and not something this seam remembers. The
// mirror of TestRosterReloadsCurrentCharacterIdentityOnEveryCall, over the
// field slice three adds.
func TestTheRosterFlagFollowsTheSheetRatherThanTheCall(t *testing.T) {
	fixture := newRosterFixture(t)
	input := &session.RosterInput{Session: "sess", Player: "player-alice"}

	concentrateOn(t, fixture, "alice", "dnd5e:spells:true-strike", "True Strike")
	first, err := fixture.manager.Roster(context.Background(), input)
	require.NoError(t, err)
	require.True(t, first.Members[0].Concentrating)

	// The spell ends: the condition leaves the sheet, by whatever route ended
	// it. This seam is told nothing and asks again.
	fixture.characters.byID["alice"].Conditions = nil

	second, err := fixture.manager.Roster(context.Background(), input)
	require.NoError(t, err)
	require.False(t, second.Members[0].Concentrating,
		"a concentration that ended is gone from the next read, because the row is the "+
			"sheet's answer rather than this seam's memory")
}

// concentrateOn puts a concentrating condition on a stored character exactly
// as the game server holds one: an opaque blob the sheet loads back.
func concentrateOn(t *testing.T, fixture *rosterFixture, id, spellRef, spellName string) {
	t.Helper()

	holding := conditions.NewConcentratingCondition(id, spellRef, spellName, 10)
	blob, err := holding.ToJSON()
	require.NoError(t, err)

	stored, ok := fixture.characters.byID[id]
	require.True(t, ok, "the fixture holds a character called %q", id)
	stored.Conditions = []json.RawMessage{blob}
}
