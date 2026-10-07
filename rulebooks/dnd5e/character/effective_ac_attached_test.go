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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// EffectiveACAttachmentSuite pins EffectiveAC's refusals.
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
//  3. A CONTRIBUTOR THAT CANNOT READ ITS HOLDER. Attached, but on a context
//     with no cast (or a cast without this character), Unarmored Defense
//     cannot learn the WIS it adds. It used to leave the chain untouched and
//     the fold answered base armour; it now refuses with gamectx.ErrNotInCast
//     and the fold returns that, by (2).
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
		MemberID: "char-monk",
		Type:     conditions.UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
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

// The other half: attached AND in a cast, the very same bytes fold Unarmored
// Defense and report 15. Without this, a permanently-erroring EffectiveAC would
// pass above.
//
// Attachment alone is no longer enough, and the split below says why: the sheet
// has to be on a bus to fold at all, and the condition has to be able to find
// itself in the cast to contribute. See castOf for why installing one here
// stands in for a real installer rather than inventing one.
func (s *EffectiveACAttachmentSuite) TestAttachedSheetFoldsUnarmoredDefense() {
	loaded, err := Load(s.ctx, s.monkData())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, events.NewEventBus()))

	breakdown, acErr := loaded.EffectiveAC(castOf(s.ctx, loaded))

	s.Require().NoError(acErr)
	s.Require().NotNil(breakdown)
	s.Assert().Equal(15, breakdown.Total, "10 base + 3 DEX + 2 WIS (Unarmored Defense)")
}

// Attached but with NO cast: Unarmored Defense cannot find its holder, so the
// fold REFUSES with gamectx.ErrNotInCast rather than answering 13.
//
// This test used to pin 13 — "10 base + 3 DEX, and no Unarmored Defense" — on
// the theory that a condition that cannot answer should leave the chain
// untouched so it cannot poison the fold for everybody else. That theory was
// written when EffectiveAC swallowed fold errors. It no longer does, and a
// missing AC contributor is not a smaller answer, it is a wrong one: rpg-api
// folded exactly this way on equip and saved a monk's AC without WIS
// (rpg-toolkit#1965 tier 1 #2). The refusal is the answer.
//
// The answer for such a caller is still not to install a cast of its own. It
// is R6: bring the fold to resolution, where one door installs the truth on
// every path. resolution.ProjectCharacter is that entry for a caller holding a
// record.
func (s *EffectiveACAttachmentSuite) TestAttachedSheetWithoutACastRefusesEffectiveAC() {
	loaded, err := Load(s.ctx, s.monkData())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, events.NewEventBus()))

	breakdown, acErr := loaded.EffectiveAC(s.ctx)

	s.Require().ErrorIs(acErr, gamectx.ErrNotInCast,
		"Unarmored Defense could not read its holder, so 13 would be a lie")
	s.Assert().Nil(breakdown, "a refused read returns no breakdown to be mistaken for an answer")
}

// A cast that does not hold the monk is the same lie by another route: the
// condition asks for its holder and nobody can name it.
func (s *EffectiveACAttachmentSuite) TestACastWithoutTheHolderRefusesEffectiveAC() {
	loaded, err := Load(s.ctx, s.monkData())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, events.NewEventBus()))

	ctx := gamectx.WithCast(s.ctx, &fakeCast{members: map[string]combat.Member{}})
	breakdown, acErr := loaded.EffectiveAC(ctx)

	s.Require().ErrorIs(acErr, gamectx.ErrNotInCast)
	s.Assert().Nil(breakdown)
}

// The refusal is the condition's, not a blanket "no cast, no AC": a sheet with
// nothing that reads the cast folds on a bare context exactly as before. Without
// this, an EffectiveAC that refused every castless call would pass above.
func (s *EffectiveACAttachmentSuite) TestASheetNeedingNoCastStillFoldsWithoutOne() {
	data := s.monkData()
	data.Conditions = nil
	loaded, err := Load(s.ctx, data)
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, events.NewEventBus()))

	breakdown, acErr := loaded.EffectiveAC(s.ctx)

	s.Require().NoError(acErr)
	s.Require().NotNil(breakdown)
	s.Assert().Equal(13, breakdown.Total, "10 base + 3 DEX, and nothing on the sheet that adds to it")
}

// An attached monk on a bare context cannot produce an equipment view either:
// the view carries the folded AC and inherits its refusal.
func (s *EffectiveACAttachmentSuite) TestAttachedSheetWithoutACastRefusesEquipmentView() {
	loaded, err := Load(s.ctx, s.monkData())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, events.NewEventBus()))

	view, viewErr := loaded.EquipmentView(s.ctx)

	s.Require().ErrorIs(viewErr, gamectx.ErrNotInCast)
	s.Assert().Nil(view)
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
