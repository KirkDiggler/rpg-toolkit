// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type participationCharacterStore struct {
	byID  map[string]*character.Data
	asked map[string]int
	err   error
}

func (s participationCharacterStore) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	if s.asked != nil {
		s.asked[id]++
	}
	if s.err != nil {
		return nil, s.err
	}
	data, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *data
	return &copy, nil
}

func (participationCharacterStore) SaveCharacter(context.Context, *character.Data) error { return nil }

func TestParticipationMapsProviderFactsWithoutThresholds(t *testing.T) {
	dying := dwarfCharacterRecord("dying", 0)
	dying.DeathSaveState = &saves.DeathSaveState{Successes: 1, Failures: 1}
	stabilized := dwarfCharacterRecord("stabilized", 0)
	stabilized.DeathSaveState = &saves.DeathSaveState{Successes: 3, Stabilized: true}
	dead := dwarfCharacterRecord("dead", 0)
	dead.DeathSaveState = &saves.DeathSaveState{Failures: 3, Dead: true}
	conscious := dwarfCharacterRecord("conscious", 1)
	monsterSheet, err := instantiate("defeated", "dnd5e:monsters:skeleton")
	require.NoError(t, err)
	monsterSheet.HitPoints = 0

	seam := standingSeam{
		ctx: context.Background(),
		chars: participationCharacterStore{byID: map[string]*character.Data{
			"dying": dying, "stabilized": stabilized, "dead": dead, "conscious": conscious,
		}},
		data: &SessionData{NPCs: []monster.Data{*monsterSheet}},
	}
	ids := []encounter.MemberID{"dying", "stabilized", "dead", "defeated", "conscious"}

	snapshot, err := seam.participation(ids)
	require.NoError(t, err)
	require.Equal(t, &encounter.ParticipationAssessment{
		Members: []encounter.MemberParticipation{
			{Member: "dying", Down: true, Turn: encounter.TurnParticipationWait},
			{Member: "stabilized", Down: true, Turn: encounter.TurnParticipationAutoPass},
			{Member: "dead", Down: true, Turn: encounter.TurnParticipationRemove},
			{Member: "defeated", Down: true, Turn: encounter.TurnParticipationRemove},
			{Member: "conscious", Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait},
		},
		KeepTurnOrder: true,
	}, snapshot.assessment)

	require.Equal(t, LifeStateDying, snapshot.views["dying"].LifeState)
	require.Equal(t, &DeathSaveProgress{
		Successes: 1, Failures: 1, SuccessesNeeded: 2, FailuresRemaining: 2,
	}, snapshot.views["dying"].DeathSaves)
	require.Equal(t, LifeStateStabilized, snapshot.views["stabilized"].LifeState)
	require.Equal(t, &DeathSaveProgress{
		Successes: 3, Failures: 0, SuccessesNeeded: 0, FailuresRemaining: 3, Stabilized: true,
	}, snapshot.views["stabilized"].DeathSaves)
	require.Equal(t, LifeStateDead, snapshot.views["dead"].LifeState)
	require.Equal(t, LifeStateDefeated, snapshot.views["defeated"].LifeState)
	require.Nil(t, snapshot.views["defeated"].DeathSaves)
	require.Equal(t, LifeStateConscious, snapshot.views["conscious"].LifeState)
	require.Nil(t, snapshot.views["conscious"].DeathSaves)

	down, err := seam.Standing(ids)
	require.NoError(t, err)
	require.Equal(t, ids[:4], down, "binary Standing delegates to the same rich answer")
}

func TestParticipationWorldNPCIdentityWinsBeforeCombatantLookup(t *testing.T) {
	dying := dwarfCharacterRecord("dying", 0)
	characterCollision := dwarfCharacterRecord("character-collision", 10)
	monsterCollisionCharacter := dwarfCharacterRecord("monster-collision", 10)
	monsterCollision, err := instantiate("monster-collision", "dnd5e:monsters:skeleton")
	require.NoError(t, err)
	merchant, err := npcs.NewMerchant(nil)
	require.NoError(t, err)

	asked := map[string]int{}
	seam := standingSeam{
		ctx: context.Background(),
		chars: participationCharacterStore{
			byID: map[string]*character.Data{
				"dying": dying, "character-collision": characterCollision,
				"monster-collision": monsterCollisionCharacter,
			},
			asked: asked,
		},
		data: &SessionData{
			NPCs: []monster.Data{*monsterCollision},
			WorldNPCs: []PlacedWorldNPC{
				{MemberID: "character-collision", NPC: *merchant.NPC().ToData()},
				{MemberID: "monster-collision", NPC: *merchant.NPC().ToData()},
			},
		},
	}

	ids := []encounter.MemberID{"dying", "character-collision", "monster-collision"}
	snapshot, err := seam.participation(ids)
	require.NoError(t, err)
	require.Equal(t, &encounter.ParticipationAssessment{
		Members: []encounter.MemberParticipation{
			{Member: "dying", Down: true, Turn: encounter.TurnParticipationWait},
			{Member: "character-collision", Turn: encounter.TurnParticipationRemove},
			{Member: "monster-collision", Turn: encounter.TurnParticipationRemove},
		},
		PartyDefeated: true,
	}, snapshot.assessment)
	require.False(t, snapshot.assessment.KeepTurnOrder,
		"world NPC collisions cannot masquerade as conscious player allies")

	for _, id := range []string{"character-collision", "monster-collision"} {
		require.Zero(t, asked[id], "%s must be classified before character lookup", id)
		view := snapshot.views[id]
		require.Equal(t, LifeStateUnknown, view.LifeState)
		require.Nil(t, view.DeathSaves)
		require.False(t, view.attackTarget,
			"a KindWorld bystander cannot leak into Attack candidates")
	}
	require.Equal(t, 1, asked["dying"], "the actual player is still fetched once")
}

func TestParticipationPartyDefeatUsesOnlyRequestedPlayers(t *testing.T) {
	dying := dwarfCharacterRecord("dying", 0)
	monsterSheet, err := instantiate("upright-monster", "dnd5e:monsters:skeleton")
	require.NoError(t, err)

	seam := standingSeam{
		ctx:   context.Background(),
		chars: participationCharacterStore{byID: map[string]*character.Data{"dying": dying}},
		data:  &SessionData{NPCs: []monster.Data{*monsterSheet}},
	}
	assessment, err := seam.Assess([]encounter.MemberID{"dying", "upright-monster"})
	require.NoError(t, err)
	require.True(t, assessment.PartyDefeated,
		"a conscious monster does not keep a non-empty player party alive")
	require.False(t, assessment.KeepTurnOrder, "there is no conscious player ally")
}

func TestParticipationKeepsLifeStateWhenProgressIsAbsent(t *testing.T) {
	view := participantView{LifeState: LifeStateDying}
	require.Equal(t, LifeStateDying, view.LifeState)
	require.Nil(t, view.DeathSaves,
		"progress absence cannot be interpreted as a different provider state")
}

func TestParticipationResolutionFailuresUseSessionVocabulary(t *testing.T) {
	seam := standingSeam{
		ctx: context.Background(),
		chars: participationCharacterStore{byID: map[string]*character.Data{
			"broken": {},
		}},
	}
	assessment, err := seam.Assess([]encounter.MemberID{"broken"})
	require.Nil(t, assessment)
	require.ErrorIs(t, err, ErrBadCharacter)
}

func TestParticipationContainsNoHitPointThreshold(t *testing.T) {
	raw, err := os.ReadFile("participation.go")
	require.NoError(t, err)
	require.NotContains(t, string(raw), "HitPoints")
	require.NotContains(t, string(raw), "MaxHitPoints")
}

func TestParticipationRepositoryErrorsRemainReachable(t *testing.T) {
	hostErr := errors.New("character store unavailable")
	seam := standingSeam{ctx: context.Background(), chars: participationCharacterStore{err: hostErr}}
	assessment, err := seam.Assess([]encounter.MemberID{"alice"})
	require.Nil(t, assessment)
	require.ErrorIs(t, err, hostErr)
}

func dwarfCharacterRecord(id string, hp int) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: id,
		Level: 1, ProficiencyBonus: 2, RaceID: races.Dwarf, ClassID: classes.Fighter,
		HitPoints: hp, MaxHitPoints: 10, ArmorClass: 12,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 12, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
	}
}
