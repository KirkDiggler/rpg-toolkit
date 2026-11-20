// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// Catches levelling that adds its gain to the effective maximum and stores
// that (the raise would then be counted twice once it is read back), or that
// drops the raise from the maximum after the level.
func (s *LevelUpSuite) TestLevellingWhileHoldingARaiseRaisesTheBase() {
	char := s.bard()
	base := char.maxHitPoints
	config := json.RawMessage(`{"application_id":"application:feast","address_source_id":"cook","role":"recipient",` +
		`"bindings":{"name":"Feasted","owner":{"member_id":"cook","condition_ref":"` + refs.Conditions.LinkedEffects().String() + `","source_id":"application:feast"},` +
		`"member_key":"` + char.GetID() + `","parts":[{"key":"feast","kind":"hit_point_maximum","on_terminal":"part","amount":{"fixed":2}}]}}`)
	made, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: char.GetID(),
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Conditions.LinkedEffects(), EntityID: "cook"}, Config: config})
	s.Require().NoError(err)
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.ConditionAppliedEvent{Target: char, Condition: made.Condition}))
	s.Require().Equal(base+2, char.GetMaxHitPoints())

	_, err = char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.HealingWord)},
	})
	s.Require().NoError(err)
	gain := char.levels[len(char.levels)-1].HitPointGain
	s.Require().Positive(gain)

	s.Equal(base+gain, char.maxHitPoints, "the level raised the base by its gain alone")
	s.Equal(base+gain+2, char.GetMaxHitPoints(), "and the raise still sits on top")
	data, err := char.ToData()
	s.Require().NoError(err)
	s.Equal(base+gain, data.MaxHitPoints, "the base is what is saved")
}
