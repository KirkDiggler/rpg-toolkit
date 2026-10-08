// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// clericWarder is a level-1 Cleric with a real spell save DC of 13 — not the
// [wardDC] the fixture wards carry — and the caster named by those wards.
// Never placed on the encounter map, and never read by the ward check: a ward
// owns its DC, so the cleric's presence changes nothing there.
func clericWarder() *character.Data {
	return &character.Data{
		ID: "cleric-1", PlayerID: "player-2", Name: "Warder", Level: 1, ClassID: "cleric", RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 10, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 16, abilities.CHA: 10,
		},
		HitPoints: 10, MaxHitPoints: 10, ProficiencyBonus: 2,
	}
}

// wardDC is the DC every fixture ward carries. It is deliberately NOT
// clericWarder's own spell save DC (13): a ward check that read the caster's
// sheet instead of the ward would answer 13, and the tests could not tell the
// two apart. TestAWardKeepsItsDCAfterItsCasterLeaves pins the cast writing
// the caster's real DC.
const wardDC = 15

func sanctuaryJSON(t *testing.T, memberID string) json.RawMessage {
	t.Helper()
	ward, err := conditions.NewSanctuaryCondition(conditions.NewSanctuaryConditionInput{
		MemberID: memberID, SourceID: "cleric-1", SourceRef: refs.Spells.Sanctuary(),
		SaveDC: wardDC,
	})
	require.NoError(t, err)
	raw, err := ward.ToJSON()
	require.NoError(t, err)
	return raw
}

func sanctuaryImmuneJSON(t *testing.T, memberID string) json.RawMessage {
	t.Helper()
	immune, err := conditions.NewSanctuaryImmuneCondition(conditions.NewSanctuaryImmuneConditionInput{
		MemberID: memberID, SourceID: "cleric-1", SourceRef: refs.Spells.Sanctuary(),
	})
	require.NoError(t, err)
	raw, err := immune.ToJSON()
	require.NoError(t, err)
	return raw
}

func hasConditionRef(t *testing.T, blobs []json.RawMessage, ref string) bool {
	t.Helper()
	for _, blob := range blobs {
		loaded, err := conditions.LoadJSON(blob)
		require.NoError(t, err)
		if loaded.Ref().String() == ref {
			return true
		}
	}
	return false
}

func TestSanctuaryBlocksAnAttackOnAFailedWardSave(t *testing.T) {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}

	roller := &actionRoller{singles: []int{5}} // wolf's WIS save: low, fails against the cleric's DC
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: target}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
		Sheets:    noSheetsAsked{},
	})
	require.NoError(t, err)

	outcome := out.Outcome.(StrikeOutcome)
	require.NotNil(t, outcome.Warded)
	require.Equal(t, "cleric-1", outcome.Warded.SourceID)
	require.False(t, outcome.Warded.Save.Success)
	require.Zero(t, outcome.Roll, "no attack roll happened")
	require.Zero(t, outcome.Damage, "no damage happened")
	require.Equal(t, 1, roller.calls, "only the ward save was ever rolled")
}

func TestSanctuarySaveSuccessDoesNotGrantAttackerImmunity(t *testing.T) {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}

	// Ward save rolls high and passes; the attack roll and damage that follow
	// are the same scripted values TestBaneAttackUsesOneSelectedContributionAndRecordsCalculation uses.
	roller := &actionRoller{singles: []int{20, 15}, damage: [][]int{{4}}}
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: target}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
		Sheets:    noSheetsAsked{},
	})
	require.NoError(t, err)

	outcome := out.Outcome.(StrikeOutcome)
	require.Nil(t, outcome.Warded)
	require.NotZero(t, outcome.Roll, "the attack proceeded exactly as if there were no ward")

	for _, dirty := range out.DirtyMonsters {
		require.False(t, hasConditionRef(t, dirty.Conditions, refs.Conditions.SanctuaryImmune().String()))
	}
}

func TestRecipientCooldownDoesNotSkipWardSaves(t *testing.T) {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}
	attacker := monsters.NewWolf(wolfID).ToData()
	attacker.Conditions = []json.RawMessage{sanctuaryImmuneJSON(t, wolfID)}

	// A recipient cooldown must not bypass the aggressor's ward save.
	roller := &actionRoller{singles: []int{20, 15}, damage: [][]int{{4}}}
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: attacker}, {Character: target}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
		Sheets:    noSheetsAsked{},
	})
	require.NoError(t, err)

	outcome := out.Outcome.(StrikeOutcome)
	require.Nil(t, outcome.Warded)
	require.NotZero(t, outcome.Roll)
	require.Equal(t, 3, roller.calls, "ward save, attack and damage all roll")
}

func TestAttackingEndsTheAttackersOwnSanctuary(t *testing.T) {
	attacker := monsters.NewWolf(wolfID).ToData()
	attacker.Conditions = []json.RawMessage{sanctuaryJSON(t, wolfID)}

	roller := &actionRoller{singles: []int{15}, damage: [][]int{{4}}}
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: attacker}, {Character: actionHero()}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
		Sheets:    noSheetsAsked{},
	})
	require.NoError(t, err)

	require.Len(t, out.DirtyMonsters, 1)
	require.False(t,
		hasConditionRef(t, out.DirtyMonsters[0].Conditions, refs.Conditions.Sanctuary().String()),
		"the wolf's own Sanctuary ended the moment it attacked",
	)
}

// castFixtures is [CastActionTestSuite.fixtures]'s own trick, used directly
// here rather than inside a suite: ContestDamageTestSuite's world (bard,
// hero, wolf) and monster fixture are reused rather than rebuilt, and
// SetT lets its suite.Require() calls work outside suite.Run.
func castFixtures(t *testing.T) *ContestDamageTestSuite {
	t.Helper()
	f := &ContestDamageTestSuite{}
	f.SetT(t)
	f.ctx = context.Background()
	return f
}

func TestSanctuaryBlocksABaneTargetOnAFailedWardSave(t *testing.T) {
	fixtures := castFixtures(t)
	wolf := fixtures.wolfData()
	wolf.Conditions = []json.RawMessage{sanctuaryJSON(t, wolfID)}

	roller := &actionRoller{singles: []int{5}} // bard's WIS ward save: fails
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		World: fixtures.world(),
		Participants: []Participant{
			{Monster: wolf}, {Character: baneCaster(1, 2)}, {Character: clericWarder()},
		},
		Machine: machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	outcome := out.Outcome.(CastOutcome)
	require.Len(t, outcome.Targets, 1)
	require.NotNil(t, outcome.Targets[0].Warded)
	require.Equal(t, "cleric-1", outcome.Targets[0].Warded.SourceID)
	require.Nil(t, outcome.Targets[0].Save, "Bane's own contest never ran")
	require.Empty(t, outcome.Targets[0].Applied)
	require.Equal(t, 1, roller.calls, "only the ward save was ever rolled")

	payer := fixtures.sheet(out, bardID)
	require.Zero(t, payer.ActionEconomy.ActionsRemaining, "the action is spent regardless of the ward")
	require.Equal(t, 1, payer.Resources[resources.SpellSlotLevel1].Current, "and so is the slot")
}

func TestSanctuarySaveSuccessDoesNotGrantCasterImmunity(t *testing.T) {
	fixtures := castFixtures(t)
	wolf := fixtures.wolfData()
	wolf.Conditions = []json.RawMessage{sanctuaryJSON(t, wolfID)}

	roller := &actionRoller{singles: []int{20, 10}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		World: fixtures.world(),
		Participants: []Participant{
			{Monster: wolf}, {Character: baneCaster(1, 2)}, {Character: clericWarder()},
		},
		Machine: machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	outcome := out.Outcome.(CastOutcome)
	require.Len(t, outcome.Targets, 1)
	require.Nil(t, outcome.Targets[0].Warded)
	require.NotNil(t, outcome.Targets[0].Save, "Bane's own contest ran exactly as if there were no ward")

	require.False(t,
		hasConditionRef(t, fixtures.sheet(out, bardID).Conditions, refs.Conditions.SanctuaryImmune().String()),
		"a successful ward save must not grant the hostile caster the recipient's cooldown",
	)
}

func TestCastingABaneEndsTheCastersOwnSanctuary(t *testing.T) {
	fixtures := castFixtures(t)
	caster := baneCaster(1, 2)
	caster.Conditions = []json.RawMessage{sanctuaryJSON(t, bardID)}

	roller := &actionRoller{singles: []int{10}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		World:        fixtures.world(),
		Participants: []Participant{{Monster: fixtures.wolfData()}, {Character: caster}},
		Machine:      machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	require.False(t,
		hasConditionRef(t, fixtures.sheet(out, bardID).Conditions, refs.Conditions.Sanctuary().String()),
		"the bard's own Sanctuary ended the moment it cast a hostile spell",
	)
}

func TestSanctuaryDoesNotGateANonHostileCast(t *testing.T) {
	fixtures := castFixtures(t)
	saver := fixtures.saver(14)
	saver.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}

	roller := &actionRoller{singles: []int{10}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		World: fixtures.world(),
		Participants: []Participant{
			{Character: saver}, {Monster: fixtures.wolfData()}, {Character: baneCaster(1, 2)},
		},
		Machine: machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	outcome := out.Outcome.(CastOutcome)
	require.Len(t, outcome.Targets, 1)
	require.Nil(t, outcome.Targets[0].Warded, "bard and hero share a faction: never hostile, never gated")
	require.NotNil(t, outcome.Targets[0].Save)
	require.Equal(t, 1, roller.calls, "just Bane's own save — no ward save attempted, and Bane deals no damage")
}

// rosterCast answers stances the way the encounter does, from a fixed table
// over a fixed roster: a member pair with no entry is no side, and a pair
// naming a non-member is refused. Every other Cast question is outside these
// tests and panics if asked.
type rosterCast struct {
	gamectx.Cast
	members []string
	stances map[[2]string]contributions.Stance
}

func (c rosterCast) stanceAnswer(a, b string) (contributions.Stance, error) {
	if !slices.Contains(c.members, a) || !slices.Contains(c.members, b) {
		return "", encounter.ErrNotMember
	}
	if stance, ok := c.stances[[2]string{a, b}]; ok {
		return stance, nil
	}
	return contributions.StanceNone, nil
}

// A harmful cast's ward gate decides from the authoritative stance and fails
// the cast when it cannot (R13): a stance it cannot answer, or no cast to ask,
// is an error, never a skipped ward.
func TestCastWardGateFailsClosedOnAnUnknownStance(t *testing.T) {
	cast := rosterCast{
		members: []string{bardID, heroID, "shopkeeper"},
		stances: map[[2]string]contributions.Stance{{bardID, heroID}: contributions.StanceHostile},
	}
	hostile, err := castStanceIsHostile(cast, bardID, heroID)
	require.NoError(t, err)
	require.True(t, hostile, "a hostile stance reaches the ward")

	hostile, err = castStanceIsHostile(cast, bardID, "shopkeeper")
	require.NoError(t, err)
	require.False(t, hostile, "a member of no faction is a known no side, never warded against")

	_, err = castStanceIsHostile(cast, bardID, "stranger")
	require.ErrorIs(t, err, contributions.ErrRuleCannotAnswer,
		"a target the cast does not hold has no stance to read: the cast fails")

	_, err = castIsHostile(context.Background(), bardID, heroID)
	require.ErrorIs(t, err, ErrBadWorld, "no cast installed is no answer, not a skipped ward")
}

// runSanctuaryGate steps a harmful cast's ward gate for heroID, cast by
// bardID, under ctx — the gate itself, the step the cast path inserts
// before the target's own machine. Resolve cannot produce an unknown stance
// today, so the gate is stepped directly.
func runSanctuaryGate(ctx context.Context) (Step, error) {
	m := &castMachine{spell: *refs.Spells.Bane(), casterID: bardID, cast: &Participants{order: []string{bardID, heroID}}}
	gate, ok := m.sanctuaryGate(castTargetMachine{targetID: heroID}, 0).(Gather)
	if !ok {
		return nil, errors.New("the sanctuary gate is not a Gather step")
	}
	return gate.run(ctx, events.NewEventBus())
}

// The ward gate fails the cast when the caster→target stance cannot be
// decided (R13): a cast installed with no run to ask, or no cast at all.
func TestSanctuaryGateFailsTheCastOnAnUnknownStance(t *testing.T) {
	cast := &Participants{order: []string{bardID, heroID}}

	next, err := runSanctuaryGate(installTruth(context.Background(), nil, cast, nil))
	require.ErrorIs(t, err, contributions.ErrRuleCannotAnswer, "no run answered the stance: the cast fails")
	require.Nil(t, next, "the cast does not go on to its target")

	next, err = runSanctuaryGate(context.Background())
	require.ErrorIs(t, err, ErrBadWorld, "no cast installed: the cast fails")
	require.Nil(t, next)
}

// A cast with no run loaded proves nothing about sides: every pair is an
// unknown stance, never the known no side.
func TestAuthoritativeStanceWithNoRunIsUnknown(t *testing.T) {
	view := &castView{cast: &Participants{order: []string{bardID, heroID}}}
	require.Equal(t, contributions.Unknown[contributions.Stance](), authoritativeStance(view, bardID, heroID))

	_, err := castStanceIsHostile(view, bardID, heroID)
	require.ErrorIs(t, err, contributions.ErrRuleCannotAnswer)
}

// strikeOnWardedHero resolves the wolf's bite on a hero warded by cleric-1,
// with the given extra participants beside the two combatants.
func strikeOnWardedHero(t *testing.T, roller *actionRoller, extra ...Participant) (*Output, error) {
	t.Helper()
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}
	return strikeOn(t, target, roller, extra...)
}

// strikeOn resolves the wolf's bite on the given hero sheet.
func strikeOn(t *testing.T, target *character.Data, roller *actionRoller, extra ...Participant) (*Output, error) {
	t.Helper()
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	return Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: append([]Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: target},
		}, extra...),
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
		Sheets:    noSheetsAsked{},
	})
}

// castSanctuaryOnHero resolves the bard casting Sanctuary on the hero and
// hands back the hero's sheet as the cast left it.
func castSanctuaryOnHero(t *testing.T, caster *character.Data) (*character.Data, error) {
	t.Helper()
	fixtures := castFixtures(t)
	caster.ActionEconomy.BonusActionsRemaining = 1
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Sanctuary})
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: bardID, TargetIDs: []string{heroID},
		Roller: &countingCastRoller{},
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: touchWorld(t), Machine: machine,
		Participants: []Participant{{Character: caster}, {Character: actionHero()}},
		Cost:         &Cost{PayerID: bardID, Profile: definition.Cost, SpellTurn: "first"},
		Initiative:   orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Roller: dice.NewRoller(),
	})
	if err != nil {
		return nil, err
	}
	return fixtures.sheet(out, heroID), nil
}

// touchWorld stands the bard beside the hero, so a touch spell reaches.
func touchWorld(t *testing.T) encounter.EncounterData {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Mover: encounter.RefusingMover{}, Announcer: quietAnnouncer{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: bardID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	return enc.ToData()
}

// THE RULING. The ward owns the DC it was cast with. A cleric (here the bard,
// CHA +3, proficiency 2: DC 13) casts Sanctuary on the hero, then leaves the
// interaction; a strike at the hero still rolls the Wisdom save at DC 13.
// Before, the ward looked its caster up and, with the caster gone, every
// strike at the warded ally refused — a stalled table.
func TestAWardKeepsItsDCAfterItsCasterLeaves(t *testing.T) {
	warded, err := castSanctuaryOnHero(t, baneCaster(1, 2))
	require.NoError(t, err)

	var dc int
	for _, blob := range warded.Conditions {
		loaded, loadErr := conditions.LoadJSON(blob)
		require.NoError(t, loadErr)
		if ward, ok := loaded.(*conditions.SanctuaryCondition); ok {
			dc = ward.SaveDC
		}
	}
	require.Equal(t, 13, dc, "the cast wrote the caster's spell save DC onto the ward")

	roller := &actionRoller{singles: []int{5}} // wolf's WIS save: fails against 13
	out, err := strikeOn(t, warded, roller)    // the caster is not in this cast
	require.NoError(t, err, "a ward whose caster left still wards")

	outcome := out.Outcome.(StrikeOutcome)
	require.NotNil(t, outcome.Warded)
	require.Equal(t, bardID, outcome.Warded.SourceID)
	require.Equal(t, 13, outcome.Warded.Save.DC, "at the DC the ward was cast with")
	require.Equal(t, 1, roller.calls, "only the ward save was rolled")
}

// The ward reads its own DC, never the caster's sheet, so the same ward holds
// with its caster present or absent.
func TestTheWardDCIsTheOneItCarries(t *testing.T) {
	for name, extra := range map[string][]Participant{
		"caster present": {{Character: clericWarder()}},
		"caster absent":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := strikeOnWardedHero(t, &actionRoller{singles: []int{5}}, extra...)
			require.NoError(t, err)

			outcome := out.Outcome.(StrikeOutcome)
			require.NotNil(t, outcome.Warded)
			require.Equal(t, wardDC, outcome.Warded.Save.DC)
		})
	}
}

// The cast path reads the same DC: a Bane on a creature warded by a caster who
// is not in the cast still meets the ward.
func TestABaneOnAWardWhoseCasterLeftStillMeetsTheWard(t *testing.T) {
	fixtures := castFixtures(t)
	wolf := fixtures.wolfData()
	wolf.Conditions = []json.RawMessage{sanctuaryJSON(t, wolfID)}

	roller := &actionRoller{singles: []int{5}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		World:        fixtures.world(),
		Participants: []Participant{{Monster: wolf}, {Character: baneCaster(1, 2)}},
		Machine:      machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	outcome := out.Outcome.(CastOutcome)
	require.NotNil(t, outcome.Targets[0].Warded)
	require.Equal(t, wardDC, outcome.Targets[0].Warded.Save.DC)
}

// FAIL CLOSED at the cast. A caster with no spell save DC cannot impose a ward
// that keeps one: the CAST is refused at the door, before any ward exists to
// be rolled against at DC 0.
func TestACasterWithNoSpellSaveDCCannotCastSanctuary(t *testing.T) {
	caster := baneCaster(1, 2)
	caster.ClassID = "fighter" // no spellcasting ability: SpellSaveDC is 0

	warded, err := castSanctuaryOnHero(t, caster)
	require.ErrorIs(t, err, ErrBadAction)
	require.ErrorContains(t, err, bardID, "the refusal names the caster")
	require.Nil(t, warded)
}

// FAIL CLOSED at the ward. A ward stored before wards kept their DC carries
// none; a save against DC 0 always succeeds, so the strike is refused.
func TestAWardWithNoDCRefusesTheStrike(t *testing.T) {
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(sanctuaryJSON(t, heroID), &fields))
	delete(fields, "save_dc")
	old, err := json.Marshal(fields)
	require.NoError(t, err)

	target := actionHero()
	target.Conditions = []json.RawMessage{old}
	roller := &actionRoller{singles: []int{20}}
	_, err = strikeOn(t, target, roller, Participant{Character: clericWarder()})
	require.ErrorIs(t, err, ErrWardUnreadable)
	require.ErrorIs(t, err, conditions.ErrWardWithoutDC, "the ward's own refusal rides inside")
	require.ErrorContains(t, err, heroID, "the error names the ward's holder")
	require.ErrorContains(t, err, "cleric-1", "and its caster")
	require.Zero(t, roller.calls, "no save was rolled against a DC of zero")
}

// A contested effect that declares a save DC key is refused rather than
// imposed without the DC it declared: only the gateless delivery binds one.
func TestAContestedEffectKeepingASaveDCIsRefused(t *testing.T) {
	definition := baneDefinition()
	profile := definition.Cast.Clone() // never write through to shared content
	profile.Effects[0].SaveDCKey = "save_dc"
	definition.Cast = &profile

	_, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: bardID, TargetIDs: []string{wolfID},
		Roller: &actionRoller{singles: []int{5}},
	})
	require.ErrorIs(t, err, ErrBadAction)
	require.ErrorContains(t, err, "save DC")
}
