// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// BiteSaveGateSuite covers the bite's move from a bare KnockdownDC to a
// declared SaveGate, and the promise that comes with it: a blob written by an
// older build still loads, and still means what it meant.
type BiteSaveGateSuite struct {
	suite.Suite
}

func TestBiteSaveGateSuite(t *testing.T) {
	suite.Run(t, new(BiteSaveGateSuite))
}

// legacyBlob is a bite exactly as an older build persisted it: a DC and no gate.
func (s *BiteSaveGateSuite) legacyBlob(knockdownDC int) monster.ActionData {
	raw, err := json.Marshal(map[string]any{
		"attack_bonus": 4,
		"damage_dice":  "2d4+2",
		"knockdown_dc": knockdownDC,
		"damage_type":  damage.Piercing,
	})
	s.Require().NoError(err)

	return monster.ActionData{Ref: *refs.MonsterActions.Bite(), Config: raw}
}

func (s *BiteSaveGateSuite) loadBite(data monster.ActionData) *BiteAction {
	loaded, err := LoadAction(data)
	s.Require().NoError(err)

	bite, ok := loaded.(*BiteAction)
	s.Require().True(ok)

	return bite
}

// The legacy field maps to the gate it always described, on every axis: STR,
// that DC, negated, no recurrence. Not "roughly equivalent" — KnockdownDC could
// express nothing else.
func (s *BiteSaveGateSuite) TestALegacyKnockdownDCLoadsAsItsGate() {
	bite := s.loadBite(s.legacyBlob(11))

	s.Require().NotNil(bite.SaveGate())
	s.Assert().Equal(saves.NewSaveGate(abilities.STR, 11), bite.SaveGate())
	s.Assert().Equal([]abilities.Ability{abilities.STR}, bite.SaveGate().Abilities)
	s.Assert().Equal(11, bite.SaveGate().DC.DC(saves.DCInput{}))
	s.Assert().Equal(saves.Negated, bite.SaveGate().OnSuccess)
	s.Assert().Equal(saves.RecurrenceNone, bite.SaveGate().Recurrence)
}

// A DC of zero means no knockdown. The other reading — a save whose DC nobody
// can fail to beat, or worse, one that auto-fails — would invent a rule out of
// an empty field (rpg-toolkit#962's done-when calls this out by name).
func (s *BiteSaveGateSuite) TestAZeroKnockdownDCIsNoGateAtAll() {
	s.Assert().Nil(s.loadBite(s.legacyBlob(0)).SaveGate())

	// And the same for a blob that never mentioned knockdown.
	raw, err := json.Marshal(map[string]any{"attack_bonus": 4, "damage_dice": "1d6"})
	s.Require().NoError(err)
	s.Assert().Nil(s.loadBite(monster.ActionData{Ref: *refs.MonsterActions.Bite(), Config: raw}).SaveGate())

	// Negative is the same answer, for the same reason.
	s.Assert().Nil(s.loadBite(s.legacyBlob(-3)).SaveGate())
}

// The migration, pinned from the writing end: a legacy blob comes back
// expressed as a gate, and the deprecated field is not written beside it. Two
// places to say the same thing is two places that can disagree.
func (s *BiteSaveGateSuite) TestALegacyBlobIsRewrittenAsAGate() {
	out := s.loadBite(s.legacyBlob(11)).ToData()

	s.Assert().Contains(string(out.Config), `"save_gate"`)
	s.Assert().NotContains(string(out.Config), `"knockdown_dc"`)

	// Nothing the old field said is missing from the new one.
	reloaded := s.loadBite(out)
	s.Assert().Equal(saves.NewSaveGate(abilities.STR, 11), reloaded.SaveGate())
}

// A gate-carrying bite round-trips byte-identically, which is the property the
// legacy blob cannot have and this one must.
func (s *BiteSaveGateSuite) TestAGateCarryingBiteRoundTripsByteIdentical() {
	bite := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		DamageType:  damage.Piercing,
		SaveGate:    saves.NewSaveGate(abilities.STR, 11),
	})

	first := bite.ToData()
	second := s.loadBite(first).ToData()

	s.Assert().Equal(string(first.Config), string(second.Config))
	s.Assert().Equal(first.Ref, second.Ref)
}

// A bite carrying a gate that is not a knockdown survives too — the point of
// declaring the ability rather than assuming it.
func (s *BiteSaveGateSuite) TestANonKnockdownGateSurvives() {
	gate := &saves.SaveGate{
		Abilities:  []abilities.Ability{abilities.CON},
		DC:         saves.DCFivePlusDamageTaken(),
		OnSuccess:  saves.Half,
		Recurrence: saves.RecurrenceEndOfTurn,
	}

	reloaded := s.loadBite(NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		DamageType:  damage.Piercing,
		SaveGate:    gate,
	}).ToData())

	s.Assert().Equal(gate, reloaded.SaveGate())
	s.Assert().Equal(12, reloaded.SaveGate().DC.DC(saves.DCInput{DamageTaken: 7}))
}

// A config carrying both keeps the explicit one: somebody wrote the gate on
// purpose, where the DC may just be a field nobody cleaned up.
func (s *BiteSaveGateSuite) TestAnExplicitGateBeatsTheDeprecatedField() {
	bite := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		SaveGate:    saves.NewSaveGate(abilities.DEX, 15),
	})

	s.Assert().Equal(saves.NewSaveGate(abilities.DEX, 15), bite.SaveGate())
}

// The scoring bonus that was always described as "knockdown potential" now
// depends on there being a knockdown. Both existing Score tests build a bite
// with a DC, so both still see it.
func (s *BiteSaveGateSuite) TestScoringReadsTheGate() {
	adjacent := &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Adjacent: true}}}

	gated := NewBiteAction(BiteConfig{DamageDice: "2d4+2", SaveGate: saves.NewSaveGate(abilities.STR, 11)})
	plain := NewBiteAction(BiteConfig{DamageDice: "2d4+2"})

	s.Assert().Equal(80, gated.Score(nil, adjacent))
	s.Assert().Equal(70, plain.Score(nil, adjacent), "a bite that knocks nobody down does not score for it")
}
