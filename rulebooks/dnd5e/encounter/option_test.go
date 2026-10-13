// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// OptionTestSuite is the one social verb on a real board (rpg-project#569,
// the mad king use case §2, rulings R1–R3 and R9): an option is offered only
// when its `requires` holds for the SPEAKER, a beaten check lands the deed of
// its SKILL on exactly who saw it, and the creature answers from the option's
// own success or failure rows.
type OptionTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *OptionTestSuite) SetupTest() { s.ctx = context.Background() }

func TestOptionSuite(t *testing.T) { suite.Run(t, new(OptionTestSuite)) }

// The props and facts this suite's room is written with.
const (
	thePurse     = encounter.PropID("purse")
	theAltar     = encounter.PropID("altar")
	feastTonight = encounter.FactID("feast-tonight")
	cultistFact  = encounter.FactID("king-is-cultist")
)

// goblinOptions is the cook's three lines from the use case, on a goblin: one
// always offered, one waiting on a fact the speaker must know, and one waiting
// on a purse the speaker must hold — plus a threat, and a line that rolls a
// skill with no deed.
func goblinOptions() []encounter.Option {
	return []encounter.Option{
		{
			ID: "gossip", Say: "Busy night?",
			Check:   []encounter.CheckApproach{{Ability: "persuasion", DC: 10}},
			Success: []encounter.Answer{{Weight: 1, Say: "Feast for the whole court.", Fact: feastTonight}},
			Failure: []encounter.Answer{{Weight: 1, Say: "Out of my kitchen."}},
		},
		{
			ID: "ashford", Say: "What did he do to Ashford?",
			Requires: encounter.TriggerFact{Fact: feastTonight},
			Check:    []encounter.CheckApproach{{Ability: "persuasion", DC: 12}},
			Success:  []encounter.Answer{{Weight: 1, Say: "Burned it.", Fact: campFact}},
		},
		{
			ID: "bribe", Say: "Ten gold says you never saw us.",
			Requires: encounter.TriggerHolds{Prop: thePurse},
			Check:    []encounter.CheckApproach{{Ability: "persuasion", DC: 8}},
			Success:  []encounter.Answer{{Weight: 1, Say: "Saw who?"}},
		},
		{
			ID: "threaten", Say: "Stand aside or bleed.",
			Check:   []encounter.CheckApproach{{Ability: "intimidation", DC: 12}},
			Success: []encounter.Answer{{Weight: 1, Say: "…go on, then.", Flee: true}},
			Failure: []encounter.Answer{{Weight: 1, Say: "You'll bleed first."}},
		},
		{
			ID: "sermon", Say: "The Pale One watches you.",
			Check:   []encounter.CheckApproach{{Ability: "religion", DC: 12}},
			Success: []encounter.Answer{{Weight: 1, Say: "…how do you know that name?"}},
		},
	}
}

// altarOptions is the use case's altar: a knowledge check on a prop.
func altarOptions() []encounter.Option {
	return []encounter.Option{{
		ID: "examine", Say: "Examine the altar.",
		Check:   []encounter.CheckApproach{{Ability: "religion", DC: 12}},
		Success: []encounter.Answer{{Weight: 1, Say: "The sigils are the Pale One's.", Fact: cultistFact}},
		Failure: []encounter.Answer{{Weight: 1, Say: "Old stone, old candles."}},
	}}
}

// capabilities is everything this suite's room asks of its host.
func (s *OptionTestSuite) capabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Actors: encounter.Actors{
			Striker:   passStriker{},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// scene is the intimidate suite's room with options on the goblin and on an
// altar: alice and the goblin in sight of each other, a purse at alice's
// feet, the altar beside her, and billy behind a wall row who can see none
// of it.
func (s *OptionTestSuite) scene() *encounter.Encounter {
	altar := encounter.PropInput{
		ID: theAltar, Ref: "test:props:altar", At: spatial.Position{X: 7, Y: 2},
		BlocksMovement: no(), BlocksLineOfSight: no(), Options: altarOptions(),
	}
	props := append(wallRow(6, 4, 8),
		holdableProp(thePurse, "test:props:purse", spatial.Position{X: 5, Y: 2}), altar)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)},
			Props:   props,
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, Options: goblinOptions()},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: s.capabilities(),
	})
	s.Require().NoError(err)

	return enc
}

// ids is the option ids a read offered, in order.
func ids(options []encounter.Option) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		out = append(out, o.ID)
	}

	return out
}

// speak is one Speak with the numbers a scene does not care about filled in.
func (s *OptionTestSuite) speak(
	enc *encounter.Encounter, option, skill string, beaten bool,
) (*encounter.SpeakOutput, error) {
	return enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: alice, Target: goblin, OptionID: option, Skill: skill,
		Beaten: beaten, DC: 12, Total: 15, Roller: rollsLowest{},
	})
}

// heldDeed reports the deed an observer holds about the actor under a verb.
func (s *OptionTestSuite) heldDeed(enc *encounter.Encounter, observer, actor encounter.MemberID, verb string) bool {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Channel == deed.Channel && h.Subject == deed.Subject(actor, verb) {
			return true
		}
	}

	return false
}

// beatsOfKind is every beat of one kind a member's story carries.
func (s *OptionTestSuite) beatsOfKind(enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}

	return out
}

// R9: an option waiting on a fact is hidden from a speaker who does not know
// it, and offered the moment THAT speaker learns it — learning it is what the
// gossip line teaches its witnesses.
func (s *OptionTestSuite) TestAFactOptionShowsOnlyOnceTheSpeakerKnowsIt() {
	enc := s.scene()

	before, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"gossip", "threaten", "sermon"}, ids(before), "ashford waits on the feast, bribe on the purse")

	_, err = s.speak(enc, "gossip", "persuasion", true)
	s.Require().NoError(err)

	after, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"gossip", "ashford", "threaten", "sermon"}, ids(after), "alice heard about the feast")

	billys, err := enc.OptionsFor(billy, goblin)
	s.Require().NoError(err)
	s.NotContains(ids(billys), "ashford", "billy was behind the wall and knows nothing about a feast")
}

// `holds` is read, never fired: the bribe is offered while the speaker
// carries the purse, and only then.
func (s *OptionTestSuite) TestAHoldsOptionShowsOnlyWhileTheSpeakerHoldsTheProp() {
	enc := s.scene()

	before, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.NotContains(ids(before), "bribe")

	_, err = enc.Hold(&encounter.HoldInput{Member: alice, Target: thePurse})
	s.Require().NoError(err)

	held, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Contains(ids(held), "bribe", "alice is carrying the purse")

	billys, err := enc.OptionsFor(billy, goblin)
	s.Require().NoError(err)
	s.NotContains(ids(billys), "bribe", "the purse is alice's, not billy's")
}

// A creature nobody wrote a line for offers none, and that is not an error:
// the offer side decides what an empty list means.
func (s *OptionTestSuite) TestACreatureWithNoOptionsOffersNone() {
	enc := s.scene()

	none, err := enc.OptionsFor(goblin, alice)
	s.Require().NoError(err)
	s.Empty(none)

	_, err = enc.OptionsFor(alice, "nobody")
	s.ErrorIs(err, encounter.ErrNotMember)
	_, err = enc.OptionsFor("nobody", goblin)
	s.ErrorIs(err, encounter.ErrNotMember)
}

// R3: a beaten Intimidation check lands `intimidate` on every witness and on
// nobody who did not see it.
func (s *OptionTestSuite) TestABeatenIntimidationLandsTheDeedOnWitnesses() {
	enc := s.scene()

	out, err := s.speak(enc, "threaten", "intimidation", true)
	s.Require().NoError(err)
	s.Equal([]encounter.MemberID{alice, goblin}, out.Witnesses)
	s.Equal(encounter.DeedIntimidate, out.Deed)

	s.True(s.heldDeed(enc, goblin, alice, encounter.DeedIntimidate), "the goblin saw the threat")
	s.True(s.heldDeed(enc, alice, alice, encounter.DeedIntimidate), "and so did alice, who can see herself")
	s.False(s.heldDeed(enc, billy, alice, encounter.DeedIntimidate), "billy is behind the wall")
}

// R3: Persuasion lands `persuade`.
func (s *OptionTestSuite) TestABeatenPersuasionLandsPersuade() {
	enc := s.scene()

	out, err := s.speak(enc, "gossip", "persuasion", true)
	s.Require().NoError(err)
	s.Equal(encounter.DeedPersuade, out.Deed)
	s.True(s.heldDeed(enc, goblin, alice, encounter.DeedPersuade))
}

// R3: a skill that is neither lands no deed at all, however well it went —
// "Religion and Insight land no deed".
func (s *OptionTestSuite) TestABeatenReligionLandsNoDeed() {
	enc := s.scene()

	out, err := s.speak(enc, "sermon", "religion", true)
	s.Require().NoError(err)
	s.Empty(out.Deed)
	s.False(s.heldDeed(enc, goblin, alice, encounter.DeedIntimidate))
	s.False(s.heldDeed(enc, goblin, alice, encounter.DeedPersuade))
}

// A missed check lands no deed and answers from the FAILURE rows.
func (s *OptionTestSuite) TestAMissedCheckPicksFromFailure() {
	enc := s.scene()

	out, err := s.speak(enc, "threaten", "intimidation", false)
	s.Require().NoError(err)
	s.Empty(out.Deed)
	s.False(s.heldDeed(enc, goblin, alice, encounter.DeedIntimidate), "a threat nobody was frightened by")
	s.Require().NotNil(out.Answered)
	s.Equal("You'll bleed first.", out.Answered.Answer.Say)

	answered := s.beatsOfKind(enc, goblin, encounter.BeatAnswered)
	s.Require().Len(answered, 1)
	s.Equal("threaten", answered[0]["option"], "the answer names the option it answered")
	s.Equal(string(encounter.AnswerFailure), answered[0]["key"])
	s.Equal("You'll bleed first.", answered[0]["say"])
}

// A beaten check answers from the SUCCESS rows, and a `flee` row lands the
// creature's own memory of having been made to run.
func (s *OptionTestSuite) TestABeatenCheckPicksFromSuccess() {
	enc := s.scene()

	out, err := s.speak(enc, "threaten", "intimidation", true)
	s.Require().NoError(err)
	s.Require().NotNil(out.Answered)
	s.Equal(string(encounter.AnswerSuccess), string(out.Answered.Key))
	s.True(out.Answered.Answer.Flee)
	s.True(s.heldDeed(enc, goblin, alice, encounter.DeedFled), "the goblin remembers who made it run")
}

// An option with no row for the verdict that happened answers nothing and
// writes no Answered beat — absent means absent.
func (s *OptionTestSuite) TestAnUnwrittenVerdictAnswersNothing() {
	enc := s.scene()

	out, err := s.speak(enc, "sermon", "religion", false)
	s.Require().NoError(err)
	s.Nil(out.Answered)
	s.Empty(s.beatsOfKind(enc, goblin, encounter.BeatAnswered))
}

// The Spoke beat is the whole account of the roll: actor, target, option, the
// line, the skill, the numbers — to every witness, whether it landed or not.
func (s *OptionTestSuite) TestTheSpokeBeatCarriesTheLineAndTheRoll() {
	for _, beaten := range []bool{true, false} {
		enc := s.scene()
		_, err := s.speak(enc, "gossip", "persuasion", beaten)
		s.Require().NoError(err)

		beats := s.beatsOfKind(enc, goblin, encounter.BeatSpoke)
		s.Require().Len(beats, 1)
		s.Equal(string(alice), beats[0]["actor"])
		s.Equal(string(goblin), beats[0]["target"])
		s.Equal("", beats[0]["prop"], "a member target names no prop, and says so")
		s.Equal("gossip", beats[0]["option"])
		s.Equal("Busy night?", beats[0]["line"])
		s.Equal("persuasion", beats[0]["skill"])
		s.EqualValues(12, beats[0]["dc"])
		s.EqualValues(15, beats[0]["total"])
		s.Equal(beaten, beats[0]["beaten"], "false beside a miss, never absent")

		s.Empty(s.beatsOfKind(enc, billy, encounter.BeatSpoke), "billy saw nothing to narrate")
	}
}

// The beat comes before its consequences and the answer after: Spoke, then
// Answered.
func (s *OptionTestSuite) TestSpokeComesBeforeAnswered() {
	enc := s.scene()
	_, err := s.speak(enc, "gossip", "persuasion", true)
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: goblin})
	s.Require().NoError(err)
	var order []string
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatSpoke || beat["beat"] == encounter.BeatAnswered {
			order = append(order, beat["beat"].(string))
		}
	}
	s.Equal([]string{encounter.BeatSpoke, encounter.BeatAnswered}, order)
}

// ErrNoOption: an id the target does not carry, and an id it carries whose
// `requires` does not hold for this speaker — the offer and the door agree,
// and nothing is written either way.
func (s *OptionTestSuite) TestAnOptionTheSpeakerWasNotOfferedIsRefused() {
	enc := s.scene()

	_, err := s.speak(enc, "sing", "performance", true)
	s.Require().ErrorIs(err, encounter.ErrNoOption, "the goblin has no such line")

	_, err = s.speak(enc, "ashford", "persuasion", true)
	s.Require().ErrorIs(err, encounter.ErrNoOption, "alice does not know about the feast yet")

	_, err = s.speak(enc, "bribe", "persuasion", true)
	s.Require().ErrorIs(err, encounter.ErrNoOption, "alice is not carrying the purse")

	s.Empty(s.beatsOfKind(enc, alice, encounter.BeatSpoke), "a refusal writes no beat")
}

// A target that cannot see the speaker is refused, as every social verb was.
func (s *OptionTestSuite) TestSpeakingThroughAWallIsRefused() {
	enc := s.scene()

	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: billy, Target: goblin, OptionID: "gossip", Skill: "persuasion",
		Beaten: true, DC: 10, Total: 15, Roller: rollsLowest{},
	})
	s.Require().ErrorIs(err, encounter.ErrUnwitnessed)
}

// The rest of the refusals, in validation order.
func (s *OptionTestSuite) TestRefusals() {
	enc := s.scene()

	_, err := enc.Speak(s.ctx, nil)
	s.ErrorIs(err, encounter.ErrNilInput)

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{Actor: alice, OptionID: "gossip", Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNoMember, "spoken to nobody")

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: alice, Target: goblin, Prop: theAltar, OptionID: "gossip", Roller: rollsLowest{},
	})
	s.ErrorIs(err, encounter.ErrNoMember, "spoken to two things at once")

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{Actor: alice, Target: goblin, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNoOption, "said nothing")

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{Actor: alice, Target: goblin, OptionID: "gossip"})
	s.ErrorIs(err, encounter.ErrNoRoller)

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: "nobody", Target: goblin, OptionID: "gossip", Roller: rollsLowest{},
	})
	s.ErrorIs(err, encounter.ErrNotMember)

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: alice, Target: "nobody", OptionID: "gossip", Roller: rollsLowest{},
	})
	s.ErrorIs(err, encounter.ErrNotMember)

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: goblin, Target: goblin, OptionID: "gossip", Roller: rollsLowest{},
	})
	s.ErrorIs(err, encounter.ErrNotMember, "talking to yourself is a caller defect")
}

// A prop answers too, and differently in exactly two ways: no deed lands
// (nobody was persuaded of anything), and the audience is whoever sees the
// SPEAKER, since a prop sees nothing. The fact still reaches every one of
// them.
func (s *OptionTestSuite) TestSpeakingToAPropLandsNoDeedAndTeachesTheSpeakersWitnesses() {
	enc := s.scene()

	offered, err := enc.PropOptionsFor(alice, theAltar)
	s.Require().NoError(err)
	s.Equal([]string{"examine"}, ids(offered))

	out, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: alice, Prop: theAltar, OptionID: "examine", Skill: "religion",
		Beaten: true, DC: 12, Total: 16, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.Empty(out.Deed, "a prop is persuaded of nothing")
	s.Equal([]encounter.MemberID{alice, goblin}, out.Witnesses, "who sees alice")

	s.False(s.heldDeed(enc, goblin, alice, encounter.DeedPersuade))

	// Who learned it, read off the journal the run persists: a member knows
	// a fact when a `known:fact:<id>` names them as its subject.
	known := func(member encounter.MemberID) bool {
		data := enc.ToData()
		s.Require().NotNil(data.Journal)
		for _, f := range data.Journal.Facts {
			if f.Kind == "known:fact:"+string(cultistFact) && f.Subject == string(member) {
				return true
			}
		}

		return false
	}
	s.True(known(alice), "alice read the sigils")
	s.True(known(goblin), "and the goblin watched her do it")
	s.False(known(billy), "billy saw nothing")

	spoke := s.beatsOfKind(enc, alice, encounter.BeatSpoke)
	s.Require().Len(spoke, 1)
	s.Equal(string(theAltar), spoke[0]["prop"])
	s.Equal("", spoke[0]["target"])

	_, err = enc.PropOptionsFor(alice, "nothing")
	s.ErrorIs(err, encounter.ErrNoProp)
}

// The options cross the save and survive the reload, prop and member alike —
// including the reload's trust boundary, which has to accept the fact an
// option row mints.
func (s *OptionTestSuite) TestOptionsSurviveAReload() {
	enc := s.scene()
	_, err := s.speak(enc, "gossip", "persuasion", true)
	s.Require().NoError(err)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Capabilities: s.capabilities(),
	})
	s.Require().NoError(err)

	offered, err := reloaded.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"gossip", "ashford", "threaten", "sermon"}, ids(offered),
		"the options and the fact alice learned both came back")

	_, err = reloaded.Hold(&encounter.HoldInput{Member: alice, Target: thePurse})
	s.Require().NoError(err)
	offered, err = reloaded.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Contains(ids(offered), "bribe", "and `holds` is still read after a reload")

	altar, err := reloaded.PropOptionsFor(alice, theAltar)
	s.Require().NoError(err)
	s.Equal(altarOptions(), altar)
}

// An option this composition could not run is refused at the door it came in
// through, never at the moment somebody speaks.
func (s *OptionTestSuite) TestAnOptionThatCouldNeverRunIsRefusedAtConstruction() {
	tests := []struct {
		name   string
		change func(*encounter.Option)
	}{
		{"no id", func(o *encounter.Option) { o.ID = "" }},
		{"no line", func(o *encounter.Option) { o.Say = "" }},
		{"no check", func(o *encounter.Option) { o.Check = nil }},
		{"an approach with nothing to beat", func(o *encounter.Option) {
			o.Check = []encounter.CheckApproach{{Ability: "persuasion"}}
		}},
		{"a time word on a verdict", func(o *encounter.Option) {
			o.Success = []encounter.Answer{{Weight: 1, Hold: true}}
		}},
		{"a requirement that is not fact or holds", func(o *encounter.Option) {
			o.Requires = encounter.TriggerRound{Round: 2}
		}},
		{"holding a prop this field does not declare", func(o *encounter.Option) {
			o.Requires = encounter.TriggerHolds{Prop: "lantern"}
		}},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			options := goblinOptions()
			test.change(&options[0])
			_, err := encounter.NewEncounter(&encounter.SetupInput{
				Field: encounter.FieldInput{
					Canvas:  openAir(),
					Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
					Props:   []encounter.PropInput{holdableProp(thePurse, "test:props:purse", spatial.Position{X: 1, Y: 1})},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
					{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Options: options},
				},
				Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
				Capabilities: s.capabilities(),
			})
			s.Require().ErrorIs(err, encounter.ErrBadMember)
		})
	}

	s.Run("two options under one id", func() {
		options := goblinOptions()
		options[1].ID = options[0].ID
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{
				Canvas:  openAir(),
				Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
				Props:   []encounter.PropInput{holdableProp(thePurse, "test:props:purse", spatial.Position{X: 1, Y: 1})},
			},
			Members: []encounter.MemberInput{
				{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Options: options},
			},
			Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
			Capabilities: s.capabilities(),
		})
		s.Require().ErrorIs(err, encounter.ErrBadMember)
	})

	s.Run("a prop that runs", func() {
		altar := encounter.PropInput{
			ID: theAltar, Ref: "test:props:altar", At: spatial.Position{X: 2, Y: 2},
			BlocksMovement: no(), BlocksLineOfSight: no(),
			Options: []encounter.Option{{
				ID: "examine", Say: "Examine it.", Check: []encounter.CheckApproach{{Ability: "religion", DC: 12}},
				Success: []encounter.Answer{{Weight: 1, Flee: true}},
			}},
		}
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{
				Canvas:  openAir(),
				Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
				Props:   []encounter.PropInput{altar},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			},
			Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
			Capabilities: s.capabilities(),
		})
		s.Require().ErrorIs(err, encounter.ErrNoField)
	})
}

// `holds` is never edge-fired: it is not something an ending, an arrival or
// an until may wait for.
func (s *OptionTestSuite) TestHoldsIsNotAnEnding() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
			Props:   []encounter.PropInput{holdableProp(thePurse, "test:props:purse", spatial.Position{X: 1, Y: 1})},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
		},
		Endings:      []encounter.EndingInput{{Key: "rich", Trigger: encounter.TriggerHolds{Prop: thePurse}}},
		Capabilities: s.capabilities(),
	})
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
}

// The witness read a caller prices against is the same set the verb lands
// on, and it answers before anything is spent.
func (s *OptionTestSuite) TestTheWitnessReadIsTheAudience() {
	enc := s.scene()

	witnesses, err := enc.Witnesses(alice)
	s.Require().NoError(err)
	s.Equal([]encounter.MemberID{alice, goblin}, witnesses, "billy is behind the wall")

	out, err := s.speak(enc, "gossip", "persuasion", true)
	s.Require().NoError(err)
	s.Equal(witnesses, out.Witnesses, "what the caller was told, and what the verb used")

	_, err = enc.Witnesses("nobody")
	s.ErrorIs(err, encounter.ErrNotMember)
}

// untrainedCheck is the arithmetic an untrained character's Persuasion or
// Intimidation check produces: two d20 faces, the lower kept, and the record
// naming the rule that decided it and the entity it was imposed on.
func untrainedCheck(total int) *encounter.RollCalculation {
	modifier := total - 7
	return &encounter.RollCalculation{
		Components: []encounter.RollComponent{
			{
				Source: encounter.RollSource{
					Ref: "dnd5e:skills:intimidation", Name: "Intimidation", SourceID: string(alice),
				},
				Dice: &encounter.DiceTrace{
					Notation: "2d20", DieSize: 20,
					OriginalRolls: []int{7, 18}, FinalRolls: []int{7, 18},
					KeptIndices: []int{0}, Subtotal: 7,
					Keep: &encounter.DiceKeep{
						Rule: encounter.KeepDisadvantage,
						Imposed: []encounter.RollSource{{
							Ref: "dnd5e:rules:untrained", Name: "Untrained",
							Label: "rule", SourceID: string(alice),
						}},
					},
				},
			},
			{
				Source:   encounter.RollSource{Ref: "dnd5e:abilities:charisma", Name: "Charisma"},
				Modifier: &modifier,
			},
		},
		Total: total,
	}
}

// TestTheBeatCarriesTheWholeRoll is the seam half of rpg-project#462, on the
// Spoke beat that replaced the two social beats: the beat used to say {dc,
// total, beaten} and the table saw one number. It now carries
// the arithmetic that produced the number, keep record and all, so a client can
// print "2d20 [7, 18] kept 7 · disadvantage: Untrained" without inventing a
// word the server never sent.
func (s *OptionTestSuite) TestTheBeatCarriesTheWholeRoll() {
	enc := s.scene()

	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: alice, Target: goblin, OptionID: "threaten", Skill: "intimidation",
		Beaten: true, DC: 9, Total: 14, Calculation: untrainedCheck(14), Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	beats := s.beatsOfKind(enc, goblin, encounter.BeatSpoke)
	s.Require().Len(beats, 1)

	raw, err := json.Marshal(beats[0]["calculation"])
	s.Require().NoError(err)
	var got encounter.RollCalculation
	s.Require().NoError(json.Unmarshal(raw, &got))

	s.Require().NoError(encounter.ValidateRollCalculation(&got), "it round-trips as valid arithmetic")
	s.Equal(14, got.Total)
	die := got.Components[0].Dice
	s.Require().NotNil(die)
	s.Equal([]int{7, 18}, die.FinalRolls, "both faces survive persistence")
	s.Equal([]int{0}, die.KeptIndices)
	s.Require().NotNil(die.Keep)
	s.Equal(encounter.KeepDisadvantage, die.Keep.Rule)
	s.Require().Len(die.Keep.Imposed, 1)
	s.Equal("Untrained", die.Keep.Imposed[0].Name, "the word comes down from the server")
	s.Equal(string(alice), die.Keep.Imposed[0].SourceID)
}

// TestABeatWithoutArithmeticSaysSo: the field is optional and absent means
// absent. A caller that recorded no arithmetic writes no key, rather than a
// zero-valued calculation a reader would have to tell apart from a real one.
func (s *OptionTestSuite) TestABeatWithoutArithmeticSaysSo() {
	enc := s.scene()

	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Actor: alice, Target: goblin, OptionID: "threaten", Skill: "intimidation",
		Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	beats := s.beatsOfKind(enc, goblin, encounter.BeatSpoke)
	s.Require().Len(beats, 1)
	_, present := beats[0]["calculation"]
	s.False(present)
}

// TestArithmeticThatCannotHaveHappenedIsRefused: a beat is what the table saw.
// Arithmetic that disagrees with the total it is filed under, or a keep record
// that does not describe its own dice, is refused at append rather than
// written down wrong and rendered wrong forever.
func (s *OptionTestSuite) TestArithmeticThatCannotHaveHappenedIsRefused() {
	tests := []struct {
		name   string
		change func(*encounter.RollCalculation)
	}{
		{
			name: "the total disagrees with the beat",
			change: func(calc *encounter.RollCalculation) {
				calc.Total = 99
			},
		},
		{
			name: "disadvantage kept the higher face",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{1}
				calc.Components[0].Dice.Subtotal = 18
				calc.Total = 25
			},
		},
		{
			name: "a rule brought by nobody",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Keep.Imposed[0].SourceID = ""
			},
		},
		{
			name: "a rule nobody has heard of",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Keep.Rule = "lucky"
			},
		},
		{
			name: "the roll did not open with a d20",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.DieSize = 6
				calc.Components[0].Dice.Notation = "2d6"
				calc.Components[0].Dice.OriginalRolls = []int{1, 6}
				calc.Components[0].Dice.FinalRolls = []int{1, 6}
				calc.Components[0].Dice.Subtotal = 1
				calc.Total = 8
			},
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			enc := s.scene()
			calc := untrainedCheck(14)
			test.change(calc)

			_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
				Actor: alice, Target: goblin, OptionID: "threaten", Skill: "intimidation",
				Beaten: true, DC: 9, Total: 14, Calculation: calc, Roller: rollsLowest{},
			})

			s.Require().Error(err)
			s.Contains(err.Error(), "calculation")
			s.Empty(s.beatsOfKind(enc, goblin, encounter.BeatSpoke), "and nothing was written")
		})
	}
}
