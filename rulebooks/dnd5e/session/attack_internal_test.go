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
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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

// TestRecordProjectsSelectedStrikeDetail pins the one session-to-encounter
// projection. Resolution keeps richer internal evidence; the replay carrier
// receives only the approved ordered subset and never modifier prose.
func TestRecordProjectsSelectedStrikeDetail(t *testing.T) {
	immunity := 0.0
	struck := resolution.StrikeOutcome{
		Roll: 15, Total: 20, TargetAC: 12, Hit: true, Damage: 9,
		DamageInstances: []damage.Instance{
			{Amount: 5, Type: damage.Slashing},
			{Amount: 4, Type: damage.Fire},
		},
		DamageComponents: []dnd5eEvents.DamageComponent{
			{
				Source: dnd5eEvents.DamageSourceWeapon, SourceRef: refs.Weapons.Longsword(), Dice: "1d8",
				OriginalDiceRolls: []int{2}, FinalDiceRolls: []int{4},
				Rerolls:   []dnd5eEvents.RerollEvent{{DieIndex: 0, Before: 2, After: 4, Reason: "ignored"}},
				FlatBonus: 0, DamageType: damage.Slashing,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier}, IsCritical: true,
			},
			{
				Source: dnd5eEvents.DamageSourceMonsterTrait, SourceRef: refs.MonsterTraits.Immunity(),
				DamageType: damage.Slashing, Multiplier: &immunity,
			},
		},
		Folded: dnd5eEvents.AttackChainEvent{
			AdvantageSources: []dnd5eEvents.AttackModifierSource{
				{SourceRef: refs.Conditions.Hidden(), SourceID: "alice", Reason: "Hidden"},
			},
			DisadvantageSources: []dnd5eEvents.AttackModifierSource{
				{SourceRef: refs.Conditions.Dodging(), SourceID: "bob", Reason: "Dodging"},
			},
		},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{},
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

	definition := combatActions.Definition{Ref: *refs.Weapons.Longsword(), Name: "Longsword"}
	recorded, err := enc.Record(recordFor(
		&AttackInput{Attacker: "alice", Target: "bob"}, struck, definition,
	))
	require.NoError(t, err)
	require.NotZero(t, recorded.Seq)

	story, err := enc.Story(&encounter.StoryInput{Audience: "bob"})
	require.NoError(t, err)
	require.NotEmpty(t, story)
	payload := string(story[len(story)-1].Payload)
	require.JSONEq(t,
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":9,`+
			`"critical":false,"attack":{"ref":"dnd5e:weapons:longsword","name":"Longsword","damage_type":""},`+
			`"damage_components":[`+
			`{"source":"weapon","source_ref":"dnd5e:weapons:longsword","dice":"1d8","final_rolls":[4],"flat_bonus":0,"damage_type":"slashing"},`+
			`{"source":"monster_trait","source_ref":"dnd5e:monster_traits:immunity","flat_bonus":0,"damage_type":"slashing","multiplier":0}],`+
			`"advantage_sources":[{"source_ref":"dnd5e:conditions:hidden","source_id":"alice"}],`+
			`"disadvantage_sources":[{"source_ref":"dnd5e:conditions:dodging","source_id":"bob"}]}`,
		payload,
	)
	for _, excluded := range []string{
		`"original_dice_rolls"`, `"rerolls"`, `"properties"`, `"is_critical"`, `"reason"`,
	} {
		require.NotContains(t, payload, excluded)
	}
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

type strikeSessions struct {
	byID  map[string]*SessionData
	saves int
}

func (s *strikeSessions) GetSession(_ context.Context, id string) (*SessionData, error) {
	data, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneFixture(data)
}

func (s *strikeSessions) SaveSession(_ context.Context, data *SessionData) error {
	s.saves++
	s.byID[data.ID] = data
	return nil
}

type strikeEncounters struct {
	byID    map[string]*encounter.EncounterData
	saves   int
	records int
}

func (s *strikeEncounters) GetEncounter(_ context.Context, id string) (*encounter.EncounterData, error) {
	data, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneFixture(data)
}

func (s *strikeEncounters) SaveEncounter(_ context.Context, id string, data *encounter.EncounterData) error {
	s.saves++
	if previous, ok := s.byID[id]; ok {
		before, after := strikeNextSeq(previous), strikeNextSeq(data)
		if after > before {
			s.records += int(after - before)
		}
	}
	s.byID[id] = data
	return nil
}

func strikeNextSeq(data *encounter.EncounterData) uint64 {
	if data == nil || data.Log.NextSeq == 0 {
		return 1
	}
	return data.Log.NextSeq
}

type strikeCharacters struct {
	byID  map[string]*character.Data
	saves int
}

func (s *strikeCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	data, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneFixture(data)
}

func (s *strikeCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	s.saves++
	s.byID[data.ID] = data
	return nil
}

func TestProjectCandidatesDefensivelyCopiesWhy(t *testing.T) {
	why := &Shortfall{Reason: ShortfallTargetOutOfReach, Text: "original"}
	projected := projectCandidates([]targetPreflight{{member: "bob", available: false, why: why}})
	require.Len(t, projected, 1)
	require.NotSame(t, why, projected[0].Why)
	projected[0].Why.Text = "mutated by caller"
	require.Equal(t, "original", why.Text)
}

// TestMoveRegenerationSkipsAttackTargetPreflight proves a current Move
// selector does not inherit Attack's target dependencies. The selector comes
// from a normal full Afford compilation; after that, Attack's shared target
// gate is made unreadable. Regenerating Move must neither ask that gate nor
// fail because the unrelated Attack can no longer compile.
func TestMoveRegenerationSkipsAttackTargetPreflight(t *testing.T) {
	ctx := context.Background()
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	characters := &strikeCharacters{byID: map[string]*character.Data{
		"alice": strikeFixtureFighter("alice"),
		"bob":   strikeFixtureFighter("bob"),
	}}
	mgr, err := NewManager(&Config{
		Dice: &scriptedDice{}, TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: DiscardEvents{},
	})
	require.NoError(t, err)

	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: aggregateRecordEveryoneSees{},
		Initiative: aggregateRecordOrderAsGiven{}, TurnDriver: passDriver{}, Standing: aggregateRecordEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	data := world.ToData()
	delete(data.Clock.Budgets, core.EntityID("alice"))
	delete(data.Clock.Budgets, core.EntityID("bob"))
	require.NoError(t, json.Unmarshal([]byte(`[{"order":["alice","bob"],"round":1}]`), &data.Bubbles))
	_, err = mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	require.NoError(t, err)

	afford, err := mgr.Afford(ctx, &AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	var move Declaration
	for _, declaration := range afford.Declarations {
		if declaration.Verb == VerbMove {
			move = declaration
			break
		}
	}
	require.True(t, move.Available)
	require.NotEmpty(t, move.ID)

	calls := 0
	mgr.targetPreflight = func(
		_ *encounter.Encounter, _ map[string]spatial.Position, _ []intel.Holding, _ string, _ int,
	) ([]targetPreflight, error) {
		calls++
		return nil, errors.New("injected Attack target preflight failure")
	}

	moved, err := mgr.Move(ctx, &MoveInput{
		Session: "sess", Member: "alice", DeclarationID: move.ID,
		Path: []spatial.Position{{X: 1, Y: 2}},
	})
	require.NoError(t, err)
	require.Len(t, moved.Steps, 1)
	require.Zero(t, calls, "Move regeneration must not read Attack target dependencies")
}

// TestInjectedTargetPreflightRefusalChangesAffordAndAttack proves projection
// and execution share one target gate. The injected refusal appears verbatim
// on Afford's candidate and makes the echoed Attack selector stale before any
// die rolls; an independent execution preflight would let the attack through.
func TestInjectedTargetPreflightRefusalChangesAffordAndAttack(t *testing.T) {
	ctx := context.Background()
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	characters := &strikeCharacters{byID: map[string]*character.Data{
		"alice": strikeFixtureFighter("alice"),
		"bob":   strikeFixtureFighter("bob"),
	}}
	roller := &scriptedDice{rolls: []int{17, 4}}
	mgr, err := NewManager(&Config{
		Dice: roller, TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: DiscardEvents{},
	})
	require.NoError(t, err)

	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: aggregateRecordEveryoneSees{},
		Initiative: aggregateRecordOrderAsGiven{}, TurnDriver: passDriver{}, Standing: aggregateRecordEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	data := world.ToData()
	delete(data.Clock.Budgets, core.EntityID("alice"))
	delete(data.Clock.Budgets, core.EntityID("bob"))
	require.NoError(t, json.Unmarshal(
		[]byte(`[{"order":["alice","bob"],"round":1}]`), &data.Bubbles,
	))
	require.NoError(t, func() error {
		_, startErr := mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &data})
		return startErr
	}())

	injected := Shortfall{Reason: ShortfallTargetOutOfReach, Text: "injected target refusal"}
	calls := 0
	mgr.targetPreflight = func(
		_ *encounter.Encounter, _ map[string]spatial.Position, _ []intel.Holding, member string, _ int,
	) ([]targetPreflight, error) {
		calls++
		require.Equal(t, "alice", member)
		why := injected
		return []targetPreflight{{member: "bob", available: false, why: &why}}, nil
	}

	afford, err := mgr.Afford(ctx, &AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	var attack Declaration
	for _, declaration := range afford.Declarations {
		if declaration.Verb == VerbAttack {
			attack = declaration
			break
		}
	}
	require.NotEmpty(t, attack.ID)
	require.False(t, attack.Available)
	require.Len(t, attack.Candidates, 1)
	require.Equal(t, injected, *attack.Candidates[0].Why)

	// Isolate the attempted Attack from setup and Afford. Counters prove no
	// repository or story mutation occurred; the state comparison separately
	// pins position and clock.
	sessions.saves, encounters.saves, encounters.records, characters.saves = 0, 0, 0, 0
	beforeState, err := json.Marshal(struct {
		Clock   any
		Bubbles any
		Members any
	}{encounters.byID["world"].Clock, encounters.byID["world"].Bubbles, encounters.byID["world"].Members})
	require.NoError(t, err)

	out, err := mgr.Attack(ctx, &AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: attack.ID,
	})
	require.ErrorIs(t, err, ErrStaleDeclaration)
	require.Nil(t, out)
	require.Equal(t, 2, calls, "Afford and regenerated Attack each use the shared seam")
	require.Zero(t, roller.next, "the injected refusal precedes every attack roll")
	require.Zero(t, characters.saves, "target preflight refusal writes no character")
	require.Zero(t, sessions.saves, "target preflight refusal writes no session")
	require.Zero(t, encounters.saves, "target preflight refusal writes no encounter")
	require.Zero(t, encounters.records, "target preflight refusal records no story beat")
	afterState, err := json.Marshal(struct {
		Clock   any
		Bubbles any
		Members any
	}{encounters.byID["world"].Clock, encounters.byID["world"].Bubbles, encounters.byID["world"].Members})
	require.NoError(t, err)
	require.JSONEq(t, string(beforeState), string(afterState), "target preflight refusal changes no position or clock")
}

func TestAttackVariantsShareOneTargetPreflight(t *testing.T) {
	ctx := context.Background()
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	alice := strikeFixtureFighter("alice")
	alice.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Scimitar), Quantity: 1},
	}
	alice.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Shortsword),
		character.SlotOffHand:  string(weapons.Scimitar),
	}
	alice.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 0, BonusActionsRemaining: 1,
		ReactionsRemaining: 1, MovementRemaining: 30,
		Granted: map[character.GrantedActionKey]int{character.GrantedOffHandStrikes: 1},
	}
	characters := &strikeCharacters{byID: map[string]*character.Data{
		"alice": alice,
		"bob":   strikeFixtureFighter("bob"),
	}}
	mgr, err := NewManager(&Config{
		Dice: &scriptedDice{}, TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: DiscardEvents{},
	})
	require.NoError(t, err)

	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: aggregateRecordEveryoneSees{},
		Initiative: aggregateRecordOrderAsGiven{}, TurnDriver: passDriver{}, Standing: aggregateRecordEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	data := world.ToData()
	delete(data.Clock.Budgets, core.EntityID("alice"))
	delete(data.Clock.Budgets, core.EntityID("bob"))
	require.NoError(t, json.Unmarshal([]byte(`[{"order":["alice","bob"],"round":1}]`), &data.Bubbles))
	_, err = mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	require.NoError(t, err)

	calls := 0
	mgr.targetPreflight = func(
		_ *encounter.Encounter, _ map[string]spatial.Position, _ []intel.Holding, _ string, _ int,
	) ([]targetPreflight, error) {
		calls++
		return []targetPreflight{{member: "bob", available: true}}, nil
	}

	afford, err := mgr.Afford(ctx, &AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	attacks := make([]Declaration, 0, 2)
	for _, declaration := range afford.Declarations {
		if declaration.Verb == VerbAttack {
			attacks = append(attacks, declaration)
		}
	}
	require.Len(t, attacks, 2)
	require.Equal(t, 1, calls, "all Attack variants share one target preflight snapshot")
	require.Equal(t, attacks[0].Candidates, attacks[1].Candidates)
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
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: aggregateRecordEveryoneSees{},
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
