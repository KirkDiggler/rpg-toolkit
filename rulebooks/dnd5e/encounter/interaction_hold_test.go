// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// The hold tests are rpg-project#924 (reserve before dice) on the
// InteractionSuite's gate: alice and billy before the captain, the two
// claims on the front gate, and two lines that read no interaction — a plain
// greeting and a checked threat.

// heldGateLines is the two claims and the two lines that touch no
// interaction.
func heldGateLines() []encounter.Option {
	return append(gateClaims(),
		encounter.Option{ID: "greet", Say: "Evening.", Success: []encounter.Answer{{Weight: 1, Say: "Evening."}}},
		encounter.Option{
			ID: "stand-aside", Say: "Stand aside.",
			Check:   []encounter.CheckApproach{{Ability: "intimidation", DC: 14}},
			Success: []encounter.Answer{{Weight: 1, Say: "Go on, then."}},
		},
	)
}

// heldGate is the gate with the held lines, built or the test fails.
func (s *InteractionSuite) heldGate() *encounter.Encounter {
	enc, err := s.gate(heldGateLines(), encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
	s.Require().NoError(err)
	return enc
}

// reserve reserves one of the captain's lines for who at the revision it is
// offered at now.
func (s *InteractionSuite) reserve(enc *encounter.Encounter, who core.EntityID, option string) *encounter.ReserveOutput {
	out, err := enc.Reserve(&encounter.ReserveInput{
		Actor: who, Target: goblin, OptionID: option, Revision: offeredRevision(enc, who, goblin, option), Range: speakReach,
	})
	s.Require().NoError(err)
	return out
}

// offer is the captain's offer to one speaker, by line id.
func (s *InteractionSuite) offer(enc *encounter.Encounter, who core.EntityID) map[string]encounter.OfferedOption {
	offered, err := enc.OptionsFor(who, goblin)
	s.Require().NoError(err)
	out := map[string]encounter.OfferedOption{}
	for _, o := range offered {
		out[o.ID] = o
	}
	return out
}

// speakAt speaks one of the captain's lines at a given revision.
func (s *InteractionSuite) speakAt(
	enc *encounter.Encounter, who core.EntityID, option, skill string, beaten bool, revision uint64,
) error {
	in := &encounter.SpeakInput{
		Range: speakReach, Actor: who, Target: goblin, OptionID: option, Skill: skill,
		Beaten: beaten, Roller: rollsLowest{}, Revision: revision,
	}
	if skill != "" {
		in.DC, in.Total = 11, 8
	}
	_, err := enc.Speak(s.ctx, in)
	return err
}

// TestLinesCarryTheirInteractionsRevision: a line touching the gate is
// offered with the gate's id and revision; a line touching nothing with
// neither; a fresh gate starts at one, and a verdict that moves the gate
// moves its revision.
func (s *InteractionSuite) TestLinesCarryTheirInteractionsRevision() {
	enc := s.heldGate()
	offer := s.offer(enc, alice)
	s.Equal(theFrontGate, offer["guest"].Interaction)
	s.Equal(uint64(1), offer["guest"].Revision, "a fresh gate is at revision one")
	s.Empty(offer["greet"].Interaction)
	s.Zero(offer["greet"].Revision, "a line touching no interaction carries no revision")

	s.Require().NoError(s.claim(enc, alice, "guest", false))
	s.Greater(s.offer(enc, alice)["troupe"].Revision, offer["troupe"].Revision, "the warned gate moved its revision")
	s.Equal(s.saved(enc).Revision, s.offer(enc, alice)["troupe"].Revision, "the offer is the record's revision")
}

// TestAReservedGateHoldsItsLinesForEveryone is ruling one of #924: while
// alice's claim waits on her roll, every line reading the gate is pending —
// shown, naming alice, refused if sent — for billy and for alice herself;
// lines that read no interaction stay open and are accepted.
func (s *InteractionSuite) TestAReservedGateHoldsItsLinesForEveryone() {
	enc := s.heldGate()
	reserved := s.reserve(enc, alice, "guest")
	s.Equal(theFrontGate, reserved.Interaction)

	for _, who := range []core.EntityID{billy, alice} {
		offer := s.offer(enc, who)
		for _, claim := range []string{"guest", "troupe"} {
			s.Equal(alice, offer[claim].PendingOn, "%s sees %s pending on alice", who, claim)
			s.True(offer[claim].Pending)
		}
		s.False(offer["greet"].Pending, "%s may still greet", who)
		s.False(offer["stand-aside"].Pending, "%s may still threaten", who)
	}

	revision := s.offer(enc, billy)["troupe"].Revision
	s.ErrorIs(s.speakAt(enc, billy, "troupe", "persuasion", true, revision), encounter.ErrReserved,
		"a pending line is refused at the current revision")
	s.ErrorIs(s.speakAt(enc, alice, "troupe", "persuasion", true, revision), encounter.ErrReserved,
		"and the holder's other claim waits on her own roll")
	s.Equal("fresh", s.saved(enc).Blob.State, "nothing moved the gate")

	s.Require().NoError(s.speakAt(enc, billy, "greet", "", false, 0), "a plain line that reads no interaction is open")
	s.Require().NoError(s.speakAt(enc, billy, "stand-aside", "intimidation", true, 0), "and so is a checked one")
}

// TestAPendingLineIsOneThatWasOffered: a claim whose other requirement the
// speaker does not meet stays absent while the gate is held — a reservation
// never reveals a line the speaker was not offered (the probe law).
func (s *InteractionSuite) TestAPendingLineIsOneThatWasOffered() {
	lines := heldGateLines()
	lines[1].Requires = append(lines[1].Requires, encounter.TriggerFact{Fact: "feast-tonight"})
	enc, err := s.gate(lines, encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
	s.Require().NoError(err)
	s.Require().NotContains(s.offer(enc, billy), "troupe", "precondition: billy never heard of the feast")

	s.reserve(enc, alice, "guest")
	offer := s.offer(enc, billy)
	s.True(offer["guest"].Pending)
	s.NotContains(offer, "troupe", "the held gate shows billy no claim he was not offered")
}

// TestAStaleReplyIsRefusedWithNothingSpent is ruling two of #924: a reply
// from an older revision is refused — Speak and Reserve alike — with no
// beat, no roll consumed, no reservation and the gate where it was; a reply
// naming no revision at all is refused the same way.
func (s *InteractionSuite) TestAStaleReplyIsRefusedWithNothingSpent() {
	enc := s.heldGate()
	offered := s.offer(enc, alice)["troupe"].Revision
	s.Require().NoError(s.claim(enc, billy, "guest", false), "billy's failed claim moves the conversation")

	before, err := enc.NextStorySeq()
	s.Require().NoError(err)
	saved := s.saved(enc)

	s.ErrorIs(s.speakAt(enc, alice, "troupe", "persuasion", false, offered), encounter.ErrStaleOption,
		"the troupe claim offered from `fresh` is not re-read against `warned-guest`")
	s.ErrorIs(s.speakAt(enc, alice, "troupe", "persuasion", false, 0), encounter.ErrStaleOption,
		"a reply naming no revision is refused")
	_, err = enc.Reserve(&encounter.ReserveInput{Actor: alice, Target: goblin, OptionID: "troupe", Revision: offered, Range: speakReach})
	s.ErrorIs(err, encounter.ErrStaleOption, "nor is a stale line reserved")

	after, err := enc.NextStorySeq()
	s.Require().NoError(err)
	s.Equal(before, after, "no beat was written")
	s.Equal(saved, s.saved(enc), "the gate, its revision and its reservation are as they were")
	s.Nil(s.saved(enc).Reservation)
}

// TestAReservationSurvivesAReloadAndSettlesOnce: a reservation is saved and
// loaded; the holder's Speak at the reserved revision is the settle — the
// verdict advances the gate once and the reservation clears; the revision
// moved at the reserve, the advance and the settle; and a second settle of a
// line still offered at the new state is refused stale.
func (s *InteractionSuite) TestAReservationSurvivesAReloadAndSettlesOnce() {
	lines := heldGateLines()
	lines[0].Requires = []encounter.Trigger{encounter.TriggerInteraction{
		Interaction: theFrontGate, In: []string{"fresh", "warned-guest", "warned-troupe"},
	}} // the guest claim stays offered once warned, so a second settle meets it
	enc, err := s.gate(lines, encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
	s.Require().NoError(err)
	at := s.offer(enc, alice)["guest"].Revision
	reserved := s.reserve(enc, alice, "guest")
	s.Equal(at+1, reserved.Revision, "placing the reservation moves the revision")

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var decoded encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	reloaded, err := s.reloaded(decoded)
	s.Require().NoError(err)
	s.Equal(&encounter.InteractionReservation{Holder: alice, Target: goblin, Option: "guest", Revision: at, Reach: speakReach},
		s.saved(reloaded).Reservation, "the reservation comes back as placed")

	s.ErrorIs(s.speakAt(reloaded, alice, "guest", "persuasion", false, reserved.Revision), encounter.ErrStaleOption,
		"the settle carries the reserved revision, not the one after it")
	s.Require().NoError(s.speakAt(reloaded, alice, "guest", "persuasion", false, at), "the settle lands")
	s.Equal("warned-guest", s.saved(reloaded).Blob.State, "the verdict advanced the gate")
	s.Nil(s.saved(reloaded).Reservation, "and the reservation cleared")
	s.Equal(at+3, s.saved(reloaded).Revision, "reserve, advance and settle each moved the revision")

	settled := s.saved(reloaded)
	s.Require().Contains(s.offer(reloaded, alice), "guest", "precondition: the guest claim is still offered when warned")
	s.ErrorIs(s.speakAt(reloaded, alice, "guest", "persuasion", false, at), encounter.ErrStaleOption,
		"a second settle at the reserved revision is a reply from an old menu")
	s.Equal(settled, s.saved(reloaded), "and advances nothing")
	s.False(s.offer(reloaded, billy)["guest"].Pending, "the gate's lines return")
}

// TestReleaseReturnsTheLines: releasing the holder's reservation advances
// nothing, moves the revision, tells the holder's witnesses, and the lines
// return to every speaker; a member holding none is refused.
func (s *InteractionSuite) TestReleaseReturnsTheLines() {
	enc := s.heldGate()
	reserved := s.reserve(enc, alice, "guest")

	out, err := enc.ReleaseReservation(&encounter.ReleaseReservationInput{Member: alice})
	s.Require().NoError(err)
	s.Equal([]encounter.InteractionRevision{{Interaction: theFrontGate, Revision: reserved.Revision + 1}}, out.Released)
	s.Equal("fresh", s.saved(enc).Blob.State, "a release is no verdict")
	s.Nil(s.saved(enc).Reservation)

	for _, who := range []core.EntityID{alice, billy} {
		offer := s.offer(enc, who)
		s.False(offer["guest"].Pending, "%s is offered the guest claim again", who)
		s.Equal(reserved.Revision+1, offer["guest"].Revision)
	}
	reader := &OptionTestSuite{}
	reader.SetT(s.T())
	held := reader.beatsOfKind(enc, billy, encounter.BeatInteractionHeld)
	s.Require().NotEmpty(held, "billy saw the gate wait and stop waiting")
	s.Equal(false, held[len(held)-1]["held"])
	s.Equal(string(alice), held[len(held)-1]["holder"])

	_, err = enc.ReleaseReservation(&encounter.ReleaseReservationInput{Member: alice})
	s.ErrorIs(err, encounter.ErrNoReservation)
	s.Require().NoError(s.claim(enc, billy, "guest", true), "the released claim is spoken by somebody else")
}

// TestOneReservationPerInteraction: a second reserve on a held gate is
// refused, by another speaker and by the holder; a plain line never waits
// on a roll and cannot be reserved; a line that reads no interaction holds
// nothing.
func (s *InteractionSuite) TestOneReservationPerInteraction() {
	enc := s.heldGate()
	s.reserve(enc, alice, "guest")
	for _, who := range []core.EntityID{billy, alice} {
		_, err := enc.Reserve(&encounter.ReserveInput{
			Actor: who, Target: goblin, OptionID: "troupe", Revision: offeredRevision(enc, who, goblin, "troupe"), Range: speakReach,
		})
		s.ErrorIs(err, encounter.ErrReserved, "%s cannot reserve a held gate", who)
	}

	_, err := enc.Reserve(&encounter.ReserveInput{Actor: billy, Target: goblin, OptionID: "greet", Range: speakReach})
	s.ErrorIs(err, encounter.ErrNoOption, "a plain line never waits")
	out := s.reserve(enc, billy, "stand-aside")
	s.Empty(out.Interaction, "a line touching no interaction holds nothing")
}

// TestALineTouchesOneInteraction: a line reading one interaction and
// advancing another is refused at construction — one revision per line.
func (s *InteractionSuite) TestALineTouchesOneInteraction() {
	lines := gateClaims()
	lines[0].Advance.Interaction = "side-door"
	recipe := frontGateRecipe()
	recipe.Ref.ID = "side-door"
	_, err := s.gate(lines,
		encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()},
		encounter.InteractionInput{ID: "side-door", Recipe: recipe},
	)
	s.ErrorIs(err, encounter.ErrBadMember)
}

// TestASavedReservationNobodyCouldPlaceIsRefused: a saved reservation with
// no holder, a holder the run lacks, a line the creature does not carry, or
// a revision not below the gate's is refused with ErrInvalidData.
func (s *InteractionSuite) TestASavedReservationNobodyCouldPlaceIsRefused() {
	enc := s.heldGate()
	s.reserve(enc, alice, "guest")
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)

	for name, change := range map[string]func(*encounter.InteractionReservation){
		"no holder":            func(r *encounter.InteractionReservation) { r.Holder = "" },
		"a holder not in it":   func(r *encounter.InteractionReservation) { r.Holder = "nobody" },
		"a line not carried":   func(r *encounter.InteractionReservation) { r.Option = "bribe" },
		"a revision not below": func(r *encounter.InteractionReservation) { r.Revision = 99 },
	} {
		s.Run(name, func() {
			var copied encounter.EncounterData
			s.Require().NoError(json.Unmarshal(raw, &copied))
			change(copied.Interactions[0].Reservation)
			_, err := s.reloaded(copied)
			s.ErrorIs(err, encounter.ErrInvalidData)
		})
	}
}

// TestATargetsExitReleasesTheHold: the captain leaves while alice's claim
// waits; the hold is released with its held beat, the revision moves, and
// the saved record loads.
func (s *InteractionSuite) TestATargetsExitReleasesTheHold() {
	enc := s.heldGate()
	reserved := s.reserve(enc, alice, "guest")
	_, err := enc.Exit(&encounter.ExitInput{Member: goblin})
	s.Require().NoError(err, "the captain leaves")
	s.Nil(s.saved(enc).Reservation, "a line said to somebody gone waits on nothing")
	s.Greater(s.saved(enc).Revision, reserved.Revision, "and the conversation moved")
	s.Equal(false, s.lastHeld(enc, alice)["held"], "alice is told the line stopped waiting")

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var decoded encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	_, err = s.reloaded(decoded)
	s.NoError(err, "a run whose captain left with a held line still loads")
}

// TestAHoldersExitDropsTheHold: the holder leaves; nobody is left to roll,
// so the gate's lines return to billy.
func (s *InteractionSuite) TestAHoldersExitDropsTheHold() {
	enc := s.heldGate()
	s.reserve(enc, alice, "guest")
	_, err := enc.Exit(&encounter.ExitInput{Member: alice})
	s.Require().NoError(err)
	s.Nil(s.saved(enc).Reservation)
	s.False(s.offer(enc, billy)["guest"].Pending)
}

// lastHeld is the last interaction-held beat a member was told.
func (s *InteractionSuite) lastHeld(enc *encounter.Encounter, who core.EntityID) map[string]any {
	reader := &OptionTestSuite{}
	reader.SetT(s.T())
	held := reader.beatsOfKind(enc, who, encounter.BeatInteractionHeld)
	s.Require().NotEmpty(held, "%s was told of the hold", who)
	return held[len(held)-1]
}

// farGate is the yard with the captain far across it from alice: in sight,
// beyond any speaking reach.
func (s *InteractionSuite) farGate() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:       openAir(),
			Regions:      []encounter.RegionInput{rectRegion("yard", 0, 0, 20, 20)},
			Interactions: []encounter.InteractionInput{{ID: theFrontGate, Recipe: frontGateRecipe()}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 4, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 15, Y: 15}, Options: heldGateLines()},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: (&OptionTestSuite{}).capabilities(),
	})
	s.Require().NoError(err)
	return enc
}

// TestTheDoorsAgreeOnReach: Speak, ChooseOption and Reserve all refuse a
// speaker out of reach, so no reservation is placed whose settle is refused.
func (s *InteractionSuite) TestTheDoorsAgreeOnReach() {
	enc := s.farGate()
	rev := offeredRevision(enc, alice, goblin, "guest")
	reply := &encounter.ChooseOptionInput{Actor: alice, Target: goblin, OptionID: "guest", Revision: rev, Range: speakReach}

	s.ErrorIs(s.speakAt(enc, alice, "guest", "persuasion", false, rev), encounter.ErrOutOfRange)
	_, err := enc.ChooseOption(reply)
	s.ErrorIs(err, encounter.ErrOutOfRange)
	_, err = enc.Reserve(reply)
	s.ErrorIs(err, encounter.ErrOutOfRange)
	s.Nil(s.saved(enc).Reservation)
}

// wallGate is the option suite's walled room with the front gate: alice and
// the captain in sight of each other, billy behind a wall row who sees
// neither.
func (s *InteractionSuite) wallGate(participation encounter.Participation) *encounter.Encounter {
	caps := (&OptionTestSuite{}).capabilities()
	if participation != nil {
		caps.Participation = participation
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:       encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions:      []encounter.RegionInput{rectRegion("yard", 0, 0, 12, 12)},
			Props:        wallRow(6, 4, 8),
			Interactions: []encounter.InteractionInput{{ID: theFrontGate, Recipe: frontGateRecipe()}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, Options: heldGateLines()},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: caps,
	})
	s.Require().NoError(err)
	return enc
}

// TestTheDoorsAgreeOnWitnessing: billy behind the wall is refused by every
// door as unwitnessed — and is told a held line is pending without being
// told whose roll it waits on, since he cannot perceive alice; alice sees
// her own name.
func (s *InteractionSuite) TestTheDoorsAgreeOnWitnessing() {
	enc := s.wallGate(nil)
	rev := offeredRevision(enc, billy, goblin, "guest")
	reply := &encounter.ChooseOptionInput{Actor: billy, Target: goblin, OptionID: "guest", Revision: rev, Range: 20}
	_, err := enc.ChooseOption(reply)
	s.ErrorIs(err, encounter.ErrUnwitnessed)
	_, err = enc.Reserve(reply)
	s.ErrorIs(err, encounter.ErrUnwitnessed)

	s.reserve(enc, alice, "guest")
	billys := s.offer(enc, billy)["guest"]
	s.True(billys.Pending, "billy's guest claim waits")
	s.Empty(billys.PendingOn, "on somebody billy cannot perceive, who is not named to him")
	s.Equal(alice, s.offer(enc, alice)["guest"].PendingOn, "alice is told it waits on her own roll")
}

// TestAHoldIsReleasedWhenItsTargetFalls: the captain goes down while alice's
// claim waits; the next participation pass releases it with its beat.
func (s *InteractionSuite) TestAHoldIsReleasedWhenItsTargetFalls() {
	down := &downList{}
	enc := s.wallGate(down)
	reserved := s.reserve(enc, alice, "guest")
	down.down = []encounter.MemberID{goblin}
	_, err := enc.Step(&encounter.StepInput{Member: billy, To: spatial.Position{X: 6, Y: 11}, EndWalk: true, Reach: speakReach})
	s.Require().NoError(err)
	s.Nil(s.saved(enc).Reservation, "a down captain answers nothing, so nothing waits on him")
	s.Greater(s.saved(enc).Revision, reserved.Revision)
	s.Equal(false, s.lastHeld(enc, alice)["held"])
}

// TestAHoldIsReleasedWhenTheHolderWalksOutOfReach: alice steps away from
// the captain beyond the reach she reserved within; the step's sweep
// releases the hold.
func (s *InteractionSuite) TestAHoldIsReleasedWhenTheHolderWalksOutOfReach() {
	enc := s.wallGate(nil)
	_, err := enc.Reserve(&encounter.ReserveInput{
		Actor: alice, Target: goblin, OptionID: "guest", Revision: offeredRevision(enc, alice, goblin, "guest"), Range: 2,
	})
	s.Require().NoError(err)
	for _, x := range []float64{5, 4, 3, 2} {
		_, err := enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: x, Y: 2}, EndWalk: x == 2, Reach: speakReach})
		s.Require().NoError(err)
	}
	s.Nil(s.saved(enc).Reservation, "out of the reach it was reserved within, the settle could not land")
	s.False(s.offer(enc, alice)["guest"].Pending)
}

// TestASavedReservationOnALineThatDoesNotTouchItIsRefused: a saved
// reservation naming a line that reads no interaction is refused.
func (s *InteractionSuite) TestASavedReservationOnALineThatDoesNotTouchItIsRefused() {
	enc := s.heldGate()
	s.reserve(enc, alice, "guest")
	data := enc.ToData()
	data.Interactions[0].Reservation.Option = "stand-aside"
	_, err := s.reloaded(data)
	s.ErrorIs(err, encounter.ErrInvalidData)
}
