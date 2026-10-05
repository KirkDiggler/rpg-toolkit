// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// shortSight gives each named member a sight range in cells; anyone unnamed
// sees ten.
type shortSight map[encounter.MemberID]int

func (s shortSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 10
		if cells, ok := s[id]; ok {
			out[id] = cells
		}
	}
	return out, nil
}

// TestAbsentCandidateKeepsItsRowAndThePanel pins a candidate the actor's
// observed context does not hold: it is still asked about, answers DEPENDS
// rather than an error, and keeps its place.
//
// WHY HERE AND NOT THROUGH AFFORD. Afford cannot produce this candidate. Its
// candidates are the View holdings current on sight, and ObservedContext keeps
// exactly those whose latest testimony is also on sight. A holding whose
// latest testimony arrived on another channel keeps its channel-qualified
// subject in View ("member|goblin-1" — encounter's memberIntel strips the
// qualifier only for sight), so planting one makes Afford fail at candidate
// preflight ("live candidate ... has no position in the roster") before rows
// are ever attached. So the claim is pinned at attachEffects, against the real
// encounter and the real resolution.InformAttack: if the rulebook ever starts
// refusing a known target with no pairs, this fails before a player loses
// their whole panel to it.
func TestAbsentCandidateKeepsItsRowAndThePanel(t *testing.T) {
	ctx := context.Background()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{},
		Sight: shortSight{"alice": 1}, Equipment: encNoHandsObserved{},
		Standing: aggregateRecordEveryoneStanding{}, Initiative: aggregateRecordOrderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("cave", 0, 0, 10, 5)},
			Factions: []encounter.FactionInput{{ID: "goblins"}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "goblin-1", Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}, Faction: "goblins"},
			{ID: "goblin-2", Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 3}, Faction: "goblins"},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	observed, err := enc.ObservedContext(&encounter.ViewInput{Member: "alice"})
	require.NoError(t, err)
	for _, member := range observed.Members {
		require.NotEqual(t, encounter.MemberID("goblin-2"), member.ID, "precondition: goblin two is not observed")
	}

	sneak, err := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{MemberID: "alice", Level: 1}).ToJSON()
	require.NoError(t, err)
	sheet, err := character.Load(ctx, &character.Data{
		ID: "alice", PlayerID: "player-alice", Name: "alice", Level: 1, ClassID: classes.Rogue, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 16, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
		},
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 14, ProficiencyBonus: 2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponShortsword},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: string(weapons.Shortsword)},
		Conditions:     []json.RawMessage{sneak},
	})
	require.NoError(t, err)
	definition, err := character.AssembleAttack(sheet, &character.AssembleAttackInput{Slot: character.SlotMainHand})
	require.NoError(t, err)

	offers := []compiledOffer{{
		declaration: Declaration{
			Verb: VerbAttack, TargetKind: TargetMember,
			Candidates: []TargetCandidate{{Member: "goblin-1", Available: true}, {Member: "goblin-2"}},
		},
		attack: &definition,
	}}
	require.NoError(t, attachEffects(&attachEffectsInput{
		Encounter: enc, Member: "alice", Actor: sheet, Offers: offers,
	}), "an unobserved candidate is not an error")

	declaration := offers[0].declaration
	sneakID := refs.Features.SneakAttack().String()
	require.Len(t, declaration.Effects, 1)
	require.Equal(t, sneakID, declaration.Effects[0].ID)
	require.Equal(t, EffectDepends, declaration.Effects[0].State)

	require.Len(t, declaration.Candidates, 2, "the unobserved candidate is never dropped")
	absent := declaration.Candidates[1]
	require.Equal(t, "goblin-2", absent.Member)
	require.Equal(t, []TargetEffect{{
		ID: sneakID, State: EffectDepends,
		Reason: "Needs advantage or another enemy of the target within 5 feet",
	}}, absent.Effects, "what the actor cannot see stays unresolved, never negative")
}
