// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// rage_wearer_test.go plays "a wearer rages from an item" (rpg-project#1131,
// slice rpg-project#1292) on the second-wind walk site: a fighter, who is not
// a barbarian, wears a cloak with two charges whose one grant lets her rage
// for a charge (`use: rage`, `with: { damage_bonus: 2 }`, `cost: { charges: 1 }`
// in a site's words). The module adds nothing: the rules offer the row, the
// door pays and refuses. These tests prove it through Afford and Activate.
//
// The dice are secondWindDice. The Sparring Guard is given hit points to
// spare so her swing, which keeps the rage up past the turn's end, does not
// end the fight.

const (
	rageCloakName    = "Cloak of fury"
	rageCloakRowName = "Rage · Cloak of fury"
)

func rageCloak(t *testing.T, id string) magicitems.Data {
	t.Helper()
	item, err := magicitems.New(id, magicitems.Description{
		Ref:   core.Ref{Module: "site", Type: "items", ID: "cloak-of-fury"},
		Name:  rageCloakName,
		Slots: []string{"cloak"},
		Pools: map[string]magicitems.Pool{
			"charges": {Maximum: 2, RestoreOn: []coreResources.ResetType{coreResources.ResetLongRest}},
		},
		Grants: map[string]magicitems.Grant{
			"rage": {
				Ability:    *refs.Features.Rage(),
				Parameters: json.RawMessage(`{"damage_bonus":2}`),
				Pool:       "charges",
				Uses:       1,
			},
		},
	})
	require.NoError(t, err)
	return item
}

// rageFight is alice, wearing the cloak, in the second-wind-walk fight, her
// turn first.
type rageFight struct {
	t     *testing.T
	mgr   *session.Manager
	chars *fakeCharacters
	cloak string
}

func newRageFight(t *testing.T) *rageFight {
	t.Helper()
	alice := armedFighter("alice")
	alice.HitPoints, alice.MaxHitPoints = 40, 40
	wear(t, alice, map[character.InventorySlot]magicitems.Data{character.SlotCloak: rageCloak(t, "cloak-1")})

	raw, err := os.ReadFile("testdata/second-wind-walk.yaml")
	require.NoError(t, err)
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err)
	sessions := newFakeSessions()
	f := &rageFight{t: t, chars: newFakeCharacters(alice), cloak: "cloak-1"}
	f.mgr, err = session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &secondWindDice{},
		TurnDriver: session.Driver(), Sessions: sessions, Encounters: newFakeEncounters(), Characters: f.chars,
	})
	require.NoError(t, err)
	_, err = f.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.NoError(t, err)
	turn, err := f.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active)
	for i := range sessions.byID["sess"].NPCs {
		if sessions.byID["sess"].NPCs[i].ID == "guard" {
			sessions.byID["sess"].NPCs[i].HitPoints = 500
		}
	}
	return f
}

func (f *rageFight) stored() *character.Data { return storedSheet(f.t, f.chars, "alice") }

// row is her Rage row.
func (f *rageFight) row() session.Declaration {
	f.t.Helper()
	panel, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(f.t, err)
	var found []session.Declaration
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == refs.Features.Rage().String() {
			found = append(found, d)
		}
	}
	require.Len(f.t, found, 1, "exactly one Rage row")
	return found[0]
}

func (f *rageFight) activate(id string) (*session.ActivateOutput, error) {
	return f.mgr.Activate(context.Background(), &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
}

// swingAndPass swings at the guard (a swing keeps the rage up) and ends her
// turn; the guard has his, and it is hers again.
func (f *rageFight) swingAndPass() {
	f.t.Helper()
	_, err := f.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "guard",
		DeclarationID: currentAttackID(f.t, f.mgr, "sess", "alice"),
	})
	require.NoError(f.t, err)
	_, err = f.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(f.t, f.mgr, "sess", "alice"),
	})
	require.NoError(f.t, err)
	turn, err := f.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(f.t, err)
	require.Equal(f.t, "alice", turn.Active, "it is alice's turn again")
}

// Test 6. She is offered it.
func TestTheWearersRageIsOfferedAsABonusActionForACharge(t *testing.T) {
	f := newRageFight(t)

	row := f.row()
	require.Equal(t, rageCloakRowName, row.Ability.Name)
	require.Equal(t, &session.AbilityGiver{ItemID: f.cloak, ItemName: rageCloakName, Grant: "rage"}, row.Ability.Giver)
	require.Equal(t, session.SlotBonus, row.Slot)
	require.True(t, row.Available)
	require.Nil(t, row.Why)
	require.Equal(t, 2, itemLeft(f.stored(), f.cloak), "the count is 2 of 2")
	require.Equal(t, 2, f.stored().Items[0].Description.Pools["charges"].Maximum)
}

// Test 7. She uses it through Manager.Activate.
func TestTheWearerRagesThroughTheDoor(t *testing.T) {
	f := newRageFight(t)
	mark := lastSeq(storyBeats(t, f.mgr, "sess", "alice"))

	out, err := f.activate(f.row().ID)
	require.NoError(t, err)
	require.Equal(t, refs.Features.Rage().String(), out.Ability)

	saved := f.stored()
	require.Zero(t, saved.ActionEconomy.BonusActionsRemaining, "her bonus action is spent")
	require.Equal(t, 1, itemLeft(saved, f.cloak), "one charge is spent")
	status := ragingStatusOf(t, saved.Conditions)
	require.NotNil(t, status, "her sheet holds the raging status")
	require.Equal(t, refs.Features.Rage().String(), status.Origin.Ref)
	require.Equal(t, "alice", status.Origin.EntityID)

	events := activationEvents(t, storyBeatsAfter(t, f.mgr, "sess", "alice", mark))
	require.Equal(t, encounter.BeatActivated, events[0].Beat)
	require.Equal(t, encounter.BeatActivationResult, events[1].Beat)
	result := resultOf(t, events[1])
	require.Equal(t, "alice", result.Actor)
	require.Equal(t, encounter.ResultConditionApplied, result.Kind)
	require.Equal(t, "alice", result.Target)
	require.Equal(t, refs.Conditions.Raging().String(), result.Ref)
	require.Equal(t, "Raging", result.Name)
}

// Test 8. She is refused after: next turn the bonus action is back and a
// charge is left, and she is still raging.
func TestTheWearerIsRefusedARageWhileSheRages(t *testing.T) {
	f := newRageFight(t)
	id := f.row().ID
	_, err := f.activate(id)
	require.NoError(t, err)
	f.swingAndPass()

	before := f.stored()
	require.Equal(t, 1, before.ActionEconomy.BonusActionsRemaining, "her bonus action is back")
	require.Equal(t, 1, itemLeft(before, f.cloak), "a charge is left")
	require.NotNil(t, ragingStatusOf(t, before.Conditions), "and she is still raging")
	marks := storyLengths(t, f.mgr, "alice")

	row := f.row()
	require.False(t, row.Available, "the row is not offered")
	require.NotNil(t, row.Why)
	require.Contains(t, row.Why.Text, "already raging")
	t.Logf("the offer's reason: %s", row.Why.Text)

	out, err := f.activate(row.ID)
	require.Nil(t, out)
	// The verb regenerates the offers and refuses a selector naming one that is
	// no longer available as stale (activate.go, selectCompiledOffer) BEFORE it
	// asks the sheet for the item's ability, so the words "already raging" are
	// the offer's (above), and the verb's answer is the barbarian's own
	// "no charges" answer (barbarian_rage_door_test.go). The brief expected
	// ErrCannotActivate here: reported as a gap on the issue.
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.NotErrorIs(t, err, session.ErrCannotActivate)
	t.Logf("the door's answer: %v", err)

	require.Equal(t, before, f.stored(), "her sheet is unchanged")
	require.Equal(t, marks, storyLengths(t, f.mgr, "alice"), "no beat is told")
}
