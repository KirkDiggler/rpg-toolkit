// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// fighter_action_surge_test.go plays "a fighter uses her own Action Surge"
// (rpg-project#1131, slice rpg-project#1271).
//
// THE LEVEL-2 FIGHTER IS BUILT BY THE LEVEL, not seeded: the fixture is the
// level-1 fighter that has earned level 2 (advancingFighter), given a
// longsword so she can swing, and taken to level 2 through Manager.LevelUp.
// What she then holds (the feature, her shared pool) is exactly what the
// level gave her. No test here builds a sheet in the old shape.

// levelTwoFighter is alice, taken from level 1 to level 2 through LevelUp.
func levelTwoFighter(t *testing.T) *character.Data {
	t.Helper()
	data := advancingFighter("alice")
	data.Name = "alice"
	data.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponMartial}
	data.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
	}
	data.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: string(weapons.Longsword)}
	mgr, chars := advancementManager(t, testDice{}, data)
	_, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character: "alice", HitPointMethod: session.HitPointsAverage,
	})
	require.NoError(t, err)
	require.Equal(t, 2, chars.byID["alice"].Level)
	return chars.byID["alice"]
}

// surgeScene is the level-2 fighter beside bob on the duel world.
type surgeScene struct {
	t          *testing.T
	mgr        *session.Manager
	characters *fakeCharacters
	encounters *fakeEncounters
	sessions   *fakeSessions
	stream     *actCapture
}

// newSurgeScene launches the duel world and lays alice's sheet over the
// launch's own. inFight starts her turn on an authored clock.
func newSurgeScene(t *testing.T, alice *character.Data, inFight bool) *surgeScene {
	t.Helper()
	s := &surgeScene{
		t: t, sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")), stream: &actCapture{},
	}
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: s.stream, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	require.NoError(t, err)
	s.mgr = mgr
	launchScene(t, mgr, freeRoamDuelWorld())
	// A launch is a long rest; her own level-2 sheet goes down after it, as a
	// rejoin's sheet does in rest_test.go.
	s.characters.byID["alice"] = alice
	if inFight {
		authorTurnClock(t, s.encounters, testSession, duelClock, 0)
	}
	return s
}

// row is her Action Surge row, exactly one, or nil when none is offered.
func (s *surgeScene) row() *session.Declaration {
	s.t.Helper()
	panel, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(s.t, err)
	var found []session.Declaration
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == surgeRef {
			found = append(found, d)
		}
	}
	if len(found) == 0 {
		return nil
	}
	require.Len(s.t, found, 1, "exactly one Action Surge row")
	return &found[0]
}

func (s *surgeScene) activate(id string) (*session.ActivateOutput, error) {
	return s.mgr.Activate(context.Background(), &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
}

func (s *surgeScene) attack() error {
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
		DeclarationID: currentAttackID(s.t, s.mgr, "sess", "alice"),
	})
	return err
}

func (s *surgeScene) stored() *character.Data {
	s.t.Helper()
	return storedSheet(s.t, s.characters, "alice")
}

// swings is alice's blows in her story.
func (s *surgeScene) swings() []witnessed {
	return beatsNamed(storyBeats(s.t, s.mgr, "sess", "alice"), "struck", "missed")
}

// surgePool is her shared pool, read from her stored sheet.
func (s *surgeScene) surgePool() character.RecoverableResourceData {
	return s.stored().Resources[resources.ActionSurge]
}

const fighterSurgeName = "Action Surge"

// Test 1. It arrives with the level.
func TestTheFighterArrivesWithHerActionSurgeAtLevelTwo(t *testing.T) {
	before := advancingFighter("alice")
	require.Empty(t, before.Features, "a level-1 fighter has no Action Surge")
	require.NotContains(t, before.Resources, resources.ActionSurge)

	alice := levelTwoFighter(t)
	require.Equal(t, character.RecoverableResourceData{
		Current: 1, Maximum: 1, ResetType: coreResources.ResetShortRest,
	}, alice.Resources[resources.ActionSurge], "the level hands her the pool full: 1 of 1")
	require.Len(t, alice.Features, 1, "and the feature")

	s := newSurgeScene(t, alice, true)
	row := s.row()
	require.NotNil(t, row, "her next fight offers it")
	require.Equal(t, session.VerbActivate, row.Verb)
	require.Equal(t, session.SlotNone, row.Slot, "a free action: it draws none of her turn's three slots")
	require.True(t, row.Available)
	require.Nil(t, row.Why)
	require.Equal(t, fighterSurgeName, row.Ability.Name)
	require.Nil(t, row.Ability.Giver, "her own: no item gives it")
	require.Equal(t, session.TargetNone, row.TargetKind)
	require.Equal(t, 1, s.surgePool().Current)
	require.Equal(t, 1, s.surgePool().Maximum, "1 of 1")
}

// Test 2. Using it through the door.
func TestTheFighterUsesHerOwnActionSurgeThroughTheDoor(t *testing.T) {
	s := newSurgeScene(t, levelTwoFighter(t), true)
	require.Equal(t, 1, s.stored().Resources[resources.ActionSurge].Current)
	out, err := s.activate(s.row().ID)
	require.NoError(t, err)
	require.Equal(t, surgeRef, out.Ability)
	require.Equal(t, "1 action", out.GrantedCapacity)
	require.Equal(t, []string{"character:alice", "encounter:sess"}, out.Saved.Written)

	saved := s.stored()
	require.Zero(t, s.surgePool().Current, "one use spent from her shared pool")
	require.Equal(t, 1, s.surgePool().Maximum, "0 of 1")
	require.Equal(t, 2, saved.ActionEconomy.ActionsRemaining, "two actions")
	require.Equal(t, 1, saved.ActionEconomy.BonusActionsRemaining, "her bonus action is untouched")

	story := storyBeats(t, s.mgr, "sess", "alice")
	activated := beatsNamed(story, encounter.BeatActivated)
	require.Len(t, activated, 1, "one activation")
	var told activatedBeat
	activated[0].decode(t, &told)
	require.Equal(t, "alice", told.Actor)
	require.Equal(t, session.SpellRef{Ref: surgeRef, Name: fighterSurgeName}, told.Ability,
		"named Action Surge, not for an item")
	require.Empty(t, told.Target, "it lands on its user")
	results := beatsNamed(story, encounter.BeatActivationResult)
	require.Len(t, results, 1)
	result := resultOf(t, results[0])
	require.Equal(t, encounter.ActivationResultKind("capacity-granted"), result.Kind)
	require.Equal(t, "1 action", result.Description)
}

// Test 3. Two attacks in the turn.
func TestWithTheSecondActionTheFighterAttacksTwiceInATurn(t *testing.T) {
	s := newSurgeScene(t, levelTwoFighter(t), true)
	_, err := s.activate(s.row().ID)
	require.NoError(t, err)
	require.NoError(t, s.attack())
	require.Equal(t, 1, s.stored().ActionEconomy.ActionsRemaining, "one action left after the first blow")
	require.NoError(t, s.attack())
	require.Zero(t, s.stored().ActionEconomy.ActionsRemaining, "both spent")
	require.Len(t, s.swings(), 2, "the record shows two blows in her one turn")
	require.ErrorIs(t, s.attack(), session.ErrStaleDeclaration, "and not a third")
}

// Test 4. It needs neither her action nor her bonus action.
func TestTheFightersActionSurgeNeedsNeitherHerActionNorHerBonusAction(t *testing.T) {
	s := newSurgeScene(t, levelTwoFighter(t), true)
	require.NoError(t, s.attack(), "her action")
	// Her bonus action: she has no bonus-action ability of her own at level 2,
	// so it is spent on her saved sheet, where the panel reads her turn from.
	s.characters.byID["alice"].ActionEconomy.BonusActionsRemaining = 0
	spent := s.stored().ActionEconomy
	require.Zero(t, spent.ActionsRemaining, "her action is spent")
	require.Zero(t, spent.BonusActionsRemaining, "her bonus action is spent")

	row := s.row()
	require.True(t, row.Available, "still usable with both spent")
	require.Equal(t, session.SlotNone, row.Slot)
	_, err := s.activate(row.ID)
	require.NoError(t, err)
	got := s.stored().ActionEconomy
	require.Equal(t, 1, got.ActionsRemaining, "it gives her one action back to spend")
	require.Zero(t, got.BonusActionsRemaining, "and takes no bonus action to do it")
	require.NoError(t, s.attack())
}

// Test 5. Once.
func TestWithTheUseSpentTheFightersActionSurgeIsNotUsable(t *testing.T) {
	s := newSurgeScene(t, levelTwoFighter(t), true)
	row := s.row()
	_, err := s.activate(row.ID)
	require.NoError(t, err)
	before := s.stored()

	spent := s.row()
	require.Equal(t, row.ID, spent.ID, "the same row")
	require.False(t, spent.Available)
	require.NotNil(t, spent.Why)
	require.Equal(t, "no action surge uses remaining", spent.Why.Text)

	out, err := s.activate(row.ID)
	require.Nil(t, out)
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.EqualError(t, err, "activate: declaration is stale")
	after := s.stored()
	require.Equal(t, before.ActionEconomy, after.ActionEconomy, "no action was given")
	require.Equal(t, before.Resources, after.Resources, "nothing more is spent")
	require.Zero(t, s.surgePool().Current)
}

// Test 6. A short rest gives it back.
func TestAShortRestGivesTheFighterHerActionSurgeBack(t *testing.T) {
	ctx := context.Background()
	// Spent as an earlier fight leaves it: 0 of 1 on her saved sheet, at rest
	// in the world (a rest is refused in a fight).
	alice := levelTwoFighter(t)
	pool := alice.Resources[resources.ActionSurge]
	pool.Current = 0
	alice.Resources[resources.ActionSurge] = pool
	s := newSurgeScene(t, alice, false)
	require.Zero(t, s.surgePool().Current, "spent before the rest")

	_, err := s.mgr.Rest(ctx, &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice"}},
	})
	require.NoError(t, err)
	require.Equal(t, 1, s.surgePool().Current, "1 of 1 again")
	require.Equal(t, 1, s.surgePool().Maximum)

	authorTurnClock(t, s.encounters, testSession, duelClock, 0)
	row := s.row()
	require.NotNil(t, row)
	require.True(t, row.Available, "usable in the next fight")
	_, err = s.activate(row.ID)
	require.NoError(t, err)
	require.Equal(t, 2, s.stored().ActionEconomy.ActionsRemaining)
}
