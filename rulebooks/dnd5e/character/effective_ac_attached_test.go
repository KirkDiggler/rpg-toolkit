// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// EffectiveACAttachmentSuite pins EffectiveAC's two refusals.
//
// They are different failures and it is worth not conflating them, because the
// first is the one everybody assumes and the second is the one that actually
// cost us a wrong AC in production:
//
//  1. NO BUS AT ALL. Folding a chain on a nil bus PANICS — verified by
//     mutation: disabling the guard turns these tests into a nil-bus panic, not
//     a quiet 13. So this guard converts a crash in a read path into a typed
//     error. It is not preventing a silent wrong answer; there was never a
//     silent answer to prevent here.
//
//  2. A CONTRIBUTOR THAT FAILS MID-FOLD. This is the silent one. EffectiveAC
//     used to discard both the publish and the execute error and return
//     whatever the breakdown held, so one broken contributor degraded the total
//     to base armour with nothing in the call stack saying so — how a monk
//     fought at 10+DEX with Unarmored Defense attached. Those errors are now
//     returned.
//
// Both halves of (1) matter. The refusal is only meaningful if the same sheet,
// once attached, actually produces the higher number; otherwise these would
// pass against an EffectiveAC that had simply been broken.
type EffectiveACAttachmentSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *EffectiveACAttachmentSuite) SetupTest() { s.ctx = context.Background() }

// monkData is a level-1 monk with Unarmored Defense persisted the way the
// class grant writes it: DEX 16 (+3) and WIS 14 (+2), wearing nothing.
//
// Correct AC is 15. Base armour is 13. The two differ by the WIS modifier,
// which is precisely the contribution an unattached sheet loses, so a wrong
// answer here cannot coincide with the right one.
func (s *EffectiveACAttachmentSuite) monkData() *Data {
	ud := conditions.NewUnarmoredDefenseCondition(conditions.UnarmoredDefenseInput{
		CharacterID: "char-monk",
		Type:        conditions.UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})
	raw, err := ud.ToJSON()
	s.Require().NoError(err)

	return &Data{
		ID:               "char-monk",
		PlayerID:         "player-1",
		Name:             "Attachment Monk",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12,
			abilities.DEX: 16, // +3
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 14, // +2
			abilities.CHA: 8,
		},
		HitPoints:      9,
		MaxHitPoints:   9,
		ArmorClass:     15,
		EquipmentSlots: EquipmentSlots{},
		Conditions:     []json.RawMessage{raw},
	}
}

// A bus-free Load is the shape rpg-api and the session SDK both reach for when
// they only want to read static facts off a sheet. Asking THAT sheet for an
// effective AC used to panic on the nil bus; it is now refused by name.
func (s *EffectiveACAttachmentSuite) TestUnattachedSheetRefusesEffectiveAC() {
	loaded, err := Load(s.ctx, s.monkData())
	s.Require().NoError(err)

	breakdown, acErr := loaded.EffectiveAC(s.ctx)

	s.Require().Error(acErr, "an unattached sheet must refuse by name rather than panic on its nil bus")
	s.Assert().Nil(breakdown, "a refused read returns no breakdown to be mistaken for an answer")
}

// The other half: attached, the very same bytes fold Unarmored Defense and
// report 15. Without this, a permanently-erroring EffectiveAC would pass above.
func (s *EffectiveACAttachmentSuite) TestAttachedSheetFoldsUnarmoredDefense() {
	loaded, err := Load(s.ctx, s.monkData())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, events.NewEventBus()))

	breakdown, acErr := loaded.EffectiveAC(s.ctx)

	s.Require().NoError(acErr)
	s.Require().NotNil(breakdown)
	s.Assert().Equal(15, breakdown.Total, "10 base + 3 DEX + 2 WIS (Unarmored Defense)")
}

// EquipmentView carries a folded AC, so it inherits the refusal rather than
// reporting base armour in a display the player reads as authoritative.
func (s *EffectiveACAttachmentSuite) TestUnattachedSheetRefusesEquipmentView() {
	loaded, err := Load(s.ctx, s.monkData())
	s.Require().NoError(err)

	view, viewErr := loaded.EquipmentView(s.ctx)

	s.Require().Error(viewErr)
	s.Assert().Nil(view)
}

func TestEffectiveACAttachmentSuite(t *testing.T) {
	suite.Run(t, new(EffectiveACAttachmentSuite))
}
