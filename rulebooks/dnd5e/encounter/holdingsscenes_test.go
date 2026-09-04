// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// holdingsscenes_test.go is the acceptance table of rpg-project#368 design
// §8, one scene per row this module proves. The fixture and the helpers are
// in holdings_test.go beside it.

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestLootOnTheCaptainRevealsTheDoorToTheLooterAlone is §8 row 2: the
// looter's own DOOR_REVEALED, and the other member's atlas unchanged.
func (s *HoldingsSuite) TestLootOnTheCaptainRevealsTheDoorToTheLooterAlone() {
	enc := s.open(true)
	before := s.atlasBytes(enc, partner)
	s.drop(enc)
	s.walkTo(enc, raider, captainCell)

	_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err)

	s.Run("the looter is told about the door", func() {
		reveals := s.beatsOfKind(enc, raider, "door_revealed")
		s.Require().Len(reveals, 1)
		s.Require().Equal(tombVault, reveals[0]["door"])
	})

	s.Run("the door is on the looter's own map", func() {
		doors, err := enc.DoorsFor(raider)
		s.Require().NoError(err)
		s.Require().True(doorsListed(doors, tombVault))
	})

	s.Run("nobody else hears it and nobody else's map moved", func() {
		s.Require().Empty(s.beatsOfKind(enc, partner, "door_revealed"))
		doors, err := enc.DoorsFor(partner)
		s.Require().NoError(err)
		s.Require().False(doorsListed(doors, tombVault))
		s.Require().Equal(before, s.atlasBytes(enc, partner),
			"the other member's atlas is unchanged until the door is opened in their presence")
	})

	s.Run("the room stays hidden — knowing a door is not seeing behind it", func() {
		atlas, err := enc.AtlasFor(raider)
		s.Require().NoError(err)
		for _, r := range atlas.Regions {
			s.Require().NotEqual("vault", r.ID, "two knowledge moments, deliberately distinct")
		}
	})
}

// TestLootOnABodyWithNothingIsIndistinguishable is §8 row 3 and design P3 —
// the law this whole verb turns on. A body that carried the run's only
// secret and a body that carried nothing produce THE SAME BYTES up to the
// reveal beat, in the story and in every atlas.
func (s *HoldingsSuite) TestLootOnABodyWithNothingIsIndistinguishable() {
	loot := func(knows bool) (string, string, string) {
		enc := s.open(knows)
		s.drop(enc)
		s.walkTo(enc, raider, captainCell)
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().NoError(err)
		return s.storyBytes(enc, partner), s.atlasBytes(enc, partner), s.atlasBytes(enc, raider)
	}

	richStory, richPartnerAtlas, richLooterAtlas := loot(true)
	s.SetupTest()
	poorStory, poorPartnerAtlas, poorLooterAtlas := loot(false)

	s.Run("the bystander's whole story is byte-identical", func() {
		s.Require().Equal(poorStory, richStory,
			"the looted beat names the looter and the body and nothing of what moved")
	})
	s.Run("the bystander's atlas is byte-identical", func() {
		s.Require().Equal(poorPartnerAtlas, richPartnerAtlas)
	})
	s.Run("the LOOTER's atlas differs, and ONLY by the door", func() {
		// The one thing that may differ is what the transfer CAUSED, and
		// slice 1 already decides what that looks like: a found door stops
		// being masked as a wall and appears as a doorway. Asserted as a
		// bound rather than as inequality, because "these differ" would pass
		// for any difference at all — including the room behind the door
		// leaking, which is exactly what must not happen.
		s.Require().NotEqual(poorLooterAtlas, richLooterAtlas)

		enc := s.open(true)
		s.drop(enc)
		s.walkTo(enc, raider, captainCell)
		blind, err := enc.AtlasFor(raider)
		s.Require().NoError(err)
		_, err = enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().NoError(err)
		knowing, err := enc.AtlasFor(raider)
		s.Require().NoError(err)

		s.Require().Empty(blind.Doorways)
		s.Require().Len(knowing.Doorways, 1, "the found door, and nothing else")
		s.Require().Equal(tombVault, knowing.Doorways[0].Door)
		s.Require().Equal(blind.Cells, knowing.Cells, "not one cell of the vault leaked")
		s.Require().Equal(blind.Regions, knowing.Regions, "and not the room behind it")
		s.Require().Equal(blind.Props, knowing.Props)
		s.Require().Len(knowing.Boundaries, len(blind.Boundaries)-1,
			"the wall the door was masquerading as is the one that went")
	})
	s.Run("the looted beat itself is the same beat either way", func() {
		enc := s.open(false)
		s.drop(enc)
		s.walkTo(enc, raider, captainCell)
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().NoError(err)
		looted := s.beatsOfKind(enc, raider, "looted")
		s.Require().Len(looted, 1)
		s.Require().Equal(map[string]any{
			"beat": "looted", "member": string(raider), "target": string(captain),
		}, looted[0], "no field here varies with what the body carried")
	})
}

// TestKillingTheCaptainAndNeverLootingLearnsNothing is §8 row 1 — the
// secrecy scene. Kirk, 2026-09-02: a secrecy check, not a win rule.
func (s *HoldingsSuite) TestKillingTheCaptainAndNeverLootingLearnsNothing() {
	blind := func(knows bool) (string, string) {
		enc := s.open(knows)
		s.drop(enc)
		return s.atlasBytes(enc, raider), s.storyBytes(enc, raider)
	}

	knewAtlas, knewStory := blind(true)
	s.SetupTest()
	neverKnewAtlas, neverKnewStory := blind(false)

	s.Require().Equal(neverKnewAtlas, knewAtlas,
		"the door stays a wall for everyone: the fall transfers nothing (design R2)")
	s.Require().Equal(neverKnewStory, knewStory,
		"and no beat anywhere says the body was carrying anything")

	s.Run("the door is on nobody's map", func() {
		enc := s.open(true)
		s.drop(enc)
		for _, member := range []core.EntityID{raider, partner} {
			doors, err := enc.DoorsFor(member)
			s.Require().NoError(err)
			s.Require().False(doorsListed(doors, tombVault))
		}
	})
}

// TestHoldRemovesThePropForEveryoneAndTheHolderHasIt is §8 row 4.
func (s *HoldingsSuite) TestHoldRemovesThePropForEveryoneAndTheHolderHasIt() {
	enc := s.open(false)
	s.walkTo(enc, raider, chaliceCell)

	_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
	s.Require().NoError(err)

	s.Run("the prop is gone from EVERY member's atlas", func() {
		for _, member := range []core.EntityID{raider, partner, captain} {
			atlas, err := enc.AtlasFor(member)
			s.Require().NoError(err)
			_, present := propInAtlas(atlas, chalice)
			s.Require().False(present, "%s still sees it", member)
		}
		full, err := enc.Atlas()
		s.Require().NoError(err)
		_, present := propInAtlas(full, chalice)
		s.Require().False(present, "and it is gone from the truth-grain atlas too")
	})

	s.Run("the held beat reaches everyone present", func() {
		for _, member := range []core.EntityID{raider, partner} {
			taken := s.beatsOfKind(enc, member, "held")
			s.Require().Len(taken, 1, "%s did not hear it", member)
			s.Require().Equal(map[string]any{
				"beat": "held", "holder": string(raider), "prop": chalice,
			}, taken[0])
		}
	})

	s.Run("the holder carries it out", func() {
		out, err := enc.Exit(&encounter.ExitInput{Member: raider})
		s.Require().NoError(err)
		exited := s.beatsOfKind(enc, partner, "exited")
		s.Require().Len(exited, 1)
		s.Require().Equal([]any{chalice}, exited[0]["holding"])
		s.Require().Nil(out.Closed, "no ending names the chalice, so nothing ended")
	})

	s.Run("the other props are untouched", func() {
		atlas, err := enc.Atlas()
		s.Require().NoError(err)
		_, present := propInAtlas(atlas, pillar)
		s.Require().True(present, "a fold that dropped the wrong prop would show here")
	})
}

// TestExitAtTheBoundExitHoldingTheArtifactEndsTheRun is §8 row 5: it ends
// once, for everyone, naming the carrier — and a member leaving WITHOUT it
// simply departs.
func (s *HoldingsSuite) TestExitAtTheBoundExitHoldingTheArtifactEndsTheRun() {
	enc := s.open(false, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})

	s.Run("the non-carrier leaves first and the run goes on", func() {
		s.walkTo(enc, partner, raiderCell)
		out, err := enc.Exit(&encounter.ExitInput{Member: partner})
		s.Require().NoError(err)
		s.Require().Nil(out.Closed, "one member departed; the others are still in the dungeon")
		exited := s.beatsOfKind(enc, raider, "exited")
		s.Require().Len(exited, 1)
		s.Require().Equal([]any{}, exited[0]["holding"], "they carried nothing out")
		s.Require().Equal(frontGate, exited[0]["exit"], "and they still left through a real exit")
	})

	// The carrier walks into the tomb, takes the heirloom, and comes back.
	s.walkTo(enc, raider, heirloomCell)
	_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: heirloom})
	s.Require().NoError(err)
	s.walkTo(enc, raider, raiderCell)

	out, err := enc.Exit(&encounter.ExitInput{Member: raider})
	s.Require().NoError(err)

	s.Run("the run ends on the scenario's key", func() {
		s.Require().NotNil(out.Closed)
		s.Require().Equal(recovered, out.Closed.Ending)
	})

	s.Run("the departure beat names the exit and what they carried, and comes FIRST", func() {
		beats := s.beats(enc, raider)
		var exitedAt, endedAt = -1, -1
		for i, b := range beats {
			switch b["beat"] {
			case "exited":
				if b["member"] == string(raider) {
					exitedAt = i
					s.Require().Equal(frontGate, b["exit"])
					s.Require().Equal([]any{heirloom}, b["holding"])
				}
			case "ended":
				endedAt = i
				s.Require().Equal(recovered, b["ending"])
			}
		}
		s.Require().Greater(exitedAt, -1)
		s.Require().Greater(endedAt, exitedAt,
			`the record reads "left through the front gate with the heirloom" and THEN "ended"`)
	})

	s.Run("the carrier hears their own run end", func() {
		s.Require().Len(s.beatsOfKind(enc, raider, "ended"), 1,
			"a fresh roster read would have left out the person who just won")
	})

	s.Run("it ends ONCE", func() {
		s.Require().Len(s.beatsOfKind(enc, captain, "ended"), 1)
	})

	s.Run("and nothing was dropped", func() {
		s.Require().Empty(s.beatsOfKind(enc, captain, "dropped"))
	})
}

// TestExitAwayFromTheExitDropsTheHolding is §8 row 6 and design R9: the
// carrier leaves from the vault, the run continues, and the heirloom lies
// where they stood.
func (s *HoldingsSuite) TestExitAwayFromTheExitDropsTheHolding() {
	enc := s.open(false, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})
	s.walkTo(enc, raider, heirloomCell)
	_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: heirloom})
	s.Require().NoError(err)

	out, err := enc.Exit(&encounter.ExitInput{Member: raider})
	s.Require().NoError(err)

	s.Run("the run does not end", func() {
		s.Require().Nil(out.Closed)
	})

	s.Run("the departure names no exit", func() {
		exited := s.beatsOfKind(enc, partner, "exited")
		s.Require().Len(exited, 1)
		s.Require().Equal("", exited[0]["exit"])
	})

	s.Run("a dropped beat reaches everyone present", func() {
		dropped := s.beatsOfKind(enc, partner, "dropped")
		s.Require().Len(dropped, 1)
		s.Require().Equal(heirloom, dropped[0]["prop"])
		s.Require().Equal(string(raider), dropped[0]["member"])
	})

	s.Run("the heirloom is back on the floor, where they stood", func() {
		atlas, err := enc.Atlas()
		s.Require().NoError(err)
		prop, present := propInAtlas(atlas, heirloom)
		s.Require().True(present)
		s.Require().Equal(cellAt(int(heirloomCell.X), int(heirloomCell.Y)), prop.At)
		s.Require().True(prop.Holdable, "and it is the same holdable thing it was")
	})

	s.Run("somebody else can pick it up and finish", func() {
		s.walkTo(enc, partner, heirloomCell)
		_, err := enc.Hold(&encounter.HoldInput{Member: partner, Target: heirloom})
		s.Require().NoError(err)
		s.walkTo(enc, partner, raiderCell)
		out, err := enc.Exit(&encounter.ExitInput{Member: partner})
		s.Require().NoError(err)
		s.Require().NotNil(out.Closed)
		s.Require().Equal(recovered, out.Closed.Ending)
	})
}

// TestLeavingThroughAnUnboundExitDoesNotWalkTheArtifactOut is the reading
// this build takes where the design left an ambiguity, pinned so the choice
// is visible rather than incidental.
//
// The dungeon declares TWO exits and the scenario binds one. A carrier who
// leaves by the other has not won, and R9's stated reason — "otherwise a
// carrier who leaves... takes the only win out of the run with them" — is
// about the hole, not about the cell. So the rule is: a departure that did
// not end the run drops what the member carried, wherever they left from.
func (s *HoldingsSuite) TestLeavingThroughAnUnboundExitDoesNotWalkTheArtifactOut() {
	enc := s.open(false, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})
	s.walkTo(enc, raider, heirloomCell)
	_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: heirloom})
	s.Require().NoError(err)
	s.walkTo(enc, raider, sideDoorCell)

	out, err := enc.Exit(&encounter.ExitInput{Member: raider})
	s.Require().NoError(err)

	s.Require().Nil(out.Closed, "the side door is not the bound exit")

	exited := s.beatsOfKind(enc, partner, "exited")
	s.Require().Len(exited, 1)
	s.Require().Equal(sideDoor, exited[0]["exit"], "they did leave by a real, authored exit")

	dropped := s.beatsOfKind(enc, partner, "dropped")
	s.Require().Len(dropped, 1, "and the artifact stays in the run, at the side door")

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	prop, present := propInAtlas(atlas, heirloom)
	s.Require().True(present)
	s.Require().Equal(cellAt(int(sideDoorCell.X), int(sideDoorCell.Y)), prop.At)
}

// TestEveryBeatNamesItsVerbAsAStatement is §8 row 8 (design §4.1): the beat
// kinds are the words the record says, and no beat here says "interacted".
func (s *HoldingsSuite) TestEveryBeatNamesItsVerbAsAStatement() {
	enc := s.open(true, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})
	s.drop(enc)
	s.walkTo(enc, raider, captainCell)
	_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err)
	s.walkTo(enc, raider, chaliceCell)
	_, err = enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
	s.Require().NoError(err)
	_, err = enc.Exit(&encounter.ExitInput{Member: raider})
	s.Require().NoError(err)

	kinds := map[string]bool{}
	for _, beat := range s.beats(enc, raider) {
		kinds[beat["beat"].(string)] = true
	}
	s.Require().True(kinds["looted"])
	s.Require().True(kinds["held"])
	s.Require().True(kinds["dropped"])
	s.Require().False(kinds["interacted"], "Interact stays the NPC verb; it names no rule half here")
}

// TestHoldingsSurviveASaveAndLoad is the load-act-save law: the journal is
// the one answer to who has what, it persists, and Load replays it rather
// than re-seeding the author's knowledge links — which would hand a looted
// captain back the intel somebody already took.
func (s *HoldingsSuite) TestHoldingsSurviveASaveAndLoad() {
	enc := s.open(true, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})
	s.drop(enc)
	s.walkTo(enc, raider, captainCell)
	_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err)
	s.walkTo(enc, raider, heirloomCell)
	_, err = enc.Hold(&encounter.HoldInput{Member: raider, Target: heirloom})
	s.Require().NoError(err)

	data := enc.ToData()
	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
	})
	s.Require().NoError(err)

	s.Run("the taken prop is still gone", func() {
		atlas, err := reloaded.Atlas()
		s.Require().NoError(err)
		_, present := propInAtlas(atlas, heirloom)
		s.Require().False(present)
	})

	s.Run("the looter still knows the door", func() {
		doors, err := reloaded.DoorsFor(raider)
		s.Require().NoError(err)
		s.Require().True(doorsListed(doors, tombVault))
	})

	s.Run("and the second player to loot the body learns it too", func() {
		// Knowledge copies (holdings.go): the captain still knows the way
		// in, and the next person to search the body finds it exactly as
		// the first did.
		s.walkTo(reloaded, partner, captainCell)
		_, err := reloaded.Loot(&encounter.LootInput{Member: partner, Target: captain})
		s.Require().NoError(err)
		partnerDoors, err := reloaded.DoorsFor(partner)
		s.Require().NoError(err)
		s.Require().True(doorsListed(partnerDoors, tombVault))
	})

	s.Run("the journal is REPLAYED, never re-seeded", func() {
		// A load that re-ran MemberInput.Knows would append the author's
		// links a second time, and a third on the next load, growing the
		// blob without bound. The fact count is what catches that.
		fresh, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data:  data,
			Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: s.witness,
		})
		s.Require().NoError(err)
		s.Require().Equal(len(data.Holdings.Facts), len(fresh.ToData().Holdings.Facts))
	})

	s.Run("the carrier still walks out and wins", func() {
		s.walkTo(reloaded, raider, raiderCell)
		out, err := reloaded.Exit(&encounter.ExitInput{Member: raider})
		s.Require().NoError(err)
		s.Require().NotNil(out.Closed)
		s.Require().Equal(recovered, out.Closed.Ending)
	})
}

// TestAPlainDungeonWritesNoHoldings is the byte-identity claim: an encounter
// where nobody was authored knowing anything and nobody has taken anything
// carries no holdings key at all.
func (s *HoldingsSuite) TestAPlainDungeonWritesNoHoldings() {
	enc := s.open(false)
	s.Require().Nil(enc.ToData().Holdings)

	withIntel := s.open(true)
	s.Require().NotNil(withIntel.ToData().Holdings,
		"a seeded knowledge link is a holding, and it persists")
}

// TestTheLootedIntelKeepsTravelling: a looter's own body can be looted, and
// the intel moves on. One transfer routine, so every caller moves things the
// same way.
func (s *HoldingsSuite) TestTheLootedIntelKeepsTravelling() {
	enc := s.open(true)
	s.drop(enc)
	s.walkTo(enc, raider, captainCell)
	_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err)

	s.standing.down = []encounter.MemberID{captain, raider}
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.walkTo(enc, partner, spatial.Position{X: captainCell.X - 1, Y: captainCell.Y})
	_, err = enc.Loot(&encounter.LootInput{Member: partner, Target: raider, Range: 2})
	s.Require().NoError(err)

	doors, err := enc.DoorsFor(partner)
	s.Require().NoError(err)
	s.Require().True(doorsListed(doors, tombVault))
}

// TestLootTakesThePropOffTheBody is design P5's other half — "a fallen
// holder's body holds it still, and the same loot verb takes it back."
//
// One transfer routine moves EVERY holding, and until this scene existed
// only the intel half of it was covered: a mutation pass that stopped props
// transferring killed no test.
func (s *HoldingsSuite) TestLootTakesThePropOffTheBody() {
	enc := s.open(false, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})
	s.walkTo(enc, raider, heirloomCell)
	_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: heirloom})
	s.Require().NoError(err)

	s.Run("the carrier falls, still holding it", func() {
		s.standing.down = []encounter.MemberID{raider}
		_, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		atlas, err := enc.Atlas()
		s.Require().NoError(err)
		_, present := propInAtlas(atlas, heirloom)
		s.Require().False(present, "a body holds what it was carrying; nothing fell out")
	})

	s.Run("a companion loots the body and carries it out", func() {
		s.walkTo(enc, partner, heirloomCell)
		_, err := enc.Loot(&encounter.LootInput{Member: partner, Target: raider, Range: 2})
		s.Require().NoError(err)

		s.walkTo(enc, partner, raiderCell)
		out, err := enc.Exit(&encounter.ExitInput{Member: partner})
		s.Require().NoError(err)
		s.Require().NotNil(out.Closed, "the heirloom changed hands, so the run can still be won")
		s.Require().Equal(recovered, out.Closed.Ending)
	})
}

// TestLootingTheSameIntelTwiceRevealsOnce: the transfer reuses the reveal
// path search owns, and that path is idempotent — knowledge already held is
// never re-written and never re-beat (conceal.go's own law).
func (s *HoldingsSuite) TestLootingTheSameIntelTwiceRevealsOnce() {
	// TWO monsters, both authored knowing the vault door — which is what it
	// takes to reach the guard at all: looting one body empties it, so a
	// second loot of the SAME body transfers nothing and would say nothing
	// whatever the guard did.
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
		Field: heirloomField(),
		Members: []encounter.MemberInput{
			{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
			{ID: captain, Kind: encounter.KindMonster, Position: captainCell,
				Knows: []encounter.DoorID{tombVault}},
			{ID: sentry, Kind: encounter.KindMonster, Position: sentryCell,
				Knows: []encounter.DoorID{tombVault}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)

	s.standing.down = []encounter.MemberID{captain, sentry}
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.walkTo(enc, raider, captainCell)
	_, err = enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err)
	for _, b := range s.beats(enc, raider) {
		s.T().Logf("BEAT %v", b["beat"])
	}
	s.Require().Len(s.beatsOfKind(enc, raider, "door_revealed"), 1)

	s.Run("the same body a second time has nothing left to give", func() {
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().NoError(err)
		s.Require().Len(s.beatsOfKind(enc, raider, "door_revealed"), 1)
	})

	s.Run("a DIFFERENT body that still carries it reveals nothing new", func() {
		// The guard that matters: knowledge already held is never
		// re-written and never re-beat (conceal.go's own law), reached
		// through a body that genuinely still has the intel to give.
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: sentry, Range: 2})
		s.Require().NoError(err)
		s.Require().Len(s.beatsOfKind(enc, raider, "door_revealed"), 1,
			"a second arrival of knowledge already held is not a second reveal")
		s.Require().Len(s.beatsOfKind(enc, raider, "looted"), 3, "and all three loots happened")
	})
}

// TestLootingIntelForAnOrdinaryDoorRevealsNothing: knowing an unconcealed
// door is inert ([MemberInput.Knows]), and inert means no beat — even in a
// dungeon that DOES carry concealment elsewhere, which is the case the
// world-is-nil short-circuit does not cover.
func (s *HoldingsSuite) TestLootingIntelForAnOrdinaryDoorRevealsNothing() {
	// The fixture, with a SECOND opening in the hall|tomb seam at row 1 and
	// an ordinary door standing in it. Rebuilt rather than appended to: the
	// seam is a whole wall set, and adding a second copy of it would list
	// every crossing twice.
	field := heirloomField()
	field.Walls = append(
		seamWallExcept(3, 8, hallGapRow, 1),
		seamWallExcept(7, 8, vaultSeamRow)...)
	field.Doors = append(append([]encounter.DoorInput(nil), field.Doors...), encounter.DoorInput{
		ID: "hall-tomb-gate", Edges: doorEdgesAcross(3, 1), State: encounter.DoorIsClosed(),
	})

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
		Field: field,
		Members: []encounter.MemberInput{
			{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
			// The captain knows the ORDINARY gate, not the vault door. The
			// field still carries concealment, so the world exists and the
			// only thing standing between this loot and a spurious reveal is
			// the concealed check itself.
			{ID: captain, Kind: encounter.KindMonster, Position: partnerCell,
				Knows: []encounter.DoorID{"hall-tomb-gate"}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)

	s.standing.down = []encounter.MemberID{captain}
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	before, err := enc.AtlasFor(raider)
	s.Require().NoError(err)
	_, err = enc.Loot(&encounter.LootInput{Member: raider, Target: captain, Range: 2})
	s.Require().NoError(err)
	after, err := enc.AtlasFor(raider)
	s.Require().NoError(err)

	s.Require().Empty(s.beatsOfKind(enc, raider, "door_revealed"),
		"there is nothing to reveal about a door anybody can already see")
	s.Require().Equal(before, after)
	s.Require().Len(s.beatsOfKind(enc, raider, "looted"), 1, "and the loot itself happened")
}

// TestLeavingTheBoundExitWithTheWRONGThingDoesNotWin: the ending names an
// item, and carrying a different one is not carrying that one.
func (s *HoldingsSuite) TestLeavingTheBoundExitWithTheWRONGThingDoesNotWin() {
	enc := s.open(false, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})
	s.walkTo(enc, raider, chaliceCell)
	_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
	s.Require().NoError(err)
	s.walkTo(enc, raider, raiderCell)

	out, err := enc.Exit(&encounter.ExitInput{Member: raider})
	s.Require().NoError(err)

	s.Require().Nil(out.Closed, "the chalice is not the artifact")
	exited := s.beatsOfKind(enc, partner, "exited")
	s.Require().Len(exited, 1)
	s.Require().Equal(frontGate, exited[0]["exit"], "they did leave by the bound exit")
	s.Require().Equal([]any{chalice}, exited[0]["holding"])

	dropped := s.beatsOfKind(enc, partner, "dropped")
	s.Require().Len(dropped, 1, "and the run goes on, so what they carried stays in it")
	s.Require().Equal(chalice, dropped[0]["prop"])
}

// TestTheAtlasCarriesTheWaysOut is the projection half of the exits contract
// (rpg-project#368): a way out is structure on the truth grain, so it is in
// every member's atlas, unchanged, whatever they have and have not found.
//
// The client needs both halves of this to draw a dungeon: where the way out
// is, and which things on the floor can be picked up.
func (s *HoldingsSuite) TestTheAtlasCarriesTheWaysOut() {
	enc := s.open(true, recoverEnding(), encounter.EndingInput{
		Key: "withdrawn", Trigger: encounter.TriggerExternal{},
	})

	s.Run("an exit authored is an exit projected, sorted by id", func() {
		atlas, err := enc.Atlas()
		s.Require().NoError(err)
		s.Require().Equal([]encounter.AtlasExit{
			{ID: frontGate, At: cellAt(int(raiderCell.X), int(raiderCell.Y))},
			{ID: sideDoor, At: cellAt(int(sideDoorCell.X), int(sideDoorCell.Y))},
		}, atlas.Exits)
	})

	s.Run("and every member sees the same ones", func() {
		full, err := enc.Atlas()
		s.Require().NoError(err)
		for _, member := range []core.EntityID{raider, partner, captain} {
			mine, err := enc.AtlasFor(member)
			s.Require().NoError(err)
			s.Require().Equal(full.Exits, mine.Exits,
				"%s sees a different way out; an exit is not a secret", member)
		}
	})

	s.Run("a field with no exits projects none", func() {
		plain := encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 6, 6)},
		}
		bare, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			Field:   plain,
			Members: []encounter.MemberInput{{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell}},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)
		atlas, err := bare.Atlas()
		s.Require().NoError(err)
		s.Require().Empty(atlas.Exits, "nothing is defaulted; start is not implicitly a way out")
	})
}

// TestTheAtlasSaysWhatCanBePickedUp is the other half: the holdable flag
// round-trips to every member, so a client offers the Hold action exactly
// where the author allowed it and nowhere else.
func (s *HoldingsSuite) TestTheAtlasSaysWhatCanBePickedUp() {
	enc := s.open(false)

	s.Run("the flag round-trips, and only for what was declared", func() {
		for _, member := range []core.EntityID{raider, partner} {
			atlas, err := enc.AtlasFor(member)
			s.Require().NoError(err)

			heirloomProp, ok := propInAtlas(atlas, heirloom)
			s.Require().True(ok)
			s.Require().True(heirloomProp.Holdable)

			chaliceProp, ok := propInAtlas(atlas, chalice)
			s.Require().True(ok)
			s.Require().True(chaliceProp.Holdable)

			pillarProp, ok := propInAtlas(atlas, pillar)
			s.Require().True(ok)
			s.Require().False(pillarProp.Holdable,
				"a thing nobody declared holdable stays scenery, and the client must not offer it")
		}
	})

	s.Run("the vault's relic is withheld from a member who has not found it", func() {
		atlas, err := enc.AtlasFor(raider)
		s.Require().NoError(err)
		_, ok := propInAtlas(atlas, relic)
		s.Require().False(ok, "a holdable prop inside concealed space is still concealed")
	})
}
