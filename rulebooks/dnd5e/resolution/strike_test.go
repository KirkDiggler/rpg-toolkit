package resolution

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type actionRoller struct {
	singles []int
	pairs   [][]int
	damage  [][]int
	calls   int
}

func (r *actionRoller) Roll(_ context.Context, _ int) (int, error) {
	r.calls++
	if len(r.singles) == 0 {
		return 0, errors.New("no scripted single")
	}
	value := r.singles[0]
	r.singles = r.singles[1:]
	return value, nil
}

func (r *actionRoller) RollN(_ context.Context, count, sides int) ([]int, error) {
	r.calls++
	var queue *[][]int
	if sides == 20 && count == 2 {
		queue = &r.pairs
	} else {
		queue = &r.damage
	}
	if len(*queue) == 0 {
		return nil, errors.New("no scripted roll group")
	}
	values := (*queue)[0]
	*queue = (*queue)[1:]
	return values, nil
}

func actionWorld(t *testing.T, targetX float64) encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 30, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 1}},
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: targetX, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	return enc.ToData()
}

func actionHero() *character.Data {
	return &character.Data{
		ID: heroID, PlayerID: "player", Name: "Hero", Level: 1, ClassID: "fighter", RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 20, MaxHitPoints: 20, ArmorClass: 12, ProficiencyBonus: 2,
	}
}

func resolveActionDefinition(
	t *testing.T, definition combatActions.Definition, targetX float64, roller dice.Roller,
) (*Output, error) {
	machine, err := NewAction(&ActionInput{
		Definition: definition, AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	return Resolve(context.Background(), &Input{
		World: actionWorld(t, targetX),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: actionHero()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{}, Roller: dice.NewRoller(),
	})
}

func resolveActionDefinitionAgainstMonster(
	t *testing.T, definition combatActions.Definition, roller dice.Roller,
) (*Output, error) {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 1}},
			{ID: secondWolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	machine, err := NewAction(&ActionInput{
		Definition: definition, AttackerID: wolfID, TargetID: secondWolfID, Roller: roller,
	})
	require.NoError(t, err)
	return Resolve(context.Background(), &Input{
		World: enc.ToData(),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Monster: monsters.NewSkeleton(secondWolfID).ToData()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{}, Roller: dice.NewRoller(),
	})
}

func TestUnknownContentRefResolvesByProfile(t *testing.T) {
	definition := validMeleeDefinition()
	definition.Ref.ID = "unknown-claw"
	roller := &actionRoller{singles: []int{15}, damage: [][]int{{3}}}

	out, err := resolveActionDefinition(t, definition, 2, roller)

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.True(t, outcome.Hit)
	require.Equal(t, 5, outcome.Damage)
}

func TestDeliveryRangeAndLongRangeDisadvantage(t *testing.T) {
	t.Run("melee beyond reach refuses before dice", func(t *testing.T) {
		roller := &actionRoller{}
		_, err := resolveActionDefinition(t, validMeleeDefinition(), 3, roller)
		require.ErrorIs(t, err, ErrOutOfRange)
		require.Zero(t, roller.calls)
	})

	t.Run("ranged inside normal range rolls once", func(t *testing.T) {
		definition := validMeleeDefinition()
		definition.Attack.Delivery = combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 20, LongFeet: 40}}
		roller := &actionRoller{singles: []int{15}, damage: [][]int{{3}}}
		out, err := resolveActionDefinition(t, definition, 4, roller)
		require.NoError(t, err)
		outcome := out.Outcome.(StrikeOutcome)
		require.False(t, outcome.Folded.IsMelee)
		require.Empty(t, outcome.Folded.DisadvantageSources)
	})

	t.Run("ranged beyond long range refuses before dice", func(t *testing.T) {
		definition := validMeleeDefinition()
		definition.Attack.Delivery = combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 20, LongFeet: 40}}
		roller := &actionRoller{}
		_, err := resolveActionDefinition(t, definition, 10, roller)
		require.ErrorIs(t, err, ErrOutOfRange)
		require.Contains(t, err.Error(), "distance 9 cells exceeds maximum 8 cells (40 feet)")
		require.Zero(t, roller.calls)
	})

	t.Run("ranged long range takes lower die with attribution", func(t *testing.T) {
		definition := validMeleeDefinition()
		definition.Attack.Delivery = combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 20, LongFeet: 40}}
		roller := &actionRoller{pairs: [][]int{{17, 4}}, damage: [][]int{{3}}}
		out, err := resolveActionDefinition(t, definition, 7, roller)
		require.NoError(t, err)
		outcome := out.Outcome.(StrikeOutcome)
		require.Equal(t, 4, outcome.Roll)
		require.Len(t, outcome.Folded.DisadvantageSources, 1)
		require.Equal(t, definition.Ref, *outcome.Folded.DisadvantageSources[0].SourceRef)
	})
}

func TestConditionDeclarationsApplyInOrder(t *testing.T) {
	definition := validMeleeDefinition()
	definition.Attack.OnHit = []combatActions.ConditionApplication{
		{Ref: *refs.Conditions.Prone()},
		{Ref: *refs.Conditions.Dodging(), Save: saves.NewSaveGate(abilities.STR, 30)},
	}
	roller := &actionRoller{singles: []int{20, 2}, damage: [][]int{{3}, {2}}}

	out, err := resolveActionDefinition(t, definition, 2, roller)

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Len(t, outcome.Conditions, 2)
	require.Equal(t, refs.Conditions.Prone(), &outcome.Conditions[0].Ref)
	require.True(t, outcome.Conditions[0].Applied)
	require.Nil(t, outcome.Conditions[0].Contest)
	require.Equal(t, refs.Conditions.Dodging(), &outcome.Conditions[1].Ref)
	require.True(t, outcome.Conditions[1].Applied)
	require.NotNil(t, outcome.Conditions[1].Contest)
}

func TestConditionOnlyAttackSkipsDamageAndStillApplies(t *testing.T) {
	definition := validMeleeDefinition()
	definition.Attack.Damage = nil
	definition.Attack.OnHit = []combatActions.ConditionApplication{{Ref: *refs.Conditions.Prone()}}
	roller := &actionRoller{singles: []int{20}}

	out, err := resolveActionDefinition(t, definition, 2, roller)

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Zero(t, outcome.Damage)
	require.Len(t, outcome.Conditions, 1)
	require.True(t, outcome.Conditions[0].Applied)
}

func TestAutomaticConditionPersistsOnAMonsterTarget(t *testing.T) {
	definition := validMeleeDefinition()
	definition.Attack.Damage = nil
	definition.Attack.OnHit = []combatActions.ConditionApplication{{Ref: *refs.Conditions.Prone()}}

	out, err := resolveActionDefinitionAgainstMonster(t, definition, &actionRoller{singles: []int{20}})

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Len(t, outcome.Conditions, 1)
	require.True(t, outcome.Conditions[0].Applied)
	require.Len(t, out.DirtyMonsters, 1)
	require.Equal(t, secondWolfID, out.DirtyMonsters[0].ID)
	// startingConditions is what the CATALOG holds. Attach then grants every
	// combatant its free reactions on the way past — the opportunity attack,
	// which carries per-turn state and therefore persists with the sheet
	// (rpg-toolkit#1284) — so a write-back is the catalog's conditions, plus
	// that grant, plus the one this strike applied.
	//
	// Read from the END rather than by index. What this test is about is that
	// an automatically applied condition reached the stored sheet; how many
	// other things attach grants on the way is somebody else's invariant, and
	// indexing past them made this fail for a reason it does not care about.
	startingConditions := len(monsters.NewSkeleton(secondWolfID).ToData().Conditions)
	persisted := out.DirtyMonsters[0].Conditions
	require.Len(t, persisted, startingConditions+2)
	require.Contains(t, string(persisted[len(persisted)-1]), refs.Conditions.Prone().ID)
}

func TestSpellAttackDamageKeepsItsSpellSource(t *testing.T) {
	definition := validMeleeDefinition()
	definition.Attack.Category = combatActions.AttackCategorySpell

	out, err := resolveActionDefinition(t, definition, 2, &actionRoller{
		singles: []int{15}, damage: [][]int{{3}},
	})

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Len(t, outcome.DamageComponents, 1)
	require.Equal(t, dnd5eEvents.DamageSourceSpell, outcome.DamageComponents[0].Source)
}

func TestCancelledAttackStopsBeforeDiceAndDamage(t *testing.T) {
	definition := validMeleeDefinition()
	roller := &actionRoller{singles: []int{20}, damage: [][]int{{6}}}
	bus := events.NewEventBus()
	_, err := dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(context.Background(),
		func(_ context.Context, _ dnd5eEvents.AttackChainEvent,
			c chain.Chain[dnd5eEvents.AttackChainEvent],
		) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
			cancel := func(_ context.Context, event dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
				event.CancellationSources = append(event.CancellationSources, dnd5eEvents.AttackModifierSource{
					SourceRef: &definition.Ref,
					SourceID:  heroID,
					Reason:    "test cancellation",
				})
				return event, nil
			}
			return c, c.Add(combat.StageConditions, "cancel_test_attack", cancel)
		})
	require.NoError(t, err)
	machine, err := NewAction(&ActionInput{
		Definition: definition, AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)

	out, err := resolveOn(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: actionHero()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{}, Roller: dice.NewRoller(),
	}, newSurface(bus))

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Len(t, outcome.Folded.CancellationSources, 1)
	require.False(t, outcome.Hit)
	require.Zero(t, outcome.Damage)
	require.Zero(t, roller.calls)
	require.Empty(t, out.DirtyCharacters)
	require.Empty(t, out.DirtyMonsters)
}

func TestStrikeAtZeroUpdatesAuthoritativeDeathSaveProgress(t *testing.T) {
	tests := []struct {
		name     string
		state    *saves.DeathSaveState
		roller   *actionRoller
		critical bool
		rolls    []int
		want     *saves.DeathSaveState
	}{
		{
			name:   "normal damage adds one failure",
			state:  &saves.DeathSaveState{Successes: 1},
			roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}},
			rolls:  []int{3},
			want:   &saves.DeathSaveState{Successes: 1, Failures: 1},
		},
		{
			name:     "critical damage adds two failures",
			state:    &saves.DeathSaveState{},
			roller:   &actionRoller{singles: []int{20}, damage: [][]int{{3}, {4}}},
			critical: true,
			rolls:    []int{3, 4},
			want:     &saves.DeathSaveState{Failures: 2},
		},
		{
			name:   "damage removes stabilization before adding a failure",
			state:  &saves.DeathSaveState{Successes: 3, Stabilized: true},
			roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}},
			rolls:  []int{3},
			want:   &saves.DeathSaveState{Successes: 3, Failures: 1},
		},
		{
			name:   "the third failure makes the character dead",
			state:  &saves.DeathSaveState{Failures: 2},
			roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}},
			rolls:  []int{3},
			want:   &saves.DeathSaveState{Failures: 3, Dead: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			target := deathSaveStrikeTarget(tc.state)
			original := *target.DeathSaveState
			definition := validMeleeDefinition()

			out, err := resolveStrikeAgainstDeathSaveTarget(t, target, definition, tc.roller, nil)

			require.NoError(t, err)
			struck := out.Outcome.(StrikeOutcome)
			require.True(t, struck.Hit)
			require.Positive(t, struck.Damage)
			require.Equal(t, tc.critical, struck.Critical)
			require.Len(t, struck.DamageComponents, 1)
			trace := struck.DamageComponents[0].Roll
			require.NotNil(t, trace.Source.Ref)
			require.Equal(t, definition.Ref, *trace.Source.Ref)
			require.Equal(t, definition.Name, trace.Source.Name)
			require.NotNil(t, trace.Dice)
			require.Equal(t, tc.rolls, trace.Dice.FinalRolls,
				"death-save damage keeps #1474's sourced roll trace")
			require.Len(t, out.DirtyCharacters, 1,
				"the authoritative death-save transition must be returned for persistence")
			require.Equal(t, heroID, out.DirtyCharacters[0].ID)
			require.Equal(t, tc.want, out.DirtyCharacters[0].DeathSaveState)
			require.Equal(t, original, *target.DeathSaveState,
				"the persisted input remains caller-owned")
		})
	}
}

func TestStrikeAtAnAlreadyDeadCharacterIsACompleteNoOp(t *testing.T) {
	state := &saves.DeathSaveState{Successes: 2, Failures: 3, Dead: true}
	target := deathSaveStrikeTarget(state)
	before := *target.DeathSaveState

	out, err := resolveStrikeAgainstDeathSaveTarget(t, target, validMeleeDefinition(),
		&actionRoller{singles: []int{15}, damage: [][]int{{3}}}, nil)

	require.NoError(t, err)
	struck := out.Outcome.(StrikeOutcome)
	require.True(t, struck.Hit)
	require.Positive(t, struck.Damage, "the no-op is the Dead life state, not an attack that failed to land")
	require.Empty(t, out.DirtyCharacters, "nothing about the already-Dead sheet changed")
	require.Equal(t, before, *target.DeathSaveState)
}

// Neither regression installs or observes a DamageReceivedEvent subscriber.
// The direct Strike -> Character.ApplyDamage path owns the death-save decision:
// only positive applied damage may change progress.
func TestStrikeWithNoAppliedDamageLeavesDeathSaveProgressUnchanged(t *testing.T) {
	t.Run("zero final damage", func(t *testing.T) {
		definition := validMeleeDefinition()
		definition.Attack.Damage[0].FlatBonus = -1
		target := deathSaveStrikeTarget(&saves.DeathSaveState{Successes: 1, Failures: 1})
		before := *target.DeathSaveState

		out, err := resolveStrikeAgainstDeathSaveTarget(t, target, definition,
			&actionRoller{singles: []int{15}, damage: [][]int{{1}}}, nil)

		require.NoError(t, err)
		struck := out.Outcome.(StrikeOutcome)
		require.True(t, struck.Hit)
		require.Zero(t, struck.Damage)
		require.Len(t, struck.DamageComponents, 1)
		require.NotNil(t, struck.DamageComponents[0].Roll.Dice)
		require.Equal(t, 1, struck.DamageComponents[0].Roll.Dice.Subtotal)
		require.NotNil(t, struck.DamageComponents[0].Roll.Modifier)
		require.Equal(t, -1, *struck.DamageComponents[0].Roll.Modifier)
		require.Empty(t, out.DirtyCharacters)
		require.Equal(t, before, *target.DeathSaveState)
	})

	t.Run("full immunity", func(t *testing.T) {
		bus := events.NewEventBus()
		immunityRef := &core.Ref{Module: "test", Type: "conditions", ID: "full-immunity"}
		_, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(context.Background(),
			func(_ context.Context, _ *dnd5eEvents.DamageChainEvent,
				c chain.Chain[*dnd5eEvents.DamageChainEvent],
			) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
				return c, c.Add(combat.StageFinal, "test_full_immunity",
					func(_ context.Context, event *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
						event.Components = append(event.Components, dnd5eEvents.DamageComponent{
							Source: dnd5eEvents.DamageSourceCondition,
							Roll: dnd5eEvents.RollComponent{Source: dnd5eEvents.RollSource{
								Ref: immunityRef, Name: "Full Immunity",
							}},
							Multiplier: dnd5eEvents.Multiply(0),
							DamageType: damage.Slashing,
						})
						return event, nil
					})
			})
		require.NoError(t, err)
		target := deathSaveStrikeTarget(&saves.DeathSaveState{Successes: 1, Failures: 1})
		before := *target.DeathSaveState

		out, err := resolveStrikeAgainstDeathSaveTarget(t, target, validMeleeDefinition(),
			&actionRoller{singles: []int{15}, damage: [][]int{{3}}}, bus)

		require.NoError(t, err)
		struck := out.Outcome.(StrikeOutcome)
		require.True(t, struck.Hit)
		require.Zero(t, struck.Damage)
		require.Len(t, struck.DamageComponents, 2)
		require.Equal(t, immunityRef, struck.DamageComponents[1].Roll.Source.Ref)
		require.NotSame(t, immunityRef, struck.DamageComponents[1].Roll.Source.Ref,
			"the #1474 outcome owns its sourced trace")
		require.Equal(t, "Full Immunity", struck.DamageComponents[1].Roll.Source.Name)
		require.Empty(t, out.DirtyCharacters)
		require.Equal(t, before, *target.DeathSaveState)
	})
}

func deathSaveStrikeTarget(state *saves.DeathSaveState) *character.Data {
	target := actionHero()
	target.HitPoints = 0
	target.DeathSaveState = state
	return target
}

func resolveStrikeAgainstDeathSaveTarget(
	t *testing.T,
	target *character.Data,
	definition combatActions.Definition,
	roller dice.Roller,
	bus events.EventBus,
) (*Output, error) {
	t.Helper()
	in := &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: target},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Definition: definition,
			Roller:     roller,
		}),
		Initiative: orderAsGiven{},
		Standing:   everyoneStanding{},
		Sight:      everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{},
		Roller:     dice.NewRoller(),
	}
	if bus != nil {
		return resolveOn(context.Background(), in, newSurface(bus))
	}
	return Resolve(context.Background(), in)
}
