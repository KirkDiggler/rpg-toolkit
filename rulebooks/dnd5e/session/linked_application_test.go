// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func (s *CastSuite) linkedAttachment(member string, role conditions.LinkedRole) *conditions.LinkedCondition {
	for _, raw := range s.characters.byID[member].Conditions {
		condition, err := conditions.LoadJSON(raw)
		s.Require().NoError(err)
		if linked, ok := condition.(*conditions.LinkedCondition); ok && linked.LinkedApplicationInfo().Role == role {
			return linked
		}
	}
	s.Require().Fail("missing linked attachment", "%s on %s", role, member)
	return nil
}

// This uses Manager.Cast/React and a fresh manager over JSON-round-tripped stores,
// not an injected active Bless/Bane. Resistance is only the existing offer fixture.
func (s *CastSuite) TestLinkedApplicationPersistsEarlierDeliveryAcrossWindowReload() {
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane), []*character.Data{armedFighter("fighter-a"), armedFighter("fighter-b")}, 4, 3, 10)
	s.holdResistance("fighter-b", "cleric-1")
	ctx := context.Background()
	out, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID, Targets: []string{"fighter-a", "fighter-b"}})
	s.Require().NoError(err)
	s.Require().True(out.Posed)
	owner := s.linkedAttachment("bard", conditions.LinkedCoordinator)
	before := owner.LinkedApplicationInfo()
	s.NotEmpty(before.ID)
	s.NotEqual("bard", before.ID)
	s.Len(before.LiveMembers, 1)
	s.Len(before.PendingMembers, 1)
	child := s.linkedAttachment("fighter-a", conditions.LinkedRecipient)
	s.Equal(before.ID, child.LinkedApplicationInfo().ID)
	s.Equal("bard", child.ConditionOrigin().EntityID)
	info, err := owner.MaintenanceInfo()
	s.Require().NoError(err)
	s.Require().NotNil(info.Concentration)
	s.Equal(10, info.Duration.Remaining)
	childInfo, err := child.MaintenanceInfo()
	s.Require().NoError(err)
	s.Nil(childInfo.Duration, "recipient-only information does not invent an owner clock")
	paid := s.characters.byID["bard"].Resources[resources.SpellSlotLevel1].Current
	s.Equal(1, paid)
	s.Empty(s.beats(encounter.BeatCast), "the completed cast is not told prematurely")
	s.reloadHealingScene()
	restored := s.linkedAttachment("bard", conditions.LinkedCoordinator).LinkedApplicationInfo()
	s.Equal(before, restored, "pending and live membership survive the host boundary")
	row := s.reactRow("fighter-b")
	s.Require().NotEmpty(row.ID)
	_, err = s.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "fighter-b", DeclarationID: row.ID, Answer: session.Decline()})
	s.Require().NoError(err)
	after := s.linkedAttachment("bard", conditions.LinkedCoordinator).LinkedApplicationInfo()
	s.Equal(before.ID, after.ID)
	s.Empty(after.PendingMembers)
	s.Len(after.LiveMembers, 2)
	s.Equal(before.ID, s.linkedAttachment("fighter-a", conditions.LinkedRecipient).LinkedApplicationInfo().ID)
	s.Equal(before.ID, s.linkedAttachment("fighter-b", conditions.LinkedRecipient).LinkedApplicationInfo().ID)
	s.Equal(paid, s.characters.byID["bard"].Resources[resources.SpellSlotLevel1].Current, "answering does not pay again")
	s.Require().Len(s.beats(encounter.BeatCast), 1)
	s.Len(s.beats(encounter.BeatSaved), 2, "each target's verdict is told exactly once")
	s.reloadHealingScene()
	s.Equal(after, s.linkedAttachment("bard", conditions.LinkedCoordinator).LinkedApplicationInfo())
}
