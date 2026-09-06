// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

// holdout_test.go is A8 of the hold-out (rpg-project#375, design §9): the side
// questions the rules ask are answered by the run. A raider is an enemy of the
// party for Sneak Attack until the camp turns, and not after; two raiders are
// each other's allies for Pack Tactics before and after. Played through
// Resolve on a world the encounter built and turned, so what the rules read
// is what a walk will meet — and Sneak Attack and Pack Tactics are not
// touched, which is the design's own sentence (§4).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	holdOutRogue  = "rogue"
	holdOutAlly   = "ally"
	holdOutChief  = "raider-chief"
	holdOutScout  = "raider-scout"
	holdOutCamp   = "raiders"
	holdOutFact   = "saved-wiseman"
	holdOutLetter = "letter"
	holdOutRecord = "wisemans-letter"
)

type HoldOutSuite struct {
	suite.Suite
}

func TestHoldOutSuite(t *testing.T) {
	suite.Run(t, new(HoldOutSuite))
}

// camp is the smallest hold-out: one yard, the raiders hostile to the party
// until their chief knows the fact the letter reveals, and the letter on the
// ground beside the party. The chief is the camp's mind.
//
// One row, west to east: the letter, the ally, the scout, the rogue, the
// chief. So the scout stands between the two players — adjacent to both,
// which is Sneak Attack's "another enemy of the target within 5 feet" when
// the rogue swings at it — and the chief stands beside the rogue with the
// scout on the rogue's other side, which is Pack Tactics' "an ally of the
// attacker within 5 feet of the target" when the chief bites. The ally sorts
// first, so the ally acts first, so the ally can pick up the letter.
func (s *HoldOutSuite) camp() *encounter.Encounter {
	no := false
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 8, 4)},
			Intel:   []encounter.IntelRecord{{ID: holdOutRecord, Reveals: encounter.RevealTargets{Fact: holdOutFact}}},
			Props: []encounter.PropInput{{
				ID: holdOutLetter, Ref: "dnd5e:props:scroll", At: spatial.Position{X: 0, Y: 1},
				Holdable: true, BlocksMovement: &no, BlocksLineOfSight: &no,
				Holds: []encounter.IntelID{holdOutRecord},
			}},
			Factions: []encounter.FactionInput{{ID: holdOutCamp, Mind: holdOutChief}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{holdOutCamp, encounter.FactionParty},
				Stance:  encounter.StanceHostile, Until: encounter.TriggerFact{Fact: holdOutFact},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: holdOutAlly, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: holdOutScout, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}, Faction: holdOutCamp},
			{ID: holdOutRogue, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
			{ID: holdOutChief, Kind: encounter.KindMonster, Position: spatial.Position{X: 4, Y: 1}, Faction: holdOutCamp},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// turn has the ally pick up the letter, standing in the chief's yard: the
// presence that teaches the mind (design §3.6), and the camp turns.
func (s *HoldOutSuite) turn(enc *encounter.Encounter) {
	_, err := enc.Hold(&encounter.HoldInput{Member: holdOutAlly, Target: holdOutLetter})
	s.Require().NoError(err)
	stance, err := enc.Stance(holdOutCamp, encounter.FactionParty)
	s.Require().NoError(err)
	s.Require().Equal(encounter.StanceNeutral, stance, "precondition: the camp turned")
}

// rogue is a level-1 rogue carrying Sneak Attack, as the sheet persists it.
func (s *HoldOutSuite) rogue() *character.Data {
	sneak, err := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{MemberID: holdOutRogue, Level: 1}).ToJSON()
	s.Require().NoError(err)

	return &character.Data{
		ID: holdOutRogue, PlayerID: "player-rogue", Name: "Rook", Level: 1, ClassID: classes.Rogue, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 16, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
		},
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 14, ProficiencyBonus: 2,
		Conditions: []json.RawMessage{sneak},
	}
}

// ally is the other player: a fighter with nothing on the sheet that reads
// the cast, whose only job is to stand where the scene puts them.
func (s *HoldOutSuite) ally() *character.Data {
	return &character.Data{
		ID: holdOutAlly, PlayerID: "player-ally", Name: "Ash", Level: 1, ClassID: classes.Fighter, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12, ArmorClass: 16, ProficiencyBonus: 2,
	}
}

// raider is a wolf carrying Pack Tactics, as the sheet persists it.
func (s *HoldOutSuite) raider(id string) *monster.Data {
	data := monsters.NewWolf(id).ToData()
	pack, err := monstertraits.PackTactics(id).ToJSON()
	s.Require().NoError(err)
	data.Conditions = append(data.Conditions, pack)

	return data
}

// dagger is a finesse swing: Dexterity is the ability, which is the one
// property Sneak Attack checks before it asks about the target's enemies.
func dagger() combatActions.Definition {
	weapon := *refs.Weapons.Dagger()

	return combatActions.Definition{
		Ref:  weapon,
		Name: "Dagger",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 5,
			Ability:     &combatActions.AbilityContribution{Ability: abilities.DEX, Modifier: 3},
			Weapon:      &combatActions.WeaponContext{Ref: &weapon},
			Damage: []damage.Damage{{
				Dice:       "1d4",
				Type:       damage.Piercing,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			}},
		},
	}
}

// bite is a raider's plain melee bite: no rider, so the only dice in the scene
// are the attack and its damage.
func bite() combatActions.Definition {
	return combatActions.Definition{
		Ref:  *refs.MonsterActions.WolfBite(),
		Name: "bite",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}},
		},
	}
}

// resolve runs one strike on the given world with the whole camp in the cast.
func (s *HoldOutSuite) resolve(world encounter.EncounterData, strike *StrikeInput) StrikeOutcome {
	out, err := Resolve(context.Background(), &Input{
		World: world,
		Participants: []Participant{
			{Character: s.rogue()}, {Character: s.ally()},
			{Monster: s.raider(holdOutScout)}, {Monster: s.raider(holdOutChief)},
		},
		Machine:    NewStrike(strike),
		Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{}, Roller: strike.Roller,
	})
	s.Require().NoError(err)
	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit, "precondition: the scripted swing lands")

	return outcome
}

// sneakAttackFired reports whether the outcome carries Sneak Attack's damage.
func sneakAttackFired(outcome StrikeOutcome) bool {
	for _, component := range outcome.DamageComponents {
		if component.Roll.Source.Ref != nil && component.Roll.Source.Ref.String() == refs.Features.SneakAttack().String() {
			return true
		}
	}

	return false
}

// packTacticsFired reports whether the attack folded with Pack Tactics'
// advantage.
func packTacticsFired(outcome StrikeOutcome) bool {
	for _, source := range outcome.Folded.AdvantageSources {
		if source.SourceRef != nil && source.SourceRef.String() == refs.MonsterTraits.PackTactics().String() {
			return true
		}
	}

	return false
}

// TestSneakAttackTreatsARaiderAsAnEnemyUntilTheCampTurns is A8's Sneak Attack
// half. The rogue stabs the scout with the ally beside it. While the camp is
// hostile the ally is another enemy of the scout and the sneak dice roll;
// once the chief has read the letter the ally is nobody's enemy and the same
// swing, on the same cells, rolls no sneak dice — the rule did not change,
// the run's answer did.
func (s *HoldOutSuite) TestSneakAttackTreatsARaiderAsAnEnemyUntilTheCampTurns() {
	stab := func(roller *interactionRoller) *StrikeInput {
		return &StrikeInput{AttackerID: holdOutRogue, TargetID: holdOutScout, Definition: dagger(), Roller: roller}
	}

	s.Run("before the flip the sneak dice roll", func() {
		roller := &interactionRoller{script: []interactionRoll{
			{count: 1, sides: 20, faces: []int{15}},
			{count: 1, sides: 4, faces: []int{3}},
			{count: 1, sides: 6, faces: []int{5}},
		}}

		outcome := s.resolve(s.camp().ToData(), stab(roller))

		s.True(sneakAttackFired(outcome), "components: %+v", outcome.DamageComponents)
		s.Equal(11, outcome.Damage, "1d4=3, Dexterity 3, Sneak Attack 1d6=5")
		s.Empty(roller.script, "every scripted die was thrown: %v", roller.calls)
	})

	s.Run("after the flip the same swing rolls none", func() {
		enc := s.camp()
		s.turn(enc)
		roller := &interactionRoller{script: []interactionRoll{
			{count: 1, sides: 20, faces: []int{15}},
			{count: 1, sides: 4, faces: []int{3}},
		}}

		outcome := s.resolve(enc.ToData(), stab(roller))

		s.False(sneakAttackFired(outcome), "a raider is no longer an enemy of the party: %+v", outcome.DamageComponents)
		s.Equal(6, outcome.Damage, "1d4=3, Dexterity 3, and nothing else")
		s.Empty(roller.script, "%v", roller.calls)
	})
}

// TestPackTacticsCountsARaiderAsAnAllyBeforeAndAfterTheFlip is A8's Pack
// Tactics half. The chief bites the rogue with the scout beside it. The
// raiders are allied with themselves whatever they are to the party, so the
// bite has advantage before the camp turns and after: neutral toward the
// party is not the end of the pack.
func (s *HoldOutSuite) TestPackTacticsCountsARaiderAsAnAllyBeforeAndAfterTheFlip() {
	script := func() *interactionRoller {
		return &interactionRoller{script: []interactionRoll{
			{count: 2, sides: 20, faces: []int{15, 3}},
			{count: 2, sides: 4, faces: []int{2, 2}},
		}}
	}
	chomp := func(roller *interactionRoller) *StrikeInput {
		return &StrikeInput{AttackerID: holdOutChief, TargetID: holdOutRogue, Definition: bite(), Roller: roller}
	}

	s.Run("before the flip the pack has advantage", func() {
		roller := script()
		outcome := s.resolve(s.camp().ToData(), chomp(roller))
		s.True(packTacticsFired(outcome), "advantage: %+v", outcome.Folded.AdvantageSources)
		s.Equal(15, outcome.Roll, "the higher die")
		s.Empty(roller.script, "%v", roller.calls)
	})

	s.Run("after the flip the pack still has advantage", func() {
		enc := s.camp()
		s.turn(enc)
		roller := script()
		outcome := s.resolve(enc.ToData(), chomp(roller))
		s.True(packTacticsFired(outcome), "two raiders are still each other's allies: %+v", outcome.Folded.AdvantageSources)
		s.Equal(15, outcome.Roll)
		s.Empty(roller.script, "%v", roller.calls)
	})
}
