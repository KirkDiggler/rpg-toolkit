// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type aggregateRecordOrderAsGiven struct{}

func (aggregateRecordOrderAsGiven) RollInitiative(
	members []encounter.MemberID,
) ([]encounter.MemberID, error) {
	return members, nil
}

type aggregateRecordEveryoneStanding struct{}

func (aggregateRecordEveryoneStanding) Standing(
	_ []encounter.MemberID,
) ([]encounter.MemberID, error) {
	return nil, nil
}

type aggregateRecordEveryoneSees struct{}

func (aggregateRecordEveryoneSees) Sight(
	members []encounter.MemberID,
) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, member := range members {
		out[member] = 1_000_000
	}
	return out, nil
}

type scriptedDice struct {
	rolls []int
	next  int
}

func (d *scriptedDice) Roll(_ context.Context, _ int) (int, error) {
	if d.next >= len(d.rolls) {
		return 0, fmt.Errorf("scriptedDice: asked for roll %d of %d", d.next+1, len(d.rolls))
	}
	roll := d.rolls[d.next]
	d.next++
	return roll, nil
}

// TestRecordUsesAggregateFromTypedStrikeOutcome pins the session boundary at
// the real encounter recording seam: resolution may retain typed damage
// evidence, but the story stores only the aggregate amount.
func TestRecordUsesAggregateFromTypedStrikeOutcome(t *testing.T) {
	struck := resolution.StrikeOutcome{
		Roll:     15,
		Total:    20,
		TargetAC: 12,
		Hit:      true,
		Damage:   9,
		DamageInstances: []damage.Instance{
			{Amount: 5, Type: damage.Slashing},
			{Amount: 4, Type: damage.Fire},
		},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{},
		Sight:      aggregateRecordEveryoneSees{},
		Standing:   aggregateRecordEveryoneStanding{},
		Initiative: aggregateRecordOrderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	// A minimal but real profile — Ref and Name non-empty, per
	// rpg-toolkit#1172/#1175: Record refuses an Attack whose Ref or Name is
	// empty. This test is about damage aggregation, not attack identity, so
	// the profile is otherwise bare; DamageType stays unchecked and empty is
	// still a legal answer for it.
	definition := combatActions.Definition{Ref: *refs.Weapons.Longsword(), Name: "Longsword"}

	recorded, err := enc.Record(recordFor(
		&AttackInput{Attacker: "alice", Target: "bob"}, struck, definition,
	))
	require.NoError(t, err)
	require.NotZero(t, recorded.Seq)

	story, err := enc.Story(&encounter.StoryInput{Audience: "bob"})
	require.NoError(t, err)
	require.NotEmpty(t, story)
	require.JSONEq(t,
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":9,`+
			`"critical":false,"attack":{"ref":"longsword","name":"Longsword","damage_type":""}}`,
		string(story[len(story)-1].Payload),
		"the persisted encounter record carries the aggregate and no typed damage collection",
	)
}

// halfBrokenCharacters writes the first sheet and refuses the second.
type halfBrokenCharacters struct {
	written []string
	err     error
}

func (h *halfBrokenCharacters) GetCharacter(context.Context, string) (*character.Data, error) {
	return nil, ErrNotFound
}

func (h *halfBrokenCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	if len(h.written) > 0 {
		return h.err
	}
	h.written = append(h.written, data.ID)
	return nil
}

// TestAPartialCharacterSaveNamesWhatLanded pins S6 across MULTIPLE sheets.
//
// One swing can dirty both sides — the target takes damage, and an effect that
// writes the attacker's own sheet dirties them too. If the second write fails,
// the first is ALREADY DURABLE, and a caller told only about the failure would
// retry a write that succeeded. Naming both halves is the difference between
// repair and retry, which is the whole reason the report exists.
//
// Tested against saveDirty directly rather than through a swing, because no v1
// interaction dirties two character sheets: the strike damages its target and
// nothing yet writes the attacker back. A test that drove this through Attack
// would skip, and a skipping test proves nothing. When an effect that writes
// the swinger arrives, this pin is already here and already honest.
func TestAPartialCharacterSaveNamesWhatLanded(t *testing.T) {
	broken := errors.New("store is down")
	store := &halfBrokenCharacters{err: broken}
	m := &Manager{characters: store}

	err := m.saveDirty(context.Background(), &writeScope{data: &SessionData{}}, &resolution.Output{
		DirtyCharacters: []*character.Data{{ID: "alice"}, {ID: "bob"}},
	})

	require.Error(t, err)
	var saved *SaveError
	require.ErrorAs(t, err, &saved)
	require.ErrorIs(t, err, broken)

	require.Equal(t, []string{"character:alice"}, saved.Report.Written,
		"alice's sheet is durable and the caller must not be told to retry it")
	require.Equal(t, []string{"character:bob"}, saved.Report.Failed,
		"and bob's is the one that needs repair")
}

func cloneFixture[T any](in *T) (*T, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type strikeSessions struct{ byID map[string]*SessionData }

func (s *strikeSessions) GetSession(_ context.Context, id string) (*SessionData, error) {
	data, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneFixture(data)
}

func (s *strikeSessions) SaveSession(_ context.Context, data *SessionData) error {
	s.byID[data.ID] = data
	return nil
}

type strikeEncounters struct {
	byID map[string]*encounter.EncounterData
}

func (s *strikeEncounters) GetEncounter(_ context.Context, id string) (*encounter.EncounterData, error) {
	data, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneFixture(data)
}

func (s *strikeEncounters) SaveEncounter(_ context.Context, id string, data *encounter.EncounterData) error {
	s.byID[id] = data
	return nil
}

type strikeCharacters struct{ byID map[string]*character.Data }

func (s *strikeCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	data, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneFixture(data)
}

func (s *strikeCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	s.byID[data.ID] = data
	return nil
}

func strikeFixtureFighter(id string) *character.Data {
	return &character.Data{
		ID:       id,
		PlayerID: "player-" + id,
		Name:     id,
		Level:    3,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints:           24,
		MaxHitPoints:        28,
		ArmorClass:          16,
		ProficiencyBonus:    2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponMartial},
		Inventory: []character.InventoryItemData{{
			Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1,
		}},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: string(weapons.Longsword)},
	}
}

// TestStrikeRefusesAPersistedMonsterPriceBeforeRolling pins the one door the
// driven monster strike now shares with a player's own swing: if persisted
// content declares a non-nil price, it is refused there rather than executing
// free.
func TestStrikeRefusesAPersistedMonsterPriceBeforeRolling(t *testing.T) {
	ctx := context.Background()
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	characters := &strikeCharacters{byID: map[string]*character.Data{"fighter": strikeFixtureFighter("fighter")}}
	roller := &scriptedDice{rolls: []int{17, 4}}

	mgr, err := NewManager(&Config{
		Dice: roller, TurnDriver: Pass{},
		Sessions: sessions, Encounters: encounters, Characters: characters, Events: DiscardEvents{},
	})
	require.NoError(t, err)

	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: aggregateRecordEveryoneSees{},
		Initiative: aggregateRecordOrderAsGiven{}, TurnDriver: passDriver{}, Standing: aggregateRecordEveryoneStanding{},
		Field:     encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)}},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	worldData := world.ToData()

	_, err = mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &worldData})
	require.NoError(t, err)
	_, err = mgr.Join(ctx, &JoinInput{Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0}})
	require.NoError(t, err)
	_, err = mgr.Spawn(ctx, &SpawnInput{Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(), Position: spatial.Position{X: 1, Y: 0}})
	require.NoError(t, err)

	stored, err := sessions.GetSession(ctx, "sess")
	require.NoError(t, err)
	var actionRef *core.Ref
	for i := range stored.NPCs {
		if stored.NPCs[i].ID != "skel-1" {
			continue
		}
		for j := range stored.NPCs[i].Actions {
			stored.NPCs[i].Actions[j].Cost = &combat.SpendProfile{
				Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
			}
		}
		actionRef = refs.MonsterActions.SkeletonShortsword()
	}
	require.NotNil(t, actionRef, "the spawned skeleton must be the persisted attacker under test")
	require.NoError(t, sessions.SaveSession(ctx, stored))

	roller.next = 0 // isolate the strike: Join/Spawn already spent formation dice.

	scope, err := mgr.openForWrite(ctx, "sess")
	require.NoError(t, err)
	beforeStory, err := scope.enc.Story(&encounter.StoryInput{Audience: "fighter"})
	require.NoError(t, err)
	beforeHP := characters.byID["fighter"].HitPoints

	err = (strikerSeam{m: mgr, scope: scope}).Strike(ctx, scope.enc, "skel-1", "fighter", *actionRef)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBadCost)
	require.NotErrorIs(t, err, resolution.ErrNoPayer, "the seam translates resolution's monster-payer refusal")
	require.Zero(t, roller.next, "the priced strike is refused before attack or damage dice roll")
	require.Equal(t, beforeHP, characters.byID["fighter"].HitPoints, "no free hit lands when pricing is declared")
	afterStory, err := scope.enc.Story(&encounter.StoryInput{Audience: "fighter"})
	require.NoError(t, err)
	require.Len(t, afterStory, len(beforeStory), "an unrecorded refusal must not append a strike beat")
}
