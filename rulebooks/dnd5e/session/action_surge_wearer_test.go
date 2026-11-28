// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// action_surge_wearer_test.go plays "a wearer uses Action Surge from an item"
// (rpg-project#1131, slice rpg-project#1265) on the Lieutenant's cloak site
// (testdata/lieutenants-cloak-walk.yaml) with the cloak's one `lets` entry
// replaced by `use: action surge`, a charge. The module adds nothing: the
// rules (rulebooks/dnd5e v0.248.0) offer an item's ability that charges no
// slot as a free action, and this plays it through the manager's own verbs.
//
// TWO RUNS OF ONE SAVED CHARACTER. The site's own scene ends its fight when
// the Lieutenant is downed, so the fight she wears the cloak in is a second
// run: run one loots the cloak and puts it on in free roam; run two launches
// the same text over the same character store (a launch is a long rest, so
// the cloak is at 1 of 1) with the Lieutenant standing and her turn first.
//
// The dice are secondWindDice: every die is min(size, 10), so a d10 is a 10
// and every d20 a 10.

const (
	surgeRowName = "Action Surge · Lieutenant's cloak"
	surgeRef     = "dnd5e:features:action_surge"
)

// surgeCloakSite is the cloak site whose cloak lets its wearer use Action
// Surge for a charge. The text replaced occurs once, and the test says so.
func surgeCloakSite(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(cloakSitePath)
	require.NoError(t, err)
	const from = "      - use: second wind\n        with: { level: 1 }\n"
	require.Equal(t, 1, bytes.Count(raw, []byte(from)), "precondition: the cloak's one entry occurs once in the site")
	return string(bytes.Replace(raw, []byte(from), []byte("      - use: action surge\n"), 1))
}

// surgeFight is alice wearing the cloak in a fight, her turn first, the
// Lieutenant standing with hit points to spare.
type surgeFight struct {
	*cloakScene
	cloak string // the item's id on her sheet
}

func newSurgeFight(t *testing.T) *surgeFight {
	t.Helper()
	ctx := context.Background()
	site := surgeCloakSite(t)

	// Run one: the site as written. She downs the Lieutenant, loots the cloak
	// and puts it on in free roam.
	first := newCloakSceneOn(t, site, "sess-1", nil, "alice")
	_, err := first.loot("alice")
	require.NoError(t, err)
	require.Len(t, first.stored("alice").Items, 1)
	cloak := first.stored("alice").Items[0].ID
	_, err = first.mgr.Equip(ctx, &session.EquipInput{Character: "alice", Slot: string(character.SlotCloak), Item: cloak})
	require.NoError(t, err)

	// Run two: the same saved character, a new session, the Lieutenant up.
	s := &cloakScene{t: t, chars: first.chars, sess: newFakeSessions(), acts: &actCapture{},
		dice: &secondWindDice{}, seats: newFakeSeats(), member: "alice", id: "sess"}
	s.repo = &failingSaves{fakeCharacters: s.chars}
	s.mgr, err = session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: s.acts, Seats: s.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: s.dice,
		TurnDriver: session.Driver(), Sessions: s.sess, Encounters: newFakeEncounters(), Characters: s.repo,
	})
	require.NoError(t, err)
	compiled := compileCloakSite(t, site)
	_, err = s.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.NoError(t, err)
	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, session.ClockKind(turn.Clock), "a fight")
	require.Equal(t, "alice", turn.Active, "her turn first")
	for i := range s.sess.byID["sess"].NPCs {
		if s.sess.byID["sess"].NPCs[i].ID == "guard" {
			s.sess.byID["sess"].NPCs[i].HitPoints = 200
		}
	}
	return &surgeFight{cloakScene: s, cloak: cloak}
}

// row is the item's Action Surge row in her panel, and the whole panel's
// activate rows by name for what else is offered.
func (f *surgeFight) row() session.Declaration {
	f.t.Helper()
	panel, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(f.t, err)
	var found []session.Declaration
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == surgeRef {
			found = append(found, d)
		}
	}
	require.Len(f.t, found, 1, "exactly one Action Surge row")
	return found[0]
}

func (f *surgeFight) activate(id string) (*session.ActivateOutput, error) {
	return f.mgr.Activate(context.Background(), &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
}

func (f *surgeFight) attack() error {
	_, err := f.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "guard",
		DeclarationID: currentAttackID(f.t, f.mgr, "sess", "alice"),
	})
	return err
}

func (f *surgeFight) economy() *character.ActionEconomyData { return f.stored("alice").ActionEconomy }

// swings is alice's blows in her story.
func (f *surgeFight) swings() []witnessed {
	return beatsNamed(storyBeats(f.t, f.mgr, "sess", "alice"), "struck", "missed")
}

// Test 1. Her row.
func TestTheWearersActionSurgeIsOfferedAsAFreeActionInAFight(t *testing.T) {
	f := newSurgeFight(t)

	row := f.row()
	require.Equal(t, session.VerbActivate, row.Verb)
	require.Equal(t, session.SlotNone, row.Slot, "a free action: it draws none of her turn's three slots")
	require.True(t, row.Available)
	require.Nil(t, row.Why)
	require.Equal(t, surgeRowName, row.Ability.Name)
	require.Equal(t, &session.AbilityGiver{ItemID: f.cloak, ItemName: cloakName, Grant: "action-surge"}, row.Ability.Giver)
	require.Equal(t, session.TargetNone, row.TargetKind)
	require.Nil(t, row.Remaining, "an activation's offer carries no count: the count is the sheet's")

	// Its count, 1 of 1, is on her saved sheet.
	sheet := f.stored("alice")
	require.Equal(t, 1, itemLeft(sheet, f.cloak), "at 1 of 1")
	require.Equal(t, 1, sheet.Items[0].Description.Pools["charges"].Maximum)
	require.Equal(t, 1, sheet.Items[0].Description.Grants["action-surge"].Uses)

	// Her own Second Wind row is untouched and is a bonus action: the surge is
	// the only free one.
	panel, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == secondWindRef {
			require.Equal(t, session.SlotBonus, d.Slot)
		}
	}
}

// Test 2. Using it.
func TestUsingTheWearersActionSurgeSpendsTheChargeAndGivesAnAction(t *testing.T) {
	f := newSurgeFight(t)
	require.Equal(t, 1, f.economy().ActionsRemaining, "one action before")
	require.Equal(t, 1, f.economy().BonusActionsRemaining)

	out, err := f.activate(f.row().ID)
	require.NoError(t, err)
	require.Equal(t, surgeRef, out.Ability)
	require.Equal(t, "1 action", out.GrantedCapacity)
	require.Equal(t, []string{"character:alice", "encounter:sess"}, out.Saved.Written)

	saved := f.stored("alice")
	require.Zero(t, itemLeft(saved, f.cloak), "the cloak's charge is spent")
	require.Equal(t, 2, saved.ActionEconomy.ActionsRemaining, "two actions")
	require.Equal(t, 1, saved.ActionEconomy.BonusActionsRemaining, "her bonus action is untouched")
}

// Test 3. What the second action buys.
func TestWithTheSecondActionTheWearerAttacksTwiceInATurn(t *testing.T) {
	t.Run("without the surge one action is one attack", func(t *testing.T) {
		f := newSurgeFight(t)
		require.NoError(t, f.attack())
		require.Zero(t, f.economy().ActionsRemaining)
		require.ErrorIs(t, f.attack(), session.ErrStaleDeclaration, "no second attack")
		require.Len(t, f.swings(), 1)
	})

	t.Run("with it, two", func(t *testing.T) {
		f := newSurgeFight(t)
		_, err := f.activate(f.row().ID)
		require.NoError(t, err)
		require.NoError(t, f.attack())
		require.Equal(t, 1, f.economy().ActionsRemaining, "one action left after the first blow")
		require.NoError(t, f.attack())
		require.Zero(t, f.economy().ActionsRemaining, "both spent")
		require.Len(t, f.swings(), 2, "the record shows two blows in her one turn")
		require.ErrorIs(t, f.attack(), session.ErrStaleDeclaration, "and not a third")
	})
}

// Test 4. It needs neither her action nor her bonus action.
func TestTheWearersActionSurgeNeedsNeitherHerActionNorHerBonusAction(t *testing.T) {
	ctx := context.Background()
	f := newSurgeFight(t)

	require.NoError(t, f.attack(), "her action")
	panel, err := f.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == secondWindRef {
			_, err = f.activate(d.ID) // her bonus action
			require.NoError(t, err)
		}
	}
	spent := f.economy()
	require.Zero(t, spent.ActionsRemaining)
	require.Zero(t, spent.BonusActionsRemaining)

	row := f.row()
	require.True(t, row.Available, "still usable with both spent")
	require.Equal(t, session.SlotNone, row.Slot)
	_, err = f.activate(row.ID)
	require.NoError(t, err)
	got := f.economy()
	require.Equal(t, 1, got.ActionsRemaining, "it gives her one action back to spend")
	require.Zero(t, got.BonusActionsRemaining, "and takes no bonus action to do it")
	require.NoError(t, f.attack())
}

// Test 5. Spent.
func TestWithTheChargeSpentTheWearersActionSurgeIsNotUsable(t *testing.T) {
	f := newSurgeFight(t)
	row := f.row()
	_, err := f.activate(row.ID)
	require.NoError(t, err)
	before := f.stored("alice")

	spent := f.row()
	require.Equal(t, row.ID, spent.ID, "the same row")
	require.False(t, spent.Available)
	require.Equal(t, &session.Shortfall{
		Reason: session.ShortfallNoBudget, Currency: session.CurrencyCharges, Needed: 1, Left: 0,
		Text: "the " + cloakName + " has no charges left",
	}, spent.Why)

	out, err := f.activate(row.ID)
	require.Nil(t, out)
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.EqualError(t, err, "activate: declaration is stale")
	require.Equal(t, before.ActionEconomy, f.stored("alice").ActionEconomy, "no action was given")
	require.Zero(t, itemLeft(f.stored("alice"), f.cloak))
}

// Test 6. What a player is told, for her own surge: the shape of
// TestThePlayerIsToldTheLieutenantsActivationAsAPlayersIs, for a player's.
func TestThePlayerIsToldHerOwnActionSurgeAsAPlayersIs(t *testing.T) {
	f := newSurgeFight(t)
	_, err := f.activate(f.row().ID)
	require.NoError(t, err)

	story := storyBeats(t, f.mgr, "sess", "alice")
	activated := beatsNamed(story, encounter.BeatActivated)
	require.Len(t, activated, 1, "one activation")
	var told activatedBeat
	activated[0].decode(t, &told)
	require.Equal(t, "alice", told.Actor)
	require.Equal(t, session.SpellRef{Ref: surgeRef, Name: surgeRowName}, told.Ability)
	require.Empty(t, told.Target, "it lands on its user")

	results := beatsNamed(story, encounter.BeatActivationResult)
	require.Len(t, results, 1)
	result := resultOf(t, results[0])
	require.Equal(t, encounter.ActivationResultKind("capacity-granted"), result.Kind)
	require.Equal(t, "1 action", result.Description)

	var live []session.ActivateAct
	for _, act := range f.acts.of("alice") {
		if body, ok := act.Body.(session.ActivateAct); ok {
			live = append(live, body)
			require.Equal(t, "activate", act.Verb)
		}
	}
	require.Len(t, live, 1, "her client was handed it live")
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "alice"}, live[0].By)
	require.Equal(t, surgeRowName, live[0].Ability.Name)
	require.Equal(t, session.Party{}, live[0].Target)
	require.Len(t, live[0].Results, 1)
	require.Equal(t, "capacity-granted", live[0].Results[0].Kind)
	require.Equal(t, "1 action", live[0].Results[0].Description)
}
