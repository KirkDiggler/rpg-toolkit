// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

// activationSelector asks Afford for the current offer for one ability and
// hands back its selector — which is exactly what a client does.
func activationSelector(t *testing.T, mgr *session.Manager, member, ref string) string {
	t.Helper()
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	require.NoError(t, err)
	for _, d := range out.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == ref {
			return d.ID
		}
	}
	t.Fatalf("no activation offer for %q", ref)
	return ""
}

func storedSheet(t *testing.T, chars *fakeCharacters, id string) *character.Data {
	t.Helper()
	sheet, err := chars.GetCharacter(context.Background(), id)
	require.NoError(t, err)
	return sheet
}

func storedConditionRefs(t *testing.T, chars *fakeCharacters, id string) []string {
	t.Helper()
	out := make([]string, 0)
	for _, raw := range storedSheet(t, chars, id).Conditions {
		var envelope struct {
			Ref string `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(raw, &envelope))
		out = append(out, envelope.Ref)
	}
	return out
}

// THE TEST THE WHOLE SLICE IS FOR, and the one #294 and #295 could not have
// written: a player activates something, and it is there afterwards.
//
// The assertion is on what the REPOSITORY holds, not on the returned value or
// an in-memory sheet. Rage does not attach its own condition — it publishes one
// and the owner's keeper applies it — so a version of this that read back the
// object it just acted on would pass with the bus missing entirely.
func TestRagingSurvivesTheRoundTripToStorage(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, chars := aFight(t, alice, []int{1, 1})

	id := activationSelector(t, mgr, "alice", "dnd5e:features:rage")
	out, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})

	require.NoError(t, err)
	require.Equal(t, "dnd5e:features:rage", out.Ability)
	require.Contains(t, out.Saved.Written, "character:alice")
	require.Contains(t, storedConditionRefs(t, chars, "alice"), "dnd5e:conditions:raging")
}

// The charge and the bonus action come off the STORED sheet too, from the same
// call. A rage that applied its condition without spending anything would be
// free and permanent.
func TestActivatingSpendsFromTheStoredSheet(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, chars := aFight(t, alice, []int{1, 1})

	id := activationSelector(t, mgr, "alice", "dnd5e:features:rage")
	_, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.NoError(t, err)

	sheet := storedSheet(t, chars, "alice")
	require.Equal(t, 1, sheet.Resources[resources.RageCharges].Current)
	require.NotNil(t, sheet.ActionEconomy)
	require.Equal(t, 0, sheet.ActionEconomy.BonusActionsRemaining, "rage costs the bonus action")
	require.Equal(t, 1, sheet.ActionEconomy.ActionsRemaining, "and nothing else")
}

// Dodge is the other arm of the lookup — a combat ability rather than a
// feature, with no resource behind it.
func TestDodgingSurvivesTheRoundTripToStorage(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, chars := aFight(t, alice, []int{1, 1})

	id := activationSelector(t, mgr, "alice", "dnd5e:combat_abilities:dodge")
	_, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.NoError(t, err)

	require.Contains(t, storedConditionRefs(t, chars, "alice"), "dnd5e:conditions:dodging")
	require.Equal(t, 0, storedSheet(t, chars, "alice").ActionEconomy.ActionsRemaining)
}

// A SELECTOR SPENT ONCE IS STALE. The second call regenerates the offer,
// finds the bonus action gone, and refuses — which is the whole reason
// execution regenerates rather than trusting what it was handed.
func TestASpentSelectorIsStale(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})

	id := activationSelector(t, mgr, "alice", "dnd5e:features:rage")
	_, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.NoError(t, err)

	_, err = mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
}

func TestAnUnknownSelectorIsStale(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})

	_, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: "v1.not-a-real-selector",
	})
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
}

func TestActivatingWithoutASelectorIsRefused(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})

	_, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice",
	})
	require.ErrorIs(t, err, session.ErrNoDeclarationID)
}

// An ability with no charges left is not AVAILABLE, so its selector never
// survives regeneration. The refusal a player sees for that is the one Afford
// already gave them — this is the door agreeing with the panel.
func TestAnUnavailableAbilityCannotBeActivated(t *testing.T) {
	alice := ragingBarbarian("alice", 0)
	mgr, _, _, chars := aFight(t, alice, []int{1, 1})

	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	var id string
	for _, d := range out.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == "dnd5e:features:rage" {
			require.False(t, d.Available)
			id = d.ID
		}
	}
	require.NotEmpty(t, id, "a refused offer still carries its selector")

	_, err = mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	// AND NOTHING MOVED. A refusal that had already spent the bonus action
	// would be the worst of both.
	require.NotContains(t, storedConditionRefs(t, chars, "alice"), "dnd5e:conditions:raging")
}

func TestActivateRefusesANilInput(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})

	_, err := mgr.Activate(context.Background(), nil)
	require.ErrorIs(t, err, session.ErrNilInput)
}

// aTwoPlayerFight puts two characters in one hall and starts a fight, so a
// test can ask what somebody does on a turn that is not theirs.
func aTwoPlayerFight(
	t *testing.T, alice, bob *character.Data,
) (*session.Manager, *fakeCharacters) {
	t.Helper()
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	chars := newFakeCharacters(alice, bob)
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions,
		Encounters: encounters, Characters: chars, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{},
		TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: encounter.MemberID(alice.ID), Kind: encounter.KindPlayer,
				Position: spatial.Position{X: 1, Y: 1}},
			{ID: encounter.MemberID(bob.ID), Kind: encounter.KindPlayer,
				Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	require.NoError(t, err)

	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)

	return mgr, chars
}

// --- The two gates mutation testing found untested ---

// A DOWNED MEMBER CANNOT ACTIVATE. Found by removing the gate and watching
// every test still pass: the whole suite acted through a standing barbarian,
// so nothing distinguished a build that checked from one that did not.
func TestADownedMemberCannotActivate(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, chars := aFight(t, alice, []int{1, 1})

	id := activationSelector(t, mgr, "alice", "dnd5e:features:rage")

	// Dropped AFTER the fight formed and after the offer was taken, so the
	// clock still names her active while the standing seam reads her downed —
	// which is also the real shape of this race.
	chars.byID["alice"].HitPoints = 0

	_, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})

	require.ErrorIs(t, err, session.ErrDowned)
	require.NotContains(t, storedConditionRefs(t, chars, "alice"), "dnd5e:conditions:raging")
}

// A MEMBER CANNOT ACTIVATE ON SOMEBODY ELSE'S TURN, and this is the one that
// mattered: nothing in a declaration selector encodes the round, so bob's own
// offer from his own turn REGENERATES IDENTICALLY on alice's. Without the
// clock gate the selector would validate and the rage would land out of turn.
//
// Also surfaced by mutation rather than by review — the gate was written from
// the pattern Attack and Move keep, and copied faithfully enough that nobody
// thought to test it.
func TestActivatingOnSomebodyElsesTurnIsRefused(t *testing.T) {
	alice, bob := ragingBarbarian("alice", 2), ragingBarbarian("bob", 2)
	mgr, chars := aTwoPlayerFight(t, alice, bob)

	ctx := context.Background()
	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, turn.Clock)

	// Whoever is NOT active takes an offer while it is not theirs to take.
	idle := "bob"
	if turn.Active != "alice" {
		idle = "alice"
	}

	_, err = mgr.Activate(ctx, &session.ActivateInput{
		Session: "sess", Member: idle, DeclarationID: "v1.any-selector-at-all",
	})

	require.ErrorIs(t, err, session.ErrNotYourTurn)
	require.NotContains(t, storedConditionRefs(t, chars, idle), "dnd5e:conditions:raging")
}
