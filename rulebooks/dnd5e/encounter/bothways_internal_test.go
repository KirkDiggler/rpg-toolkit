// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// bothways_internal_test.go is THE ORDER THE AGGRESSION LAW RUNS IN
// (rpg-project#493, R3), which is the one claim about it that cannot be made
// from outside: the pair has to turn BEFORE the deed lands, or the very pick
// the swing provokes reads `enemy: none`. And it is the same claim for the
// other deliveries R5 binds the law to — a spell that asks for a save, and
// one that lands its harm with no gate at all.
//
// A creature's own facts are the projection its table is picked against
// ([Encounter.factsFor]), and they are unexported because a host never asks
// for them — a Driver is handed a view and nothing else. Reading them here is
// reading exactly what the goblin's `time` table will read a moment later.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestTheProvokedCampReadsAnEnemyAndADeedInOnePick is the whole reason
// [Encounter.aggression] sits ahead of the deed rather than behind it.
//
// The goblin's shipped table has two rows that matter here — "attacked within
// 3 -> attack: attacker" and "enemy: reach -> attack: enemy" — and only the
// first of them used to hold after a party attacked a neutral camp. One
// aggrieved goblin, no initiative and no friends, is what the design's
// opening paragraph describes; both facts being true in the same projection
// is what replaces it.
func TestTheProvokedCampReadsAnEnemyAndADeedInOnePick(t *testing.T) {
	const camp = "goblins"
	scimitar := ActionView{
		Ref:  core.Ref{Module: "dnd5e", Type: "weapons", ID: "scimitar"},
		Name: "Scimitar", RangeFeet: 5, Kind: "melee",
	}

	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:   CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions:  []RegionInput{rectRegion("yard", 0, 0, 6, 6)},
			Factions: []FactionInput{{ID: camp}},
			Dispositions: []DispositionInput{{
				Between: [2]FactionID{camp, FactionParty}, Stance: StanceNeutral,
			}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30, SightFeet: 60},
			{
				ID: "watcher", Kind: KindMonster, Faction: camp, Position: spatial.Position{X: 2, Y: 1},
				SpeedFeet: 30, SightFeet: 60, Actions: []ActionView{scimitar},
			},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	before, err := enc.factsFor("watcher")
	require.NoError(t, err)
	require.False(t, before.EnemyInReach, "precondition: a civil camp reads no enemy at all")
	require.False(t, before.EnemySeen)
	require.Empty(t, before.Deeds, "precondition: nothing has been done to it")

	_, err = enc.Record(&RecordInput{
		Kind: OutcomeStruck, Actor: "alice", Targets: []MemberID{"watcher"},
		Values: map[OutcomeValue]int{ValueAmount: 7},
	})
	require.NoError(t, err)

	after, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.True(t, after.EnemyInReach,
		"the pair turned before the deed landed, so the row that says `enemy: reach` can fire")

	var attacked bool
	for _, held := range after.Deeds {
		if held.Kind == DeedAttack && held.Actor == "alice" {
			attacked = true
		}
	}
	require.True(t, attacked, "and the deed the swing landed is held against alice by name")
}

// TestTheMockedWatcherReadsAnEnemyAndADeedInOnePick is the review's probe at
// the grain only this package can see (rpg-project#493, R5): a cantrip that
// delivered through a SAVE, and the watcher's own projection afterwards.
//
// BOTH ROWS OR NEITHER. The goblin's shipped table reads "attacked within 3
// -> attack: attacker" and "enemy: reach -> attack: enemy", and before R5 a
// failed-save Vicious Mockery fired NEITHER: the deed never landed, so the
// victim did not even know it had been hurt, and the camp read `enemy: none`.
// This is the swing's own claim made of the spell.
func TestTheMockedWatcherReadsAnEnemyAndADeedInOnePick(t *testing.T) {
	const camp = "goblins"
	mockery := SpellIdentity{Ref: "dnd5e:spells:vicious-mockery", Name: "Vicious Mockery"}
	scimitar := ActionView{
		Ref:  core.Ref{Module: "dnd5e", Type: "weapons", ID: "scimitar"},
		Name: "Scimitar", RangeFeet: 5, Kind: "melee",
	}

	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:   CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions:  []RegionInput{rectRegion("yard", 0, 0, 6, 6)},
			Factions: []FactionInput{{ID: camp}},
			Dispositions: []DispositionInput{{
				Between: [2]FactionID{camp, FactionParty}, Stance: StanceNeutral,
			}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30, SightFeet: 60},
			{
				ID: "watcher", Kind: KindMonster, Faction: camp, Position: spatial.Position{X: 2, Y: 1},
				SpeedFeet: 30, SightFeet: 60, Actions: []ActionView{scimitar},
			},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	before, err := enc.factsFor("watcher")
	require.NoError(t, err)
	require.False(t, before.EnemyInReach, "precondition: a civil camp reads no enemy at all")
	require.Empty(t, before.Deeds, "precondition: nothing has been done to it")

	_, err = enc.RecordCast(&RecordCastInput{
		Actor: "alice", Spell: mockery,
		Targets: []CastTargetResult{{
			Target: "watcher",
			Save: &CastSave{
				Saver: "watcher", Ability: "wisdom", Roll: 6, Total: 8, DC: 13, Succeeded: false,
				Calculation: &RollCalculation{
					Components: []RollComponent{{
						Source: RollSource{Ref: mockery.Ref, Name: mockery.Name, SourceID: "alice"},
						Dice: &DiceTrace{
							Notation: "1d20", DieSize: 20,
							OriginalRolls: []int{6}, FinalRolls: []int{6}, Subtotal: 6,
						},
					}, {
						Source:   RollSource{Ref: "dnd5e:abilities:wisdom", Name: "wisdom"},
						Modifier: func() *int { m := 2; return &m }(),
					}},
					Total: 8,
				},
			},
			Results: []ActivationResult{{
				Kind: ResultDamageApplied, Target: "watcher",
				Ref: mockery.Ref, Name: mockery.Name,
				Amount: 3, Requested: 3, Before: 7, After: 4, DamageType: "psychic",
				Calculation: &RollCalculation{
					Components: []RollComponent{{
						Source: RollSource{Ref: mockery.Ref, Name: mockery.Name, SourceID: "alice"},
						Dice: &DiceTrace{
							Notation: "1d4", DieSize: 4,
							OriginalRolls: []int{3}, FinalRolls: []int{3}, Subtotal: 3,
						},
					}},
					Total: 3,
				},
			}},
		}},
	})
	require.NoError(t, err)

	after, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.True(t, after.EnemyInReach,
		"the pair turned before the deed landed, so the row that says `enemy: reach` can fire")

	var attacked bool
	for _, held := range after.Deeds {
		if held.Kind == DeedAttack && held.Actor == "alice" {
			attacked = true
		}
	}
	require.True(t, attacked, "and the spell landed the same deed a sword would, against alice by name")
}

// TestTheMissiledWatcherReadsAnEnemyAndADeedInOnePick is the third delivery
// door at the grain only this package can see (rpg-project#493, R5 as the
// review of #1868 amended it): a cast with NO attack roll and NO save, which
// simply lands nine force damage.
//
// THE PROBE THAT FOUND THE HOLE. A predicate that answered by arm read this
// as nothing having happened — no turn, no deed, the camp still civil while
// its scout bled — and contradicted itself one branch over, where a save that
// delivered nothing provoked. Magic missile and `sleep` are both in this
// toolkit's own catalog, so the shape arrives the day either is wired.
func TestTheMissiledWatcherReadsAnEnemyAndADeedInOnePick(t *testing.T) {
	const camp = "goblins"
	missile := SpellIdentity{Ref: "dnd5e:spells:magic-missile", Name: "Magic Missile"}
	scimitar := ActionView{
		Ref:  core.Ref{Module: "dnd5e", Type: "weapons", ID: "scimitar"},
		Name: "Scimitar", RangeFeet: 5, Kind: "melee",
	}

	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:   CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions:  []RegionInput{rectRegion("yard", 0, 0, 6, 6)},
			Factions: []FactionInput{{ID: camp}},
			Dispositions: []DispositionInput{{
				Between: [2]FactionID{camp, FactionParty}, Stance: StanceNeutral,
			}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30, SightFeet: 60},
			{
				ID: "watcher", Kind: KindMonster, Faction: camp, Position: spatial.Position{X: 2, Y: 1},
				SpeedFeet: 30, SightFeet: 60, Actions: []ActionView{scimitar},
			},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	before, err := enc.factsFor("watcher")
	require.NoError(t, err)
	require.False(t, before.EnemyInReach, "precondition: a civil camp reads no enemy at all")
	require.Empty(t, before.Deeds, "precondition: nothing has been done to it")

	_, err = enc.RecordCast(&RecordCastInput{
		Actor: "alice", Spell: missile,
		Targets: []CastTargetResult{{
			Target: "watcher",
			Results: []ActivationResult{{
				Kind: ResultDamageApplied, Target: "watcher",
				Ref: missile.Ref, Name: missile.Name,
				Amount: 9, Requested: 9, Before: 12, After: 3, DamageType: "force",
				Calculation: &RollCalculation{
					Components: []RollComponent{{
						Source: RollSource{Ref: missile.Ref, Name: missile.Name, SourceID: "alice"},
						Dice: &DiceTrace{
							Notation: "3d4", DieSize: 4,
							OriginalRolls: []int{2, 2, 2}, FinalRolls: []int{2, 2, 2}, Subtotal: 6,
						},
					}, {
						Source:   RollSource{Ref: missile.Ref, Name: missile.Name, SourceID: "alice"},
						Modifier: func() *int { m := 3; return &m }(),
					}},
					Total: 9,
				},
			}},
		}},
	})
	require.NoError(t, err)

	after, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.True(t, after.EnemyInReach,
		"the pair turned before the deed landed, so the row that says `enemy: reach` can fire")

	var attacked bool
	for _, held := range after.Deeds {
		if held.Kind == DeedAttack && held.Actor == "alice" {
			attacked = true
		}
	}
	require.True(t, attacked, "and an ungated spell landed the same deed a sword would, against alice by name")
}
