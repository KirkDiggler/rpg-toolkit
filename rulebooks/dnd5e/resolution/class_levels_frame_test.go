// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// class_levels_frame_test.go holds slice 3 of rpg-project#538: every frame
// resolution builds carries the actor's class levels from the actor's own
// sheet, and nothing below the sheet holds a copy for a rule to read.

// rogueAtThree is the hold-out camp's rogue with a three-level record. The
// Sneak Attack blob it carries holds no level at all — the dice come from the
// record, through the frame.
func (s *FrameTestSuite) rogueAtThree() *character.Data {
	data := s.holdOut().rogue()
	data.Level = 3
	data.Levels = syntheticLevels(classes.Rogue, 3, 9, 6)
	data.MaxHitPoints = 21
	data.HitPoints = 21
	return data
}

// resolveCampWith runs one strike on the hold-out camp with the given rogue
// sheet in place of the camp's own.
func (s *FrameTestSuite) resolveCampWith(
	bus events.EventBus, rogue *character.Data, strike *StrikeInput,
) (*Output, error) {
	camp := s.holdOut()
	return resolveOn(s.ctx, &Input{
		World: camp.camp().ToData(),
		Participants: []Participant{
			{Character: rogue}, {Character: camp.ally()},
			{Monster: camp.raider(holdOutScout)}, {Monster: camp.raider(holdOutChief)},
		},
		Machine:    NewStrike(strike),
		Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		TurnDriver: passDriver{}, Roller: strike.Roller,
	}, newSurface(bus))
}

// THE DONE-WHEN. A level-3 rogue's dagger at the scout, with the fighter beside
// it: the information row reads +2d6 and the swing rolls exactly 2d6, from one
// rule function reading one level. Both frames got the level from the same
// sheet — the information frame from the actor's loaded sheet, the execution
// frame from the cast's.
func (s *FrameTestSuite) TestInformAndExecutionAgreeOnSneakAttackAtRogueThree() {
	sheet, err := character.Load(s.ctx, s.rogueAtThree())
	s.Require().NoError(err)
	observed, err := s.holdOut().camp().ObservedContext(&encounter.ViewInput{Member: holdOutRogue})
	s.Require().NoError(err)

	informed, err := InformAttack(&InformAttackInput{
		Observed: observed, Actor: sheet, Attack: dagger().Attack, Targets: []string{holdOutScout},
	})
	s.Require().NoError(err)
	var row *contributions.Effect
	for i, effect := range informed.ByTarget[holdOutScout] {
		if effect.ID == refs.Features.SneakAttack().String() {
			row = &informed.ByTarget[holdOutScout][i]
		}
	}
	s.Require().NotNil(row, "the rogue's Sneak Attack bears on the dagger: %+v", informed.ByTarget[holdOutScout])
	s.Equal(contributions.StateApplies, row.State, "reason: %s", row.Reason)
	s.Equal("+2d6 damage", row.Benefit, "one d6 per two rogue levels, rounded up")

	bus := events.NewEventBus()
	spy := s.watchFrames(bus)
	// The roller is strict about the shape of every call: a Sneak Attack that
	// asked for 1d6 (a copied level one) or 3d6 would fail the script.
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 1, sides: 4, faces: []int{3}},
		{count: 2, sides: 6, faces: []int{4, 5}},
	}}
	out, err := s.resolveCampWith(bus, s.rogueAtThree(),
		&StrikeInput{AttackerID: holdOutRogue, TargetID: holdOutScout, Definition: dagger(), Roller: roller})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.True(sneakAttackFired(outcome), "components: %+v", outcome.DamageComponents)
	s.Equal(15, outcome.Damage, "1d4=3, Dexterity 3, Sneak Attack 2d6=4+5")

	s.Require().NotEmpty(spy.damaged)
	executed, known := spy.damaged[0].ActorClassLevels.Get()
	s.Require().True(known, "the execution frame knows the attacker's class levels")
	sheetLevels, _ := sheet.ClassLevels().Get()
	s.Equal(sheetLevels, executed, "the execution frame carries what the sheet answers")
	rogue, _ := spy.damaged[0].ActorClassLevels.Of(classes.Rogue)
	s.Equal(3, rogue)
}

// A monster actor's frame carries class levels that are KNOWN and empty — a
// stat block holds no class levels — never unknown, which would make any
// class-scaled rule it ever held unable to answer, and never a level one.
func (s *FrameTestSuite) TestAMonsterActorsFrameCarriesKnownEmptyClassLevels() {
	bus := events.NewEventBus()
	spy := s.watchFrames(bus)
	bite := s.holdOut().raider(holdOutScout).Actions[0]

	_, err := s.resolveCampWith(bus, s.holdOut().rogue(), &StrikeInput{
		AttackerID: holdOutScout, TargetID: holdOutRogue, Definition: bite,
		Roller: facedRoller{d20: 19, other: 2},
	})
	s.Require().NoError(err)

	s.Require().NotEmpty(spy.damaged, "the bite hit and folded its damage")
	frame := spy.damaged[0]
	s.Equal(holdOutScout, frame.Actor)
	levels, known := frame.ActorClassLevels.Get()
	s.True(known, "a monster's class levels are known")
	s.Empty(levels, "and it holds none")
}

// The information frame is built from what a member knows, and a member always
// knows its own sheet: unknown actor class levels are refused, never framed.
func (s *FrameTestSuite) TestInformationFrameRefusesUnknownActorClassLevels() {
	_, err := informationFrame(&informationFrameInput{
		Observed: &encounter.ObservedContextOutput{Observer: encounter.MemberID(holdOutRogue)},
		Attack:   dagger().Attack,
	})
	s.Require().ErrorIs(err, contributions.ErrRuleCannotAnswer)
}

// A contest's damage frame names the instigator, and carries the instigator's
// class levels from its sheet in the cast. An instigator with no sheet here
// has nothing attached to this bus, and its levels are framed UNKNOWN — the
// honest answer — never as an empty known list nobody read.
func (s *FrameTestSuite) TestContestDamageFrameCarriesTheInstigatorsSheet() {
	rogue, err := character.Load(s.ctx, s.rogueAtThree())
	s.Require().NoError(err)
	wolf, err := monster.LoadFromData(s.ctx, s.holdOut().raider(holdOutScout), events.NewEventBus())
	s.Require().NoError(err)
	cast := &Participants{
		characters: map[string]*character.Character{holdOutRogue: rogue},
		monsters:   map[string]*monster.Monster{holdOutScout: wolf},
	}

	framed := contestDamageFrame(cast, holdOutRogue, holdOutScout)
	s.Require().NoError(framed.Validate())
	levels, known := framed.ActorClassLevels.Of(classes.Rogue)
	s.True(known)
	s.Equal(3, levels)

	byMonster, known := contestDamageFrame(cast, holdOutScout, holdOutRogue).ActorClassLevels.Get()
	s.True(known, "a monster instigator is known to hold no class levels")
	s.Empty(byMonster)

	_, known = contestDamageFrame(cast, "nobody-here", holdOutRogue).ActorClassLevels.Get()
	s.False(known, "an instigator with no sheet in the cast is unknown")
}
