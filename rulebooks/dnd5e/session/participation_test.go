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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
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
		kinds: map[string]encounter.MemberKind{
			"dying": encounter.KindPlayer, "stabilized": encounter.KindPlayer,
			"dead": encounter.KindPlayer, "defeated": encounter.KindMonster,
			"conscious": encounter.KindPlayer,
		},
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

func TestParticipationRoutesAuthoredSheetlessMonsterByRosterKind(t *testing.T) {
	dying := dwarfCharacterRecord("dying", 0)
	monsterCollision := dwarfCharacterRecord("authored-monster", 10)
	worldCollision := dwarfCharacterRecord("world-collision", 10)
	merchant, err := npcs.NewMerchant(nil)
	require.NoError(t, err)

	asked := map[string]int{}
	seam := standingSeam{
		ctx: context.Background(),
		chars: participationCharacterStore{
			byID: map[string]*character.Data{
				"dying": dying, "authored-monster": monsterCollision,
				"world-collision": worldCollision,
			},
			asked: asked,
		},
		data: &SessionData{WorldNPCs: []PlacedWorldNPC{{
			MemberID: "world-collision", NPC: *merchant.NPC().ToData(),
		}}},
		kinds: map[string]encounter.MemberKind{
			"dying":            encounter.KindPlayer,
			"authored-monster": encounter.KindMonster,
			"world-collision":  encounter.KindWorld,
		},
	}

	ids := []encounter.MemberID{"dying", "authored-monster", "world-collision"}
	snapshot, err := seam.participation(ids)
	require.NoError(t, err)
	require.Equal(t, &encounter.ParticipationAssessment{
		Members: []encounter.MemberParticipation{
			{Member: "dying", Down: true, Turn: encounter.TurnParticipationWait},
			{Member: "authored-monster", Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait},
			{Member: "world-collision", Turn: encounter.TurnParticipationRemove},
		},
		PartyDefeated: true,
	}, snapshot.assessment)
	require.False(t, snapshot.assessment.KeepTurnOrder,
		"only actual KindPlayer provider facts participate in group policy")
	answered := make([]encounter.MemberID, 0, len(snapshot.assessment.Members))
	for _, member := range snapshot.assessment.Members {
		answered = append(answered, member.Member)
	}
	require.Equal(t, ids, answered, "one answer per request, in requested order")

	require.Equal(t, 1, asked["dying"])
	require.Zero(t, asked["authored-monster"],
		"an authored KindMonster never falls through to the character repository")
	require.Zero(t, asked["world-collision"],
		"the KindWorld collision control still reaches neither combatant store")
	require.Equal(t, LifeStateConscious, snapshot.views["authored-monster"].LifeState)
	require.True(t, snapshot.views["authored-monster"].attackTarget,
		"the existing sheetless-monster combat fallback remains up and targetable")
	require.Equal(t, LifeStateUnknown, snapshot.views["world-collision"].LifeState)
	require.False(t, snapshot.views["world-collision"].attackTarget)
}

func TestParticipationMissingPlayerFallbackDrivesTheSameViewsAndPolicy(t *testing.T) {
	dying := dwarfCharacterRecord("dying", 0)
	asked := map[string]int{}
	seam := standingSeam{
		ctx: context.Background(),
		chars: participationCharacterStore{
			byID:  map[string]*character.Data{"dying": dying},
			asked: asked,
		},
		kinds: map[string]encounter.MemberKind{
			"dying": encounter.KindPlayer, "missing-player": encounter.KindPlayer,
		},
	}

	ids := []encounter.MemberID{"dying", "missing-player"}
	snapshot, err := seam.participation(ids)
	require.NoError(t, err)
	require.Equal(t, &encounter.ParticipationAssessment{
		Members: []encounter.MemberParticipation{
			{Member: "dying", Down: true, Turn: encounter.TurnParticipationWait},
			{Member: "missing-player", Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait},
		},
		KeepTurnOrder: true,
	}, snapshot.assessment)
	require.False(t, snapshot.assessment.PartyDefeated,
		"the Conscious KindPlayer fallback used by the row must also keep the party alive")
	require.Equal(t, LifeStateConscious, snapshot.views["missing-player"].LifeState)
	require.Nil(t, snapshot.views["missing-player"].DeathSaves)
	require.True(t, snapshot.views["missing-player"].attackTarget)
	require.Equal(t, 1, asked["dying"])
	require.Equal(t, 1, asked["missing-player"])
}

func TestParticipationRefusesWrongPlayerRecordIDBeforeResolutionOrPolicy(t *testing.T) {
	asked := map[string]int{}
	seam := standingSeam{
		ctx: context.Background(),
		chars: participationCharacterStore{
			byID: map[string]*character.Data{
				"dying": dwarfCharacterRecord("dying", 0),
				"alice": dwarfCharacterRecord("different-player", 10),
			},
			asked: asked,
		},
		kinds: map[string]encounter.MemberKind{
			"dying": encounter.KindPlayer, "alice": encounter.KindPlayer,
		},
	}

	snapshot, err := seam.participation([]encounter.MemberID{"dying", "alice"})
	require.Nil(t, snapshot, "a wrong-ID repository answer must not return a partially computed policy")
	require.ErrorIs(t, err, ErrBadRepository)
	require.Contains(t, err.Error(), `character "alice"`)
	require.Contains(t, err.Error(), `returned "different-player"`)
	require.Equal(t, 1, asked["dying"])
	require.Equal(t, 1, asked["alice"])
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
		kinds: map[string]encounter.MemberKind{
			"dying":               encounter.KindPlayer,
			"character-collision": encounter.KindWorld,
			"monster-collision":   encounter.KindWorld,
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
		kinds: map[string]encounter.MemberKind{
			"dying": encounter.KindPlayer, "upright-monster": encounter.KindMonster,
		},
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
			"broken": {
				ID: "broken",
				Appearance: &customization.Appearance{Hair: &customization.HairCustomization{
					Scalp: &customization.StyleSelection{Kind: "unknown"},
				}},
			},
		}},
		kinds: map[string]encounter.MemberKind{"broken": encounter.KindPlayer},
	}
	assessment, err := seam.Assess([]encounter.MemberID{"broken"})
	require.Nil(t, assessment)
	require.ErrorIs(t, err, ErrBadCharacter)
}

func TestStandingForOwnsOneRosterKindSnapshot(t *testing.T) {
	asked := map[string]int{}
	store := participationCharacterStore{
		byID:  map[string]*character.Data{"alice": dwarfCharacterRecord("alice", 10)},
		asked: asked,
	}
	manager := &Manager{characters: store}
	authoritative := map[string]encounter.MemberKind{"alice": encounter.KindPlayer}
	seam := manager.standingFor(context.Background(), nil, authoritative)

	// A caller changing its map later cannot change the already-built seam's
	// answer or route one assessment through a second roster snapshot.
	authoritative["alice"] = encounter.KindMonster
	assessment, err := seam.Assess([]encounter.MemberID{"alice"})
	require.NoError(t, err)
	require.Equal(t, &encounter.ParticipationAssessment{Members: []encounter.MemberParticipation{{
		Member: "alice", Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait,
	}}}, assessment)
	require.Equal(t, 1, asked["alice"], "the copied KindPlayer route remains authoritative")
}

func TestParticipationRefusesMissingOrUnknownRosterKindBeforeStorage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds map[string]encounter.MemberKind
	}{
		{name: "missing", kinds: map[string]encounter.MemberKind{}},
		{name: "unknown", kinds: map[string]encounter.MemberKind{"alice": "future-kind"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asked := map[string]int{}
			seam := standingSeam{
				ctx: context.Background(),
				chars: participationCharacterStore{
					byID:  map[string]*character.Data{"alice": dwarfCharacterRecord("alice", 10)},
					asked: asked,
				},
				kinds: tc.kinds,
			}
			assessment, err := seam.Assess([]encounter.MemberID{"alice"})
			require.Nil(t, assessment)
			require.ErrorIs(t, err, ErrInvalidSession)
			require.Zero(t, asked["alice"], "kind failure precedes storage guessing")
		})
	}
}

func TestParticipationContainsNoHitPointThreshold(t *testing.T) {
	raw, err := os.ReadFile("participation.go")
	require.NoError(t, err)
	require.NotContains(t, string(raw), "HitPoints")
	require.NotContains(t, string(raw), "MaxHitPoints")
}

func TestParticipationRepositoryErrorsRemainReachable(t *testing.T) {
	hostErr := errors.New("character store unavailable")
	seam := standingSeam{
		ctx: context.Background(), chars: participationCharacterStore{err: hostErr},
		kinds: map[string]encounter.MemberKind{"alice": encounter.KindPlayer},
	}
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
