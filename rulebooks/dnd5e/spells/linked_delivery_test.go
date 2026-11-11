// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// Use the current cast declaration as the creator's consumer. A leftover opaque
// source_id parameter would ask the linked descriptor to read a retired config.
func (s *MaintenanceDeclarationsSuite) TestNamedCastDeliveryUsesCurrentOriginAndAddressBindings() {
	for _, spell := range []spells.Spell{spells.Bless, spells.Bane} {
		s.Run(string(spell), func() {
			def := spells.CastDefinition(spells.CastDefinitionInput{Spell: spell, SpellSaveDC: 13})
			s.Require().NotNil(def)
			effect := def.Cast.Effects[0]
			params := map[string]json.RawMessage{}
			if len(effect.Parameters) > 0 {
				s.Require().NoError(json.Unmarshal(effect.Parameters, &params))
			}
			if effect.CounterpartKey != "" {
				params[effect.CounterpartKey] = json.RawMessage(`"caster"`)
			}
			var config json.RawMessage
			if len(params) > 0 {
				raw, err := json.Marshal(params)
				s.Require().NoError(err)
				config = raw
			}
			prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "cast-1", CoordinatorID: "caster", Origin: dndevents.ConditionOrigin{Ref: def.Ref, EntityID: "caster"},
				Existing: []conditions.ParticipantConditions{{MemberID: "caster"}, {MemberID: "target"}}, Spec: conditions.ApplicationSpec{Name: def.Name, CoordinatorParts: def.Cast.Maintenance,
					Recipients: []conditions.RecipientSpec{{MemberID: "target", Ref: effect.Ref.String(), AddressSourceID: "caster", Config: config, Lifetime: conditions.LifetimeShared}}}})
			s.Require().NoError(err)
			candidate, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
			s.Require().NoError(err)
			s.Equal("caster", candidate.Candidate.Address.SourceID)
			origin := candidate.Condition.(dndevents.ConditionOriginProvider).ConditionOrigin()
			s.Equal(def.Ref, origin.Ref)
			s.Equal("caster", origin.EntityID)
		})
	}
}
