// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// handaxe is the dagger's swing made with a handaxe: a melee weapon that is
// neither finesse nor of a ranged category, thrown or not.
func handaxe() combatActions.Definition {
	definition := dagger()
	weapon := *refs.Weapons.Handaxe()
	definition.Ref = weapon
	definition.Name = "Handaxe"
	definition.Attack.Weapon = &combatActions.WeaponContext{Ref: &weapon}
	definition.Attack.Damage = []damage.Damage{{
		Dice: "1d6", Type: damage.Slashing, Properties: []damage.Property{damage.AddsAttackAbilityModifier},
	}}
	return definition
}

// watchAttackFrames records the frames the strike hands the attack chain.
func (s *FrameTestSuite) watchAttackFrames(bus events.EventBus) *[]contributions.Frame {
	var frames []contributions.Frame
	_, err := dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, e dnd5eEvents.AttackChainEvent,
			c chain.Chain[dnd5eEvents.AttackChainEvent],
		) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
			frames = append(frames, e.Frame.Clone())
			return c, nil
		})
	s.Require().NoError(err)
	return &frames
}

func (s *FrameTestSuite) TestAttackActionFactsReadTheWeapon() {
	facts := attackActionFacts(dagger().Attack, false)

	s.Equal(contributions.Known(refs.Weapons.Dagger().String()), facts.Weapon)
	s.Equal(contributions.Known(""), facts.WeaponSlot, "the fixture names no hand")
	s.Equal(contributions.Known(true), facts.Finesse)
	s.Equal(contributions.Known(false), facts.RangedWeapon)
	s.Equal(contributions.Known(false), facts.TwoHanded)
	s.Equal(contributions.Known(false), facts.OffHandWeapon)
	s.Equal(contributions.Known(false), facts.OffHandAttack)
	s.Equal(contributions.Known(3), facts.AbilityModifier)
	s.Equal(contributions.Known(false), facts.Opportunity)

	s.Equal(contributions.Known(true), attackActionFacts(dagger().Attack, true).Opportunity,
		"opportunity is the caller's, not the profile's")

	thrown := attackActionFacts(handaxe().Attack, false)
	s.Equal(contributions.Known(false), thrown.Finesse)
	s.Equal(contributions.Known(false), thrown.RangedWeapon, "a handaxe is a melee weapon, thrown or not")
}

func (s *FrameTestSuite) TestAttackActionFactsReadTheGrip() {
	other := *refs.Weapons.Dagger()
	held := dagger()
	held.Attack.IsOffHandAttack = true
	held.Attack.Weapon.Slot = "off_hand"
	held.Attack.Weapon.TwoHanded = true
	held.Attack.Weapon.OffHandWeaponRef = &other

	facts := attackActionFacts(held.Attack, false)

	s.Equal(contributions.Known("off_hand"), facts.WeaponSlot)
	s.Equal(contributions.Known(true), facts.TwoHanded)
	s.Equal(contributions.Known(true), facts.OffHandWeapon)
	s.Equal(contributions.Known(true), facts.OffHandAttack)
}

func (s *FrameTestSuite) TestAttackActionFactsRangedWeapon() {
	bow := *refs.Weapons.Shortbow()
	profile := dagger().Attack
	profile.Delivery = combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}}
	profile.Weapon = &combatActions.WeaponContext{Ref: &bow}

	facts := attackActionFacts(profile, false)

	s.Equal(contributions.Known(true), facts.RangedWeapon)
	s.Equal(contributions.Known(false), facts.Finesse)
}

// TestAttackActionFactsNoWeaponIsKnownNone: an attack with no weapon context
// knows every weapon fact as its zero value.
func (s *FrameTestSuite) TestAttackActionFactsNoWeaponIsKnownNone() {
	facts := attackActionFacts(bite().Attack, false)

	s.Equal(contributions.Known(""), facts.Weapon)
	s.Equal(contributions.Known(false), facts.Finesse)
	s.Equal(contributions.Known(false), facts.RangedWeapon)
	s.Equal(contributions.Known(false), facts.TwoHanded)
	s.Equal(contributions.Known(0), facts.AbilityModifier, "a stat block names no ability")
}

// TestAttackActionFactsUnreadableWeaponIsUnknown: a weapon context whose ref
// is missing, or names a weapon the catalogue does not hold, is never read as
// a plain weapon.
func (s *FrameTestSuite) TestAttackActionFactsUnreadableWeaponIsUnknown() {
	profile := dagger().Attack
	profile.Weapon = &combatActions.WeaponContext{}
	facts := attackActionFacts(profile, false)
	s.Equal(contributions.Unknown[string](), facts.Weapon)
	s.Equal(contributions.Unknown[bool](), facts.Finesse)
	s.Equal(contributions.Unknown[bool](), facts.RangedWeapon)

	homebrew := core.Ref{Module: "homebrew", Type: "weapons", ID: "moon-blade"}
	profile.Weapon = &combatActions.WeaponContext{Ref: &homebrew}
	facts = attackActionFacts(profile, false)
	s.Equal(contributions.Known(homebrew.String()), facts.Weapon)
	s.Equal(contributions.Unknown[bool](), facts.Finesse)
	s.Equal(contributions.Unknown[bool](), facts.RangedWeapon)
}

// TestAttackRollFrameDiffersFromThePostFoldFrameOnlyInAdvantage: the attack
// chain is handed the attack-roll frame with advantage unknown, and the
// damage fold the same frame with advantage settled — nothing else differs.
func (s *FrameTestSuite) TestAttackRollFrameDiffersFromThePostFoldFrameOnlyInAdvantage() {
	for name, granted := range map[string]bool{"no advantage": false, "granted advantage": true} {
		s.Run(name, func() {
			bus := events.NewEventBus()
			if granted {
				s.grantAdvantage(bus)
			}
			rolled := s.watchAttackFrames(bus)
			spy := s.watchFrames(bus)
			d20 := interactionRoll{count: 1, sides: 20, faces: []int{15}}
			if granted {
				d20 = interactionRoll{count: 2, sides: 20, faces: []int{15, 9}}
			}
			roller := &interactionRoller{script: []interactionRoll{
				d20,
				{count: 1, sides: 4, faces: []int{3}},
				{count: 1, sides: 6, faces: []int{5}},
			}}

			_, err := s.resolveCamp(bus, &StrikeInput{AttackerID: holdOutRogue, TargetID: holdOutScout, Definition: dagger(), Roller: roller})
			s.Require().NoError(err)

			s.Require().Len(*rolled, 1)
			s.Require().Len(spy.damaged, 1)
			before, after := (*rolled)[0], spy.damaged[0]
			s.Require().NoError(before.Validate())
			s.Equal(contributions.Unknown[bool](), before.Action.Advantage, "the fold has not run")
			s.Equal(contributions.Known(granted), after.Action.Advantage)

			before.Action.Advantage = after.Action.Advantage
			s.Equal(before, after, "the post-fold frame only adds advantage")
		})
	}
}

// TestOpportunityStrikeCarriesTheFact: an opportunity attack's frame says so,
// from the strike input, on the attack chain and after the fold.
func (s *FrameTestSuite) TestOpportunityStrikeCarriesTheFact() {
	bus := events.NewEventBus()
	rolled := s.watchAttackFrames(bus)
	spy := s.watchFrames(bus)
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 1, sides: 4, faces: []int{3}},
		{count: 1, sides: 6, faces: []int{5}},
	}}

	_, err := s.resolveCamp(bus, &StrikeInput{
		AttackerID: holdOutRogue, TargetID: holdOutScout, Definition: dagger(), Opportunity: true, Roller: roller,
	})
	s.Require().NoError(err)

	s.Require().Len(*rolled, 1)
	s.Equal(contributions.Known(true), (*rolled)[0].Action.Opportunity)
	s.Require().Len(spy.damaged, 1)
	s.Equal(contributions.Known(true), spy.damaged[0].Action.Opportunity)
}

// TestResumedOpportunityStrikeKeepsTheFact: a frozen opportunity attack
// resumes as one, so its rebuilt frame still says so.
func (s *FrameTestSuite) TestResumedOpportunityStrikeKeepsTheFact() {
	posed, err := heroSwings(s.T(), inspiredHero(s.T()), &actionRoller{singles: []int{8}})
	s.Require().NoError(err)
	s.Require().NotNil(posed.Posed)
	var blob map[string]any
	s.Require().NoError(json.Unmarshal(posed.Posed.Frozen, &blob))
	s.Nil(blob["opportunity"], "a swing on its own turn freezes no opportunity flag")
	folded, ok := blob["folded"].(map[string]any)
	s.Require().True(ok)
	s.NotContains(folded, "Frame", "the frame is rebuilt from truth, never frozen")
	blob["opportunity"] = true
	frozen, err := json.Marshal(blob)
	s.Require().NoError(err)

	machine, err := NewStrikeResumed(&StrikeResumeInput{
		Frozen: frozen, Answer: OfferSpend,
		Roller: &actionRoller{singles: []int{4}, damage: [][]int{{3}}},
	})
	s.Require().NoError(err)
	bus := events.NewEventBus()
	spy := s.watchFrames(bus)

	_, err = resolveHeroStrikeOn(s.T(), inspiredHero(s.T()), machine, newSurface(bus))
	s.Require().NoError(err)

	s.Require().Len(spy.damaged, 1)
	s.Equal(contributions.Known(true), spy.damaged[0].Action.Opportunity)
}

// TestHandaxeStrikeDoesNotSneakAttack is the execution half of rpg-toolkit#1929
// at the interaction: the same scene that sneak attacks with the dagger does
// not with a handaxe, whichever ability swings it.
func (s *FrameTestSuite) TestHandaxeStrikeDoesNotSneakAttack() {
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 1, sides: 6, faces: []int{3}},
	}}

	out, err := s.resolveCamp(events.NewEventBus(),
		&StrikeInput{AttackerID: holdOutRogue, TargetID: holdOutScout, Definition: handaxe(), Roller: roller})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit)
	s.False(sneakAttackFired(outcome), "components: %+v", outcome.DamageComponents)
	s.Empty(roller.script)
	s.Equal(abilities.DEX, handaxe().Attack.Ability.Ability, "precondition: the swing uses Dexterity")
}
