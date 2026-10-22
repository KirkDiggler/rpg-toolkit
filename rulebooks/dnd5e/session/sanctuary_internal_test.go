// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

type SanctuaryProjectionSuite struct{ suite.Suite }

func TestSanctuaryProjectionSuite(t *testing.T) { suite.Run(t, new(SanctuaryProjectionSuite)) }

// TestWardedSurvivesOutcomeConversion is recordStrike's own half of
// [BlessProjectionSuite.TestMissSurvivesOutcomeConversion]: a Warded
// StrikeOutcome becomes a RecordInput carrying OutcomeWarded and nothing a
// struck/missed beat would — no PresentationID, no top-level Calculation.
func (s *SanctuaryProjectionSuite) TestWardedSurvivesOutcomeConversion() {
	struck := resolution.StrikeOutcome{
		AttackerID: "alice", TargetID: "bob",
		Warded: &resolution.WardOutcome{
			SourceID: "cleric", Ability: abilities.WIS,
			Save: &saves.SavingThrowResult{Roll: 6, Total: 8, DC: 15, Success: false},
		},
	}
	ref := AttackRef{Ref: "dnd5e:weapons:longsword", Name: "Longsword", DamageType: DamageSlashing}

	recorded := recordStrike("alice", "bob", struck, ref, "presentation-should-be-dropped", nil, nil)

	s.Equal(encounter.OutcomeWarded, recorded.Kind)
	s.Equal(encounter.MemberID("alice"), recorded.Actor)
	s.Equal([]encounter.MemberID{"bob"}, recorded.Targets)
	s.Empty(recorded.PresentationID, "no attack roll happened, so no shared token names one")
	s.Nil(recorded.Calculation, "the top-level field is Struck/Missed's, not a blocked attack's")
	s.Require().NotNil(recorded.Warded)
	s.Equal(encounter.MemberID("cleric"), recorded.Warded.Source)
	s.Equal(encounter.MemberID("alice"), recorded.Warded.Save.Saver, "the ATTACKER saved, not the target")
	s.Equal("wis", recorded.Warded.Save.Ability)
	s.Equal(6, recorded.Warded.Save.Roll)
	s.Equal(15, recorded.Warded.Save.DC)
	s.False(recorded.Warded.Save.Succeeded)
}

// TestCastWardedSurvivesOutcomeConversion is castOutcome's own half, the
// Cast-door twin of [BlessProjectionSuite.TestMissSurvivesOutcomeConversion].
func (s *SanctuaryProjectionSuite) TestCastWardedSurvivesOutcomeConversion() {
	spell := SpellRef{Ref: refs.Spells.Bane().String(), Name: "Bane"}
	targets, pushes, err := castOutcome(resolution.CastOutcome{
		Spell: *refs.Spells.Bane(), CasterID: "bard",
		Targets: []resolution.CastTargetOutcome{{
			TargetID: "skeleton",
			Warded: &resolution.WardOutcome{
				SourceID: "cleric", Ability: abilities.WIS,
				Save: &saves.SavingThrowResult{Roll: 6, Total: 8, DC: 15, Success: false},
			},
		}},
	}, "bard", spell)
	s.Require().NoError(err)
	s.Empty(pushes)
	s.Require().Len(targets, 1)

	target := targets[0]
	s.Equal(encounter.MemberID("skeleton"), target.Target)
	s.False(target.Missed)
	s.Nil(target.Save)
	s.Empty(target.Results)
	s.Require().NotNil(target.Warded)
	s.Equal(encounter.MemberID("cleric"), target.Warded.Source)
	s.Equal(encounter.MemberID("bard"), target.Warded.Save.Saver, "the CASTER saved, not the named target")
	s.Equal("wis", target.Warded.Save.Ability)
	s.Equal(6, target.Warded.Save.Roll)
	s.Equal(15, target.Warded.Save.DC)
	s.False(target.Warded.Save.Succeeded)
}
