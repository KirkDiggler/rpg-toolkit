// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// hold gives the monster a recipient half holding one cannot-move part.
func (s *monsterHeldRaiseSuite) hold(application, name string) {
	config := json.RawMessage(`{"application_id":"` + application + `","address_source_id":"` + application + `","role":"recipient",` +
		`"bindings":{"name":"` + name + `","owner":{"member_id":"guard","condition_ref":"` + refs.Conditions.LinkedEffects().String() + `","source_id":"` + application + `"},` +
		`"member_key":"ogre-1","parts":[{"key":"hold","kind":"cannot","on_terminal":"part","params":{"what":["move"]}}]}}`)
	made, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: "ogre-1",
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Conditions.LinkedEffects(), EntityID: "guard"}, Config: config})
	s.Require().NoError(err)
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.ConditionAppliedEvent{Target: s.m, Condition: made.Condition}))
}

func (s *monsterHeldRaiseSuite) cannotMove() *conditions.CannotMoveOutput {
	out, err := s.m.CannotMove()
	s.Require().NoError(err)
	return out
}

// Catches a monster's CannotMove that ignores its held conditions, keeps
// answering after the hold ends, or loses it on a save and load.
func (s *monsterHeldRaiseSuite) TestAHoldTakesMovingFromTheMonster() {
	s.False(s.cannotMove().CannotMove, "a monster holding nothing may move")
	s.hold("application:hold", "Held")
	s.raise("application:feast", "2")
	held := s.cannotMove()
	s.True(held.CannotMove)
	s.Equal([]string{"Held"}, held.Sources, "a raise is not a hold")

	s.reload()
	s.Equal([]string{"Held"}, s.cannotMove().Sources, "the hold is read again from the held condition")

	s.end("application:hold")
	s.False(s.cannotMove().CannotMove)
	s.Empty(s.cannotMove().Sources)
}
