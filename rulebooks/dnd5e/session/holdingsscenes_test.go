// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// holdingsscenes_test.go is rpg-toolkit#1496's acceptance table — one scene
// per row the SESSION MODULE proves. The fixture and its helpers are in
// holdings_test.go beside it.

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestLootOnTheCaptainRevealsTheDoorToTheLooterAlone is the acceptance
// table's headline row: the looter's own DOOR_REVEALED, and no other
// member's map or stream moves at all.
//
// The reveal is the beat SEARCH already sends (design P4 — loot is a second
// writer of the fact search writes, never a second mechanism), so what this
// pins at the seam is that a second cause reaches a recipient through the
// same recipient-scoped path, carrying the same payload.
func (s *HoldingsSuite) TestLootOnTheCaptainRevealsTheVaultToTheLooterAlone() {
	ctx := context.Background()
	s.start(true)
	before := s.atlasBytes("bob")

	out, err := s.mgr.Loot(ctx, &session.LootInput{
		Session: "sess", Member: "alice", Target: "captain"})
	s.Require().NoError(err)
	s.Equal([]string{"encounter:sess"}, out.Saved.Written,
		"the transferred fact rides the world, and nothing else is written")

	s.Run("the looter alone is told about the secret", func() {
		s.Equal([]string{encounter.BeatLooted, encounter.BeatConcealmentRevealed, encounter.BeatSighted, encounter.BeatTick},
			s.kinds("alice"), "the verb's own beat first, then what it caused, then the round it spent")
		var body revealedBeat
		s.beatOf("alice", encounter.BeatConcealmentRevealed, &body)
		s.Equal(vaultSecret, body.Concealment, "the record named the vault, so the vault is what arrives")
		s.Require().Len(body.Doors, 1)
		s.Equal("veil", body.Doors[0].Door)
		s.Equal("closed", body.Doors[0].State, "looted is not opened — a found door is still shut")
	})

	s.Run("everyone present hears the LOOTED beat and nothing more", func() {
		s.Equal([]string{encounter.BeatLooted, encounter.BeatTick}, s.kinds("bob"),
			"the loot, and the round it cost the world — but nothing about a secret")
		var looted lootedBeat
		s.beatOf("bob", encounter.BeatLooted, &looted)
		s.Equal(lootedBeat{Member: "alice", Target: "captain"}, looted,
			"looter and body, and nothing of what moved")
	})

	s.Run("the door is on the looter's own list and nobody else's", func() {
		found, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		s.Require().Len(found.Doors, 1)
		s.Equal("veil", found.Doors[0].ID)

		blind, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "bob"})
		s.Require().NoError(err)
		s.Empty(blind.Doors, "for bob the veil is still nowhere")
	})

	s.Run("the other member's whole map is unchanged", func() {
		s.Equal(before, s.atlasBytes("bob"),
			"unchanged until the door is opened in their presence (slice 1, untouched)")
	})

	s.Run("the record gives away the vault, not merely its hinge", func() {
		// R7, at the seam. This sub-test used to read "knowing a door is not
		// seeing the room behind it" and pin 36 cells: the map named a DOOR,
		// and the room behind it was a second secret nobody had found. A
		// record's target is the concealment now, so looting it hands over
		// the floor and the door in one moment — which is what a map of the
		// way in was always worth.
		atlas := s.atlasOf("alice")
		s.Len(atlas.Cells, 72, "both rooms — the looter read the way in")
		s.Require().Len(atlas.Doorways, 1, "with the veil on her map")
	})
}

// TestLootOnABodyWithNothingIsIndistinguishableAtTheSeam is design P3, and
// the law the whole verb turns on: the affordance must not say which body
// carries intel.
//
// The composition pins its own bytes. What is pinned HERE is the two things
// only the seam can break — a BYSTANDER's delivered stream, and their map —
// plus the one place the two runs legitimately differ and why.
func (s *HoldingsSuite) TestLootOnABodyWithNothingIsIndistinguishableAtTheSeam() {
	loot := func(knows bool) (*session.LootOutput, string, string, string, int) {
		s.start(knows)
		out, err := s.mgr.Loot(context.Background(), &session.LootInput{
			Session: "sess", Member: "alice", Target: "captain"})
		s.Require().NoError(err)
		return out, s.atlasBytes("bob"), s.storyBytes("bob"), payloadBytes(s.T(), s.beats("bob")), len(s.beats("alice"))
	}

	rich, richAtlas, richStory, richStream, richLooter := loot(true)
	poor, poorAtlas, poorStory, poorStream, poorLooter := loot(false)

	s.Run("what the bystander was told is byte-identical", func() {
		s.Equal(poorStream, richStream,
			"one LOOTED beat either way, naming the same two members")
	})
	s.Run("the bystander's map is byte-identical", func() {
		s.Equal(poorAtlas, richAtlas)
	})
	s.Run("the bystander's whole story is byte-identical", func() {
		s.Equal(poorStory, richStory,
			"beat for beat: a reveal the bystander could see the shape of would be an oracle")
	})
	s.Run("the response says the same thing about what was written", func() {
		s.Equal(poor.Saved, rich.Saved)
	})
	s.Run("the looter alone is told the reveal and the observation it caused", func() {
		// Only the caller learns the secret and sees the newly exposed door;
		// what the bystander was told above remains byte-identical.
		s.Equal(poorLooter+2, richLooter)
		s.Empty(rich.Delivery.Failed)
		s.Empty(poor.Delivery.Failed)
	})
}

// TestLootIsOfferedOnEveryBodyAndRefusesAnUpright is design P3's other half.
// A member still standing refuses ORDINARILY — the body is visible and being
// on the floor is not a secret — which is the opposite of every prop refusal
// in Hold, and the difference is exactly whether the asker could have learned
// the answer by looking.
func (s *HoldingsSuite) TestLootIsOfferedOnEveryBodyAndRefusesAnUpright() {
	ctx := context.Background()
	s.start(true)

	_, upright := s.mgr.Loot(ctx, &session.LootInput{
		Session: "sess", Member: "alice", Target: "bob"})
	s.Require().ErrorIs(upright, session.ErrNotDown)
	s.Require().NotErrorIs(upright, session.ErrNoMember,
		"a member who is up is a member; the refusal names the real reason")

	_, stranger := s.mgr.Loot(ctx, &session.LootInput{
		Session: "sess", Member: "alice", Target: "nobody"})
	s.Require().ErrorIs(stranger, session.ErrNoMember)

	// And the affordance really is offered on a body carrying nothing: the
	// verb SUCCEEDS on the poor captain, appends the same beat, and tells
	// nobody anything.
	s.start(false)
	out, err := s.mgr.Loot(ctx, &session.LootInput{
		Session: "sess", Member: "alice", Target: "captain"})
	s.Require().NoError(err, "every downed member is lootable")
	s.Equal([]string{encounter.BeatLooted, encounter.BeatTick}, s.kinds("alice"),
		"and an empty body causes nothing — the tick is the round the verb spent, "+
			"which an action costs whether or not it found anything")
	s.Empty(out.Delivery.Failed)
}

// TestHoldRemovesThePropForEveryoneAndTheHolderHasIt is the acceptance
// table's third row.
//
// Physical state folds on the TRUTH GRAIN: an object leaving the floor is not
// a secret, so both members lose it from their own maps and both are told.
func (s *HoldingsSuite) TestHoldRemovesThePropForEveryoneAndTheHolderHasIt() {
	ctx := context.Background()
	s.start(true)
	s.Require().Contains(s.propsOf("bob"), heirloomID, "it is on the floor to begin with")

	_, err := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "alice", Target: heirloomID, Range: 2})
	s.Require().NoError(err)

	s.Run("it is gone from every member's own map", func() {
		s.NotContains(s.propsOf("alice"), heirloomID)
		s.NotContains(s.propsOf("bob"), heirloomID)
		s.Contains(s.propsOf("bob"), chaliceID,
			"and the prop nobody touched is still there")
	})

	s.Run("everyone present is told, with the same payload", func() {
		for _, who := range []string{"alice", "bob"} {
			s.Equal([]string{encounter.BeatHeld, encounter.BeatSighted}, s.kinds(who))
			var held heldBeat
			s.beatOf(who, encounter.BeatHeld, &held)
			s.Equal(heldBeat{Holder: "alice", Prop: heirloomID}, held)
		}
	})

	s.Run("the holding rides the record into the next verb", func() {
		// The proof that it was SAVED rather than merely returned: a
		// different call, loading the world back from the repository, acts on
		// the holding this one wrote.
		out, err := s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		s.Require().NotNil(out.Closed, "alice stands on the bound exit holding it")
	})
}

// TestExitAtTheBoundExitEndsTheRunNamingTheCarrier is the acceptance table's
// fourth row, and it is one scene because the two halves are one claim: a
// departure WITHOUT the artifact is today's Exit, and a departure WITH it at
// the bound exit is the run ending.
func (s *HoldingsSuite) TestExitAtTheBoundExitEndsTheRunNamingTheCarrier() {
	ctx := context.Background()

	s.Run("the carrier leaves through the bound exit and the run ends for everyone", func() {
		s.start(true)
		_, err := s.mgr.Hold(ctx, &session.HoldInput{
			Session: "sess", Member: "alice", Target: heirloomID, Range: 2})
		s.Require().NoError(err)
		s.mark()

		out, err := s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		s.Require().NotNil(out.Closed, "the scenario's ending fired")
		s.Equal(recovered, out.Closed.Ending)

		var exited exitedBeat
		s.beatOf("alice", encounter.BeatExited, &exited)
		s.Equal("alice", exited.Member)
		s.Equal([]string{heirloomID}, exited.Holding, "the record names what she carried out")
		s.Equal(frontGate, exited.Exit, "and the way she left by")

		s.Equal(recovered, s.endingTold("alice"),
			"the DEPARTING CARRIER is told about the run she just won — she has already "+
				"left the roster by the time the ending fires, and a fresh roster read "+
				"would have left her out of the beat announcing it")
		s.Equal(recovered, s.endingTold("bob"),
			"and so is the member still standing in the tomb")

		s.Equal([]string{encounter.BeatExited, encounter.BeatEnded}, s.kinds("alice"),
			"the departure is narrated BEFORE the ending: 'left through the front gate "+
				"with the heirloom', and then 'ended'")

		status, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
		s.Require().NoError(err)
		s.False(status.Open)
		s.Require().NotNil(status.Outcome)
		s.Equal(recovered, status.Outcome.Ending)
	})

	s.Run("the other member leaves first and the run goes on without them", func() {
		s.start(true)
		_, err := s.mgr.Hold(ctx, &session.HoldInput{
			Session: "sess", Member: "alice", Target: heirloomID, Range: 2})
		s.Require().NoError(err)
		s.mark()

		out, err := s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "bob"})
		s.Require().NoError(err)
		s.Require().Nil(out.Closed, "one member departing is not the run ending")

		var body exitedBeat
		s.beatOf("alice", encounter.BeatExited, &body)
		s.Equal("bob", body.Member)
		s.Empty(body.Holding, "bob carried nothing, and empty is the truth rather than unknown")
		s.Equal(sideDoor, body.Exit, "he left through an authored exit NOTHING binds")

		status, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
		s.Require().NoError(err)
		s.True(status.Open)

		s.mark()
		ended, err := s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		s.Require().NotNil(ended.Closed)
		s.Equal(recovered, ended.Closed.Ending,
			"the SCENARIO's ending, not the abandoned auto-close the last departure "+
				"would otherwise have produced")

		// A MEMBER WHO WITHDREW EARLIER IS NOT TOLD, and that is the
		// composition's audience rather than a delivery choice here: the
		// ending is announced to the roster as it stood when the verb began,
		// plus the carrier that same verb removed. Bob was already out of
		// the run. Pinned rather than argued — see the PR's findings; a
		// client that wants to tell a withdrawn player how the run finished
		// has to ask, and nothing here answers it unasked.
		s.Empty(s.kinds("bob"),
			"he had left the run before it ended, so the ending is not addressed to him")
	})
}

// TestExitAwayFromTheBoundExitDropsTheHolding is the acceptance table's fifth
// row (R9): they drop it.
//
// The carrier here walks out of an exit the dungeon DOES declare and no
// scenario binds, which is the sharper version of "anywhere but the exit" —
// R9's reason is the hole (a carrier walking off with the only win in a run
// that is still going), not the cell.
func (s *HoldingsSuite) TestExitAwayFromTheBoundExitDropsTheHolding() {
	ctx := context.Background()
	s.start(true)

	_, err := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "bob", Target: chaliceID, Range: 1})
	s.Require().NoError(err)
	s.Require().NotContains(s.propsOf("alice"), chaliceID, "off the floor while carried")
	s.mark()

	out, err := s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Require().Nil(out.Closed, "nothing binds the side door, so the run goes on")

	s.Run("everyone present is told where it landed", func() {
		var dropped droppedBeat
		s.beatOf("alice", encounter.BeatDropped, &dropped)
		s.Equal("bob", dropped.Member)
		s.Equal(chaliceID, dropped.Prop)
		s.Equal(bobCellAbsolute(), dropped.Position, "on the cell he stood on, on the way out")
	})

	s.Run("the departure is narrated before the drop", func() {
		s.Equal([]string{
			encounter.BeatExited, encounter.BeatDropped, encounter.BeatSighted,
		}, s.kinds("alice"), "and losing sight of the leaver is its own beat")
	})

	s.Run("and it is back on the map, at the drop cell", func() {
		found, present := s.propsOf("alice")[chaliceID]
		s.Require().True(present, "the prop reappears — a drop is a new fact, not an erasure")
		s.Equal(bobCellAbsolute(), found.At, "at the drop cell, not the authored one")
		s.True(found.Holdable, "and still holdable, so somebody else can finish the run")
	})
}

// TestTheProbeLawIsIndistinguishableAtTheSeam is the secrecy claim this
// module is responsible for: a guessed id must not map a room nobody has
// found, and translate must not undo that by carrying different text.
//
// FOUR CASES, ONE ANSWER. An id that names nothing at all; a holdable prop
// inside the concealed vault; a prop in there that is not holdable; and — in
// the same breath, since the vault is far away too — one out of range. The
// composition hoists the visibility gate above its whole validation order for
// this reason; the seam's contribution is that a translated error carries our
// sentinel ALONE, so no inner text survives to tell them apart.
func (s *HoldingsSuite) TestTheProbeLawIsIndistinguishableAtTheSeam() {
	ctx := context.Background()
	s.start(true)

	answers := map[string]string{}
	for _, target := range []string{"nothing-by-that-name", relicID, urnID} {
		_, err := s.mgr.Hold(ctx, &session.HoldInput{
			Session: "sess", Member: "alice", Target: target, Range: 99})
		s.Require().ErrorIs(err, session.ErrNoProp)
		answers[target] = err.Error()
	}

	s.Equal(answers["nothing-by-that-name"], answers[relicID],
		"a holdable prop in a room nobody has found answers exactly as a phantom id does")
	s.Equal(answers["nothing-by-that-name"], answers[urnID],
		"and so does an unholdable one — 'not holdable' about an unseen id would answer "+
			"'yes, there is something by that name in a room you have not found'")

	s.Run("nothing was written and nobody was told", func() {
		s.Empty(s.stream.published)
	})
}

// TestAVisiblePropRefusesByName is the probe law's boundary: there is no
// secret in a pillar. Every refusal about a prop the member CAN see says what
// it means, and each is its own sentinel a host can act on.
func (s *HoldingsSuite) TestAVisiblePropRefusesByName() {
	ctx := context.Background()
	s.start(true)

	_, scenery := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "alice", Target: pillarID, Range: 99})
	s.Require().ErrorIs(scenery, session.ErrNotHoldable,
		"a thing nobody declared holdable stays scenery, and says so")

	_, far := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "alice", Target: heirloomID, Range: 1})
	s.Require().ErrorIs(far, session.ErrOutOfRange,
		"reach is the honest refusal for something in plain sight")

	_, err := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "alice", Target: heirloomID, Range: 2})
	s.Require().NoError(err)

	_, taken := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "bob", Target: heirloomID, Range: 99})
	s.Require().ErrorIs(taken, session.ErrAlreadyHeld,
		"somebody is carrying it, and that is not a secret either")
}

// TestTheTurnClockGatesBothVerbsAtTheSeam carries design §4.4 into this
// package's error map: out of combat both verbs are free; in a fight a member
// acts on their own turn and waits otherwise.
//
// TWO REAL PLAYERS is what makes "not bob's turn" an observable state — a
// lone player fighting a Pass-driven monster cycles straight back to their own
// turn, so there would be no window to ask in (move_turnclock_test.go's own
// lesson).
func (s *HoldingsSuite) TestTheTurnClockGatesBothVerbsAtTheSeam() {
	ctx := context.Background()
	s.start(true, armedFighter("alice"), armedFighter("bob"))

	s.Run("out of combat both verbs are free", func() {
		turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "bob"})
		s.Require().NoError(err)
		s.Require().Equal(session.ClockWorld, turn.Clock)

		_, err = s.mgr.Loot(ctx, &session.LootInput{
			Session: "sess", Member: "bob", Target: "captain", Range: 3})
		s.Require().NoError(err)
	})

	launched := s.launchHeirloom(true,
		[]dungeonspec.MonsterPlacement{monsterAt("skel-1", refs.Monsters.Skeleton().String(), 3, 2)},
		armedFighter("alice"), armedFighter("bob"))
	s.Require().NotEmpty(launched.Formed, "standing in plain sight starts a fight")

	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock)
	s.Require().Equal("alice", turn.Active, "alice is first registered — first in initiative")

	s.Run("the member the clock is not waiting on is refused, by name", func() {
		_, loot := s.mgr.Loot(ctx, &session.LootInput{
			Session: "sess", Member: "bob", Target: "captain", Range: 9})
		s.Require().ErrorIs(loot, session.ErrNotYourTurn)

		_, hold := s.mgr.Hold(ctx, &session.HoldInput{
			Session: "sess", Member: "bob", Target: chaliceID, Range: 9})
		s.Require().ErrorIs(hold, session.ErrNotYourTurn)
	})

	s.Run("the active member acts freely — no slot, no cost, this slice", func() {
		_, loot := s.mgr.Loot(ctx, &session.LootInput{
			Session: "sess", Member: "alice", Target: "captain", Range: 9})
		s.Require().NoError(loot)

		_, hold := s.mgr.Hold(ctx, &session.HoldInput{
			Session: "sess", Member: "alice", Target: chaliceID, Range: 9})
		s.Require().NoError(hold)
	})
}

// TestTheAtlasCarriesTheWaysOutAndWhatCanBePickedUp pins the two facts the
// wire needs and the ONE this seam must not re-derive.
//
// The held filter is inherited BY CONSTRUCTION — the composition drops a held
// prop from the atlas itself, and both reads here call through to it — but
// projectAtlas is a field-for-field copy, so the two facts added BESIDE that
// filter reach a client only by being carried. That is the difference this
// scene states.
func (s *HoldingsSuite) TestTheAtlasCarriesTheWaysOutAndWhatCanBePickedUp() {
	ctx := context.Background()
	s.start(true)

	s.Run("an exit authored is an exit projected, identically for every member", func() {
		want := []session.AtlasExit{
			{ID: frontGate, At: aliceCellAbsolute()},
			{ID: sideDoor, At: bobCellAbsolute()},
		}
		s.Equal(want, s.atlasOf("alice").Exits, "sorted by id, in dungeon-absolute cells")
		s.Equal(want, s.atlasOf("bob").Exits,
			"a way out is structure, not knowledge: the party that has not found the "+
				"vault still walked in through the front gate")
	})

	s.Run("a prop says whether it can be picked up, and carries its author's name", func() {
		byID := map[string]session.AtlasProp{}
		for _, p := range s.propsOf("alice") {
			byID[p.ID] = p
		}
		s.Require().Contains(byID, heirloomID)
		s.True(byID[heirloomID].Holdable)
		s.Require().Contains(byID, pillarID)
		s.False(byID[pillarID].Holdable,
			"and a client offers Hold only where this is true, never guessing from an id")
	})

	s.Run("the unscoped read carries them too", func() {
		// AtlasOf answers the host's own whole truth about authored content —
		// no member, no concealment — and it is the read the builder uses.
		authored, err := s.mgr.AtlasOf(ctx, &session.AtlasOfInput{Dungeon: sceneInput(heirloomWorld(true)).Dungeon})
		s.Require().NoError(err)
		s.Len(authored.Exits, 2)
		s.Contains(propIDs(authored), relicID, "including what is concealed from every member")
	})

	s.Run("a held prop is gone from the unscoped read as well", func() {
		_, err := s.mgr.Hold(ctx, &session.HoldInput{
			Session: "sess", Member: "alice", Target: heirloomID, Range: 2})
		s.Require().NoError(err)
		s.NotContains(s.propsOf("alice"), heirloomID)
		s.NotContains(s.propsOf("bob"), heirloomID,
			"the filter is the composition's, inherited by both reads rather than "+
				"re-derived in this package")
	})
}

// TestEveryBeatNamesItsVerbAsAStatement is the naming rule (design §4.1) as
// something a test can fail: a verb is named by what the record will say.
//
// NO BEAT SAYS "took" — Take is reserved for the act that lands a thing in a
// character's inventory (R10) — and none says "interacted", which is a fine
// button and a poor fact.
func (s *HoldingsSuite) TestEveryBeatNamesItsVerbAsAStatement() {
	ctx := context.Background()
	s.start(true)

	_, err := s.mgr.Loot(ctx, &session.LootInput{
		Session: "sess", Member: "bob", Target: "captain", Range: 3})
	s.Require().NoError(err)
	_, err = s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "bob", Target: chaliceID, Range: 1})
	s.Require().NoError(err)
	_, err = s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)

	s.Equal([]string{
		encounter.BeatLooted,
		// THE WORLD CLOCK MOVED, because looting is an action and an action
		// costs the world a round (rpg-project#465 §5). It is listed rather
		// than filtered out: time passing is part of the fiction now, it goes
		// to everybody, and a test about the order beats arrive in is exactly
		// where that belongs on the record. Hold and Exit raise nothing
		// further — the clock accrues by DRIVER as a high-water mark, not as a
		// sum, so bob's second and third acts are still inside his first round.
		encounter.BeatTick,
		encounter.BeatHeld, encounter.BeatSighted, encounter.BeatExited, encounter.BeatDropped,
		// And the departure changed what alice can see, which is its own
		// statement on her own stream — bob left the map, so she holds him
		// as a ghost now.
		encounter.BeatSighted,
	}, s.kinds("alice"), "every beat arrived named, and in the order the fiction happened")
	s.Equal([]string{
		encounter.BeatLooted, encounter.BeatConcealmentRevealed, encounter.BeatSighted, encounter.BeatTick,
		encounter.BeatHeld, encounter.BeatSighted, encounter.BeatExited, encounter.BeatDropped,
	}, s.kinds("bob"), "and the ACTOR's stream is the same list plus what his loot caused, "+
		"which is the one beat nobody else may see")

	s.Run("no act reached a client undecoded", func() {
		s.Require().NotEmpty(s.stream.published)
		for _, act := range s.stream.published {
			s.Empty(act.Undecoded,
				"an unread entry would still be delivered, and would narrate nothing")
			s.NotNil(act.Body, "and every entry decodes its typed body")
		}
	})

	s.Run("the words themselves", func() {
		s.Equal("looted", encounter.BeatLooted)
		s.Equal("held", encounter.BeatHeld)
		s.Equal("dropped", encounter.BeatDropped)
	})
}

// TestAnOrdinaryDepartureCarriesNothingAndNamesNoExit is the negative that
// makes the exited beat's `holding` and `exit` honest.
//
// Empty is not "unknown" on either of them: it is the truth that this
// departure carried nothing and used no authored way out, and it is what a
// client reads to know a departure was not a withdrawal through the front
// gate. Every departure before this slice looked exactly like this one.
func (s *HoldingsSuite) TestAnOrdinaryDepartureCarriesNothingAndNamesNoExit() {
	ctx := context.Background()
	s.start(true)

	// Off both authored exit cells: one step away from the side door.
	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{hexCell(3, 1)}})
	s.Require().NoError(err)
	s.mark()

	out, err := s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Require().Nil(out.Closed)

	var body exitedBeat
	s.beatOf("alice", encounter.BeatExited, &body)
	s.Equal("bob", body.Member)
	s.Empty(body.Holding)
	s.Empty(body.Exit, "no authored exit was used, and that is a fact rather than a gap")

	s.Equal([]string{encounter.BeatExited, encounter.BeatSighted}, s.kinds("alice"),
		"nothing was carried, so nothing was dropped")
}

// TestTheBodyIsDownBecauseItsSheetSaysSo pins the one thing this seam
// contributes to Loot that the composition cannot: WHO IS DOWN.
//
// The composition holds no hit points and can only be told. The captain is a
// body because the session's standing seam read its stored sheet and the
// sheet said zero — so a run where that sheet is missing refuses the verb
// rather than guessing, and the refusal is the ordinary one.
func (s *HoldingsSuite) TestTheBodyIsDownBecauseItsSheetSaysSo() {
	ctx := context.Background()
	s.start(true)

	// Same world, same captain, the sheet taken away: a sheetless monster is
	// not a broken world, it is a monster nobody can say is down.
	stored := s.sessions.byID["sess"]
	stored.NPCs = nil

	_, err := s.mgr.Loot(ctx, &session.LootInput{
		Session: "sess", Member: "alice", Target: "captain"})
	s.Require().ErrorIs(err, session.ErrNotDown,
		"the answer comes from the sheet, not from how the fixture was authored")
}

// TestBothVerbsRefuseTheirOwnArgumentsFirst pins the cheap refusals every
// verb at this seam owes a caller, before a session is opened or anything is
// loaded.
func (s *HoldingsSuite) TestBothVerbsRefuseTheirOwnArgumentsFirst() {
	ctx := context.Background()
	s.start(true)

	_, err := s.mgr.Loot(ctx, nil)
	s.Require().ErrorIs(err, session.ErrNilInput)
	_, err = s.mgr.Hold(ctx, nil)
	s.Require().ErrorIs(err, session.ErrNilInput)

	_, err = s.mgr.Loot(ctx, &session.LootInput{Session: "sess", Target: "captain"})
	s.Require().ErrorIs(err, session.ErrNoMemberID)
	_, err = s.mgr.Loot(ctx, &session.LootInput{Session: "sess", Member: "alice"})
	s.Require().ErrorIs(err, session.ErrNoMemberID, "a body is a member id too")

	_, err = s.mgr.Hold(ctx, &session.HoldInput{Session: "sess", Target: heirloomID})
	s.Require().ErrorIs(err, session.ErrNoMemberID)
	_, err = s.mgr.Hold(ctx, &session.HoldInput{Session: "sess", Member: "alice"})
	s.Require().ErrorIs(err, session.ErrNoProp,
		"an empty prop id is a prop refusal, not a member one — the two arguments name "+
			"different kinds of thing")

	_, err = s.mgr.Loot(ctx, &session.LootInput{Member: "alice", Target: "captain"})
	s.Require().ErrorIs(err, session.ErrNoSessionID)
	_, err = s.mgr.Hold(ctx, &session.HoldInput{Member: "alice", Target: heirloomID})
	s.Require().ErrorIs(err, session.ErrNoSessionID)

	s.Empty(s.stream.published, "and none of that told anybody anything")
}

// revealedBeat is a concealment_revealed beat's payload, as far as these
// scenes read it.
type revealedBeat struct {
	Concealment string `json:"concealment"`
	Doors       []struct {
		Door  string `json:"door"`
		State string `json:"state"`
	} `json:"doors"`
}

// lootedBeat is a looted beat's payload: the looter and the body.
type lootedBeat struct {
	Member string `json:"member"`
	Target string `json:"target"`
}

// heldBeat is a held beat's payload.
type heldBeat struct {
	Holder string `json:"holder"`
	Prop   string `json:"prop"`
}

// exitedBeat is an exited beat's payload.
type exitedBeat struct {
	Member  string   `json:"member"`
	Holding []string `json:"holding"`
	Exit    string   `json:"exit"`
}

// droppedBeat is a dropped beat's payload.
type droppedBeat struct {
	Member   string           `json:"member"`
	Prop     string           `json:"prop"`
	Position spatial.Position `json:"position"`
}

// endingTold is the ending the one ended beat a recipient witnessed since
// the last mark names.
func (s *HoldingsSuite) endingTold(recipient string) string {
	s.T().Helper()
	var ended struct {
		Ending string `json:"ending"`
	}
	s.beatOf(recipient, encounter.BeatEnded, &ended)
	return ended.Ending
}

// TestHoldingAScrollTeachesTheHolderAtTheSeam is design R6 across the seam:
// the walk's easy path, end to end through the manager and the story.
//
// Kirk's reason for the ruling — "if we want to test the intel we need to be
// able to place it on a few things — not the hardest monster to kill in the
// game" — is only true if the reveal actually reaches a client. The
// composition applies the record; this asserts the seam publishes it, to the
// holder alone, with the payload a client reads.
func (s *HoldingsSuite) TestHoldingAScrollTeachesTheHolderAtTheSeam() {
	ctx := context.Background()
	s.start(false) // NOBODY was authored holding anything

	_, err := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "alice", Target: scrollID, Range: 2})
	s.Require().NoError(err)

	s.Run("the holder is told what it says, after being told they hold it", func() {
		s.Equal([]string{encounter.BeatHeld, encounter.BeatConcealmentRevealed, encounter.BeatSighted}, s.kinds("alice"),
			"picking it up is the cause; what it teaches is the consequence")

		var body revealedBeat
		s.beatOf("alice", encounter.BeatConcealmentRevealed, &body)
		s.Equal(vaultSecret, body.Concealment)
	})

	s.Run("the bystander sees a thing picked up and is taught nothing", func() {
		s.Equal([]string{encounter.BeatHeld, encounter.BeatSighted}, s.kinds("bob"),
			"holding is public; what the scroll says is not")
	})

	s.Run("and no beat anywhere says which prop carried intel", func() {
		// Design P3 at the seam: the held beat names holder and prop, and
		// nothing about records. A client offering Hold must not be able to
		// tell the scroll from the chalice.
		var held heldBeat
		s.beatOf("bob", encounter.BeatHeld, &held)
		s.Equal(heldBeat{Holder: "alice", Prop: scrollID}, held)
	})
}
