// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// holdingsrefusals_test.go is what Loot and Hold REFUSE, and the two
// construction refusals the new authored facts bring with them
// (rpg-project#368, design §4.2, §4.3, §4.4 and §6).
//
// The refusals are half the design: the probe law decides which of them may
// say what they mean, and the turn clock decides when either verb runs at
// all.

import (
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestLootRefusesInOrder walks design §4.2's validation order, each refusal
// asserted from a state where every EARLIER check would have passed — so a
// reordering shows up here rather than in production.
func (s *HoldingsSuite) TestLootRefusesInOrder() {
	enc := s.open(true)

	s.Run("nil input", func() {
		_, err := enc.Loot(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})
	s.Run("empty member", func() {
		_, err := enc.Loot(&encounter.LootInput{Target: captain})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
	s.Run("empty target", func() {
		_, err := enc.Loot(&encounter.LootInput{Member: raider})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
	s.Run("a negative range is a caller defect, not a smaller reach", func() {
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain, Range: -1})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
	s.Run("not a member", func() {
		_, err := enc.Loot(&encounter.LootInput{Member: "ghost", Target: captain})
		s.Require().ErrorIs(err, encounter.ErrNotMember)
	})
	s.Run("target not a member", func() {
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: "ghost"})
		s.Require().ErrorIs(err, encounter.ErrNotMember)
	})
	s.Run("target is still standing — an ORDINARY refusal", func() {
		// The body is visible and whether it is down is not a secret, so
		// this says what it means (design §4.2).
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().ErrorIs(err, encounter.ErrNotDown)
	})
	s.Run("out of range, once they are down", func() {
		s.drop(enc)
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	})
	s.Run("a closed encounter refuses before any of it", func() {
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

// TestLootIsOfferedOnEveryBody is design P3 from the refusal side: no body is
// specially lootable, and the verb never refuses because a body is empty.
func (s *HoldingsSuite) TestLootIsOfferedOnEveryBody() {
	// The captain knows the vault door. The partner knows nothing. Both are
	// down, and BOTH are lootable — that is the affordance half of P3.
	enc := s.open(true)
	s.standing.down = []encounter.MemberID{captain, partner}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Run("a fellow player's empty body is a body, and gives nothing", func() {
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: partner})
		s.Require().NoError(err)
		s.Require().Len(s.beatsOfKind(enc, raider, "looted"), 1)
		// THE SECRECY HALF. A body with nothing must transfer nothing —
		// which is only true if the transfer reads the holdings of THIS
		// body rather than of whoever happens to hold something.
		s.Require().Empty(s.beatsOfKind(enc, raider, "door_revealed"),
			"looting an empty body must not hand over somebody else's secret")
		doors, err := enc.DoorsFor(raider)
		s.Require().NoError(err)
		s.Require().False(doorsListed(doors, tombVault))
	})

	s.Run("and the body that DOES carry it gives it", func() {
		s.walkTo(enc, raider, captainCell)
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().NoError(err)
		s.Require().Len(s.beatsOfKind(enc, raider, "looted"), 2)
		s.Require().Len(s.beatsOfKind(enc, raider, "door_revealed"), 1)
	})
}

// TestHoldRefusesInOrder walks design §4.3's validation order.
func (s *HoldingsSuite) TestHoldRefusesInOrder() {
	enc := s.open(false)

	s.Run("nil input", func() {
		_, err := enc.Hold(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})
	s.Run("empty member", func() {
		_, err := enc.Hold(&encounter.HoldInput{Target: chalice})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
	s.Run("empty target", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: raider})
		s.Require().ErrorIs(err, encounter.ErrNoProp)
	})
	s.Run("a negative range", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice, Range: -1})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
	s.Run("not a member", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: "ghost", Target: chalice})
		s.Require().ErrorIs(err, encounter.ErrNotMember)
	})
	s.Run("no such prop", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: "nothing-like-this"})
		s.Require().ErrorIs(err, encounter.ErrNoProp)
	})
	s.Run("out of range", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	})
	s.Run("already taken", func() {
		s.walkTo(enc, raider, chaliceCell)
		_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
		s.Require().NoError(err)
		_, err = enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
		s.Require().ErrorIs(err, encounter.ErrAlreadyHeld)
		_, err = enc.Hold(&encounter.HoldInput{Member: partner, Target: chalice})
		s.Require().ErrorIs(err, encounter.ErrAlreadyHeld, "and it is gone for everybody, not just the holder")
	})
	s.Run("a closed encounter refuses before any of it", func() {
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

// TestTheProbeLawAppliesToProps is design §4.3's one subtle refusal, and the
// reason this verb has two answers for one situation.
//
// A guessed id must not be able to map a room nobody has found. So a prop
// inside concealed space refuses IDENTICALLY to a prop that does not exist,
// while a prop the member can see refuses by name — there is no secret in a
// pillar.
func (s *HoldingsSuite) TestTheProbeLawAppliesToProps() {
	enc := s.open(false)
	s.walkTo(enc, raider, chaliceCell)

	s.Run("a visible prop that is not holdable refuses BY NAME", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: pillar, Range: 9})
		s.Require().ErrorIs(err, encounter.ErrNotHoldable)
	})

	s.Run("a prop inside space they cannot see refuses as NO SUCH PROP", func() {
		// The relic is holdable, and it is in the concealed vault. It is
		// out of range too — but the probe answer is decided before range
		// is ever measured, so a guesser cannot walk the map by comparing
		// which refusal they got.
		_, err := enc.Hold(&encounter.HoldInput{Member: raider, Target: relic, Range: 99})
		s.Require().ErrorIs(err, encounter.ErrNoProp)
	})

	s.Run("and an id that names nothing refuses the same way", func() {
		_, invented := enc.Hold(&encounter.HoldInput{Member: raider, Target: "no-such-thing", Range: 99})
		_, hidden := enc.Hold(&encounter.HoldInput{Member: raider, Target: relic, Range: 99})
		// The only thing either message carries beyond the sentinel is the
		// id the CALLER supplied, which the caller already knows. Blank
		// that out and the two must be the same sentence — a guesser
		// comparing them learns nothing.
		s.Require().Equal(
			strings.Replace(invented.Error(), "no-such-thing", "«id»", 1),
			strings.Replace(hidden.Error(), relic, "«id»", 1),
			"the two must be indistinguishable, message and all")
	})
}

// TestTheTurnClockGatesBothVerbs is design §4.4: out of combat both verbs are
// free; in a fight a member acts on their own turn and not otherwise.
//
// This is the one scene that installs a capability reporting CONTACT, so a
// bubble actually forms. Everything else in this suite runs out of combat on
// purpose — see nobodyIsInContact.
func (s *HoldingsSuite) TestTheTurnClockGatesBothVerbs() {
	fight := &downList{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: fight, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
		Field: heirloomField(),
		Members: []encounter.MemberInput{
			{ID: raider, Kind: encounter.KindPlayer, Position: heirloomCell},
			{ID: partner, Kind: encounter.KindPlayer, Position: chaliceCell},
			{ID: captain, Kind: encounter.KindMonster, Position: captainCell},
			// A SECOND monster, so the fight outlives the captain's fall.
			// Without it, dropping the only monster decides the fight, the
			// bubble dissolves, and the loot below would be free — which is
			// correct behaviour and the wrong thing for this scene to
			// measure.
			{ID: sentry, Kind: encounter.KindMonster, Position: sentryCell},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)

	// The contact above formed a bubble at first light. WHO is active is
	// trigger detection's deterministic order, not this scene's to assume —
	// so the scene asks, and then says which of the two players it is
	// talking about.
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: raider})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, clock.Kind, "a player and a monster sharing a room is a fight")

	active, waiting := raider, partner
	target, otherTarget := heirloom, chalice
	if clock.Active != raider {
		active, waiting = partner, raider
		target, otherTarget = chalice, heirloom
	}
	s.Require().Equal(active, clock.Active)

	s.Run("the member whose turn it is may take", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: active, Target: target})
		s.Require().NoError(err, "free on their own turn")
	})

	s.Run("a member whose turn it is not may not", func() {
		_, err := enc.Hold(&encounter.HoldInput{Member: waiting, Target: otherTarget})
		s.Require().ErrorIs(err, encounter.ErrNotActive,
			"this mirrors Step's turn gate exactly (ADR-0044)")
	})

	s.Run("and may not loot either", func() {
		fight.down = []encounter.MemberID{captain}
		_, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		// Removing the captain from the order may have moved the turn, and
		// the sentry keeps the fight alive either way — so the scene asks
		// again who is waiting rather than assuming.
		now, err := enc.ClockOf(&encounter.ClockOfInput{Member: raider})
		s.Require().NoError(err)
		s.Require().Equal(encounter.ClockTurn, now.Kind, "the sentry keeps the fight running")
		stillWaiting := raider
		if now.Active == raider {
			stillWaiting = partner
		}

		_, err = enc.Loot(&encounter.LootInput{Member: stillWaiting, Target: captain, Range: 9})
		s.Require().ErrorIs(err, encounter.ErrNotActive)
	})
}

// TestConstructionRefusesAnUnauthoredKnowledgeLink: a link to a door that
// does not exist is a secret the author thinks they placed and did not
// (design P1). Whether the door is CONCEALED is deliberately not asked.
func (s *HoldingsSuite) TestConstructionRefusesAnUnauthoredKnowledgeLink() {
	setup := func(knows []encounter.DoorID) error {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: s.witness,
			Field: heirloomField(),
			Members: []encounter.MemberInput{
				{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
				{ID: captain, Kind: encounter.KindMonster, Position: captainCell, Knows: knows},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		return err
	}

	s.Require().ErrorIs(setup([]encounter.DoorID{"cellar-hatch"}), encounter.ErrNoDoor)
	s.Require().NoError(setup([]encounter.DoorID{tombVault}))
	s.Require().NoError(setup(nil))
}

// TestConstructionRefusesADeadEndingAndABadExit: an ending that can never
// fire is the liveness hole ErrNoEnding exists for, and an exit nobody can
// stand on is the same hole one noun over.
func (s *HoldingsSuite) TestConstructionRefusesADeadEndingAndABadExit() {
	setup := func(mutate func(*encounter.SetupInput)) error {
		in := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: s.witness,
			Field:   heirloomField(),
			Members: s.cast(false),
			Endings: []encounter.EndingInput{recoverEnding()},
		}
		mutate(in)
		_, err := encounter.NewEncounter(in)
		return err
	}

	s.Run("the authored fixture is legal", func() {
		s.Require().NoError(setup(func(*encounter.SetupInput) {}))
	})

	s.Run("an ending naming an exit the field does not declare", func() {
		err := setup(func(in *encounter.SetupInput) {
			in.Endings = []encounter.EndingInput{{
				Key: recovered, Trigger: encounter.TriggerExitedHolding{Exit: "back-door", Item: heirloom},
			}}
		})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
		s.Require().Contains(err.Error(), "back-door")
	})

	s.Run("an ending naming a prop nobody can pick up", func() {
		err := setup(func(in *encounter.SetupInput) {
			in.Endings = []encounter.EndingInput{{
				Key: recovered, Trigger: encounter.TriggerExitedHolding{Exit: frontGate, Item: pillar},
			}}
		})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
		s.Require().Contains(err.Error(), "holdable")
	})

	s.Run("an ending naming no item at all", func() {
		err := setup(func(in *encounter.SetupInput) {
			in.Endings = []encounter.EndingInput{{
				Key: recovered, Trigger: encounter.TriggerExitedHolding{Exit: frontGate},
			}}
		})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("an exit standing on nothing", func() {
		err := setup(func(in *encounter.SetupInput) {
			field := in.Field
			field.Exits = append(append([]encounter.FieldExit(nil), field.Exits...),
				encounter.FieldExit{ID: "nowhere", At: spatial.Position{X: 99, Y: 99}})
			in.Field = field
		})
		s.Require().ErrorIs(err, encounter.ErrNoExit)
	})

	s.Run("two exits sharing an id", func() {
		err := setup(func(in *encounter.SetupInput) {
			field := in.Field
			field.Exits = append(append([]encounter.FieldExit(nil), field.Exits...),
				encounter.FieldExit{ID: frontGate, At: partnerCell})
			in.Field = field
		})
		s.Require().ErrorIs(err, encounter.ErrNoExit)
	})

	s.Run("an exit with no id", func() {
		err := setup(func(in *encounter.SetupInput) {
			field := in.Field
			field.Exits = []encounter.FieldExit{{At: raiderCell}}
			in.Field = field
		})
		s.Require().ErrorIs(err, encounter.ErrNoExit)
	})

	s.Run("a holdable prop with no id", func() {
		// Without an id every atlas would advertise a thing anybody can
		// pick up while no HoldInput could ever name it. dungeonspec
		// refuses this at the file; the composition refuses it again, so a
		// host assembling a field by hand cannot produce one either
		// (Copilot, PR #1497 review).
		err := setup(func(in *encounter.SetupInput) {
			field := in.Field
			nameless := holdableProp("", "dnd5e:props:decoy", partnerCell)
			field.Props = append(append([]encounter.PropInput(nil), field.Props...), nameless)
			in.Field = field
		})
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Require().Contains(err.Error(), "holdable and has no id")
	})

	s.Run("an UNholdable prop with no id is ordinary scenery", func() {
		s.Require().NoError(setup(func(in *encounter.SetupInput) {
			field := in.Field
			plain := encounter.PropInput{
				Ref: "dnd5e:props:candles", At: partnerCell,
				BlocksMovement: no(), BlocksLineOfSight: no(),
			}
			field.Props = append(append([]encounter.PropInput(nil), field.Props...), plain)
			in.Field = field
		}), "an id is optional for everything nobody can pick up")
	})

	s.Run("two props sharing an id", func() {
		err := setup(func(in *encounter.SetupInput) {
			field := in.Field
			props := append([]encounter.PropInput(nil), field.Props...)
			props = append(props, holdableProp(heirloom, "dnd5e:props:decoy", partnerCell))
			field.Props = props
			in.Field = field
		})
		s.Require().ErrorIs(err, encounter.ErrNoField)
	})
}

// TestLoadRefusesABrokenExitedHoldingEnding is the trust boundary: persisted
// bytes that are not an ending at all are refused rather than loaded into an
// encounter that can never end.
func (s *HoldingsSuite) TestLoadRefusesABrokenExitedHoldingEnding() {
	enc := s.open(false, recoverEnding())
	data := enc.ToData()
	s.Require().Len(data.Endings, 1)
	s.Require().Equal("exited_holding", data.Endings[0].Kind)
	s.Require().Equal(frontGate, data.Endings[0].Exit)
	s.Require().Equal(heirloom, data.Endings[0].Item)

	load := func(mutate func(*encounter.EncounterData)) error {
		blob := enc.ToData()
		mutate(&blob)
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data:  blob,
			Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: s.witness,
		})
		return err
	}

	s.Require().NoError(load(func(*encounter.EncounterData) {}))
	s.Require().ErrorIs(load(func(d *encounter.EncounterData) { d.Endings[0].Exit = "" }), encounter.ErrNoEnding)
	s.Require().ErrorIs(load(func(d *encounter.EncounterData) { d.Endings[0].Item = "" }), encounter.ErrNoEnding)
	s.Require().ErrorIs(load(func(d *encounter.EncounterData) {
		d.Endings[0].Exit = "back-door"
	}), encounter.ErrNoEnding)
}

// TestAnInertKnowledgeLinkTransfersNothingVisible: knowing an ORDINARY door
// is legal and inert ([MemberInput.Knows]). Looting it writes the holding and
// causes no reveal, because there is nothing to reveal.
func (s *HoldingsSuite) TestAnInertKnowledgeLinkTransfersNothingVisible() {
	plain := encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 6, 6)},
		Walls:   seamWallExcept(2, 6, 3),
		Doors: []encounter.DoorInput{{
			ID: "plain-door", Edges: doorEdgesAcross(2, 3), State: encounter.DoorIsOpen(),
		}},
		Exits: []encounter.FieldExit{{ID: frontGate, At: raiderCell}},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: plain,
		Members: []encounter.MemberInput{
			{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
			{ID: captain, Kind: encounter.KindMonster, Position: partnerCell,
				Knows: []encounter.DoorID{"plain-door"}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err, "a field with no concealment needs no concealment capabilities")

	s.standing.down = []encounter.MemberID{captain}
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	before, err := enc.AtlasFor(raider)
	s.Require().NoError(err)
	_, err = enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err, "an inert link is inert, not an error")

	after, err := enc.AtlasFor(raider)
	s.Require().NoError(err)
	s.Require().Equal(before, after)
	s.Require().Empty(s.beatsOfKind(enc, raider, "door_revealed"))
	s.Require().Len(s.beatsOfKind(enc, raider, "looted"), 1)
}

// TestAHeldPropIsGoneForABlindMemberToo: the taken filter lives in Atlas,
// not in AtlasFor, so it applies on the path AtlasFor short-circuits — every
// plain dungeon with no concealment at all.
func (s *HoldingsSuite) TestAHeldPropIsGoneForABlindMemberToo() {
	plain := encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 6, 6)},
		Props:   []encounter.PropInput{holdableProp(chalice, "dnd5e:props:chalice", partnerCell)},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: plain,
		Members: []encounter.MemberInput{
			{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
			{ID: partner, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 3}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Hold(&encounter.HoldInput{Member: raider, Target: chalice})
	s.Require().NoError(err)

	for _, member := range []core.EntityID{raider, partner} {
		atlas, err := enc.AtlasFor(member)
		s.Require().NoError(err)
		_, present := propInAtlas(atlas, chalice)
		s.Require().False(present, "%s still sees a prop nobody is standing next to", member)
	}
}
