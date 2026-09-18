// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
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

// wardedWireBeat is a well-formed "warded" beat, the shape recordStrike
// writes for a Strike a Sanctuary-style ward stopped.
const wardedWireBeat = `{"beat":"warded","actor":"alice","targets":["bob"],` +
	`"attack":{"ref":"dnd5e:weapons:longsword","name":"Longsword","damage_type":"slashing"},` +
	`"warded":{"source":"cleric","save":{"saver":"alice","ability":"wisdom","roll":6,"total":8,"dc":15,"succeeded":false}}}`

func (s *SanctuaryProjectionSuite) TestWardedHasTypedBodyAndRequiresIdentity() {
	kind, body := decodeBeat([]byte(wardedWireBeat))
	s.Equal(EventWarded, kind)
	s.Equal(WardedBody{
		Attacker: "alice", Target: "bob",
		Attack:  AttackRef{Ref: "dnd5e:weapons:longsword", Name: "Longsword", DamageType: DamageSlashing},
		Source:  "cleric",
		Ability: "wisdom", Roll: 6, Total: 8, DC: 15,
	}, body)

	for _, key := range []string{"actor", "targets", "attack", "warded"} {
		var malformed map[string]any
		s.Require().NoError(json.Unmarshal([]byte(wardedWireBeat), &malformed))
		delete(malformed, key)
		raw, err := json.Marshal(malformed)
		s.Require().NoError(err)
		kind, body = decodeBeat(raw)
		s.Equal(EventWarded, kind)
		s.Nil(body, "missing %q must refuse the whole beat", key)
	}

	// The two facts a payload this composition never wrote would violate:
	// the save belongs to somebody other than the actor, or it "succeeded".
	for _, mutate := range []func(map[string]any){
		func(m map[string]any) { m["warded"].(map[string]any)["save"].(map[string]any)["saver"] = "bob" },
		func(m map[string]any) { m["warded"].(map[string]any)["save"].(map[string]any)["succeeded"] = true },
	} {
		var payload map[string]any
		s.Require().NoError(json.Unmarshal([]byte(wardedWireBeat), &payload))
		mutate(payload)
		raw, err := json.Marshal(payload)
		s.Require().NoError(err)
		_, body = decodeBeat(raw)
		s.Nil(body)
	}
}

// castWardedWireBeat is a well-formed "cast_warded" beat, the flat shape
// encounter's own castWardedPayload writes.
const castWardedWireBeat = `{"beat":"cast_warded","actor":"bard","target":"skeleton","source":"cleric",` +
	`"spell":{"ref":"dnd5e:spells:bane","name":"Bane"},"ability":"wisdom","roll":6,"total":8,"dc":15}`

func (s *SanctuaryProjectionSuite) TestCastWardedHasTypedBodyAndRequiresIdentity() {
	kind, body := decodeBeat([]byte(castWardedWireBeat))
	s.Equal(EventCastWarded, kind)
	s.Equal(CastWardedBody{
		Actor: "bard", Target: "skeleton", Source: "cleric",
		Spell:   SpellRef{Ref: "dnd5e:spells:bane", Name: "Bane"},
		Ability: "wisdom", Roll: 6, Total: 8, DC: 15,
	}, body)

	for _, key := range []string{"actor", "target", "source", "spell", "ability", "roll", "total", "dc"} {
		var malformed map[string]any
		s.Require().NoError(json.Unmarshal([]byte(castWardedWireBeat), &malformed))
		delete(malformed, key)
		raw, err := json.Marshal(malformed)
		s.Require().NoError(err)
		kind, body = decodeBeat(raw)
		s.Equal(EventCastWarded, kind)
		s.Nil(body, "missing %q must refuse the whole beat", key)
	}
}

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
