// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// lieutenants_cloak_test.go plays "a looted item lands in the looter's
// inventory" (rpg-project#1169, story rpg-project#850) on its own small site,
// testdata/lieutenants-cloak-walk.yaml: the fighter downs the Lieutenant,
// loots the body, puts the cloak on and uses its Second Wind.
//
// The dice are secondWindDice: every die is min(size, 10); a d20 is a 20 while
// strike is set, so the fighter's blow lands as a critical hit.

const cloakSitePath = "testdata/lieutenants-cloak-walk.yaml"

// cloakSite is the site's text with each old/new pair applied; an old that is
// not in the file fails the test, so a variant cannot silently be the site.
func cloakSite(t *testing.T, pairs ...string) string {
	t.Helper()
	raw, err := os.ReadFile(cloakSitePath)
	require.NoError(t, err)
	text := string(raw)
	for i := 0; i < len(pairs); i += 2 {
		require.Contains(t, text, pairs[i], "the site has the text being changed")
		text = strings.Replace(text, pairs[i], pairs[i+1], 1)
	}
	return text
}

func compileCloakSite(t *testing.T, text string) dungeonspec.Compiled {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(text))
	require.NoError(t, err)
	return compiled
}

// failingSaves is a character store that refuses to save a sheet that owns an
// item while fail is set.
type failingSaves struct {
	*fakeCharacters
	fail bool
}

func (f *failingSaves) SaveCharacter(ctx context.Context, data *character.Data) error {
	if f.fail && len(data.Items) > 0 {
		return os.ErrInvalid
	}
	return f.fakeCharacters.SaveCharacter(ctx, data)
}

// cloakScene is the site launched with a party, the fight won.
type cloakScene struct {
	t      *testing.T
	mgr    *session.Manager
	chars  *fakeCharacters
	repo   *failingSaves
	sess   *fakeSessions
	acts   *actCapture
	dice   *secondWindDice
	seats  *fakeSeats
	member string
}

// newCloakScene launches the site text with the party, hurts alice, and has
// her down the guard. The fight is over when it returns.
func newCloakScene(t *testing.T, siteText string, party ...string) *cloakScene {
	t.Helper()
	ctx := context.Background()
	sheets := []*character.Data{secondWindFighter(t, "alice", 10, 40)}
	for _, id := range party[1:] {
		sheets = append(sheets, armedFighter(id))
	}
	s := &cloakScene{t: t, chars: newFakeCharacters(sheets...), sess: newFakeSessions(), acts: &actCapture{},
		dice: &secondWindDice{}, seats: newFakeSeats(), member: "alice"}
	s.repo = &failingSaves{fakeCharacters: s.chars}
	var err error
	s.mgr, err = session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: s.acts, Seats: s.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: s.dice,
		TurnDriver: session.Driver(), Sessions: s.sess, Encounters: newFakeEncounters(), Characters: s.repo,
	})
	require.NoError(t, err)
	compiled := compileCloakSite(t, siteText)
	_, err = s.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: party})
	require.NoError(t, err)
	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active)

	// A launch starts a member at full health: she is hurt once the fight is on.
	data := s.stored("alice")
	data.HitPoints = 10
	require.NoError(t, s.chars.SaveCharacter(ctx, data))
	for _, id := range party {
		require.NoError(t, s.seats.SaveSeat(ctx, &session.SeatData{Character: id, Session: "sess"}))
	}

	// The Lieutenant is one blow from done and she strikes it with a critical hit.
	for i := range s.sess.byID["sess"].NPCs {
		if s.sess.byID["sess"].NPCs[i].ID == "guard" {
			s.sess.byID["sess"].NPCs[i].HitPoints = 1
		}
	}
	s.dice.mu.Lock()
	s.dice.strike = true
	s.dice.mu.Unlock()
	_, err = s.mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "guard",
		DeclarationID: currentAttackID(t, s.mgr, "sess", "alice"),
	})
	require.NoError(t, err)
	s.dice.mu.Lock()
	s.dice.strike = false
	s.dice.mu.Unlock()
	return s
}

func (s *cloakScene) stored(id string) *character.Data { return storedSheet(s.t, s.chars, id) }

func (s *cloakScene) loot(member string) (*session.LootOutput, error) {
	return s.mgr.Loot(context.Background(), &session.LootInput{Session: "sess", Member: member, Target: "guard"})
}

// secondWindRows is every Second Wind row the member is offered, in order.
func (s *cloakScene) secondWindRows(member string) []session.Declaration {
	s.t.Helper()
	panel, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	require.NoError(s.t, err)
	var out []session.Declaration
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == secondWindRef {
			out = append(out, d)
		}
	}
	return out
}

func names(rows []session.Declaration) []string {
	out := make([]string, 0, len(rows))
	for _, d := range rows {
		out = append(out, d.Ability.Name)
	}
	return out
}

// ownedCloaks is the sheet's items whose rules name is the cloak.
func ownedCloaks(data *character.Data) []string {
	var out []string
	for _, item := range data.Items {
		if item.Description.Name == cloakName {
			out = append(out, item.ID)
		}
	}
	return out
}

// Test 1. The scene, played by one test.
func TestTheLieutenantsCloakIsLootedWornAndUsed(t *testing.T) {
	ctx := context.Background()
	s := newCloakScene(t, cloakSite(t), "alice")

	before := s.stored("alice")
	require.Empty(t, before.Items, "nothing owned before the loot")
	require.Equal(t, []string{"Second Wind"}, names(s.secondWindRows("alice")))

	_, err := s.loot("alice")
	require.NoError(t, err)

	saved := s.stored("alice")
	require.Len(t, saved.Items, 1, "the cloak is on the saved sheet")
	item := saved.Items[0]
	require.Equal(t, wornDescription("lieutenants-cloak", cloakName, "cloak"), item.Description,
		"the rules' reading of the author's block")
	require.Equal(t, 1, item.Left["charges"], "at 1 of 1")
	require.Empty(t, saved.EquipmentSlots[character.SlotCloak], "in the inventory, not worn")
	require.Equal(t, []string{"Second Wind"}, names(s.secondWindRows("alice")), "still one Second Wind row")

	// The fight is over, so the cloak goes on in free roam.
	_, err = s.mgr.Equip(ctx, &session.EquipInput{Character: "alice", Slot: string(character.SlotCloak), Item: item.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"Second Wind", cloakRowName}, names(s.secondWindRows("alice")))

	cloakRow := s.secondWindRows("alice")[1]
	require.Equal(t, item.ID, cloakRow.Ability.Giver.ItemID)
	hp := s.stored("alice").HitPoints
	_, err = s.mgr.Activate(ctx, &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: cloakRow.ID})
	require.NoError(t, err)
	after := s.stored("alice")
	require.Equal(t, hp+11, after.HitPoints, "the d10 (a 10) plus the cloak's 1")
	require.Zero(t, itemLeft(after, item.ID), "the cloak is at 0 of 1")
	require.Equal(t, 1, after.Resources["second_wind"].Current, "the fighter's own Second Wind is untouched")
}

// Test 3. A second loot of the same body adds nothing.
func TestLootingTheSameBodyTwiceAddsNothing(t *testing.T) {
	s := newCloakScene(t, cloakSite(t), "alice")
	_, err := s.loot("alice")
	require.NoError(t, err)
	first := s.stored("alice")
	require.Len(t, first.Items, 1)

	_, err = s.loot("alice")
	require.NoError(t, err)
	require.Equal(t, first.Items, s.stored("alice").Items, "the second loot found nothing to take")
}

// Tests 4 and 8. A second player who was standing there is told nothing of
// what was taken, and no act piece is produced for a `taken` beat.
func TestWhatTheLooterTookIsTheLootersAlone(t *testing.T) {
	s := newCloakScene(t, cloakSite(t), "alice", "bob")
	_, err := s.loot("alice")
	require.NoError(t, err)

	require.Empty(t, s.stored("bob").Items, "bob owns nothing from the body")
	require.Empty(t, ownedCloaks(s.stored("bob")))

	bobs := storyBeats(t, s.mgr, "sess", "bob")
	require.Len(t, beatsNamed(bobs, encounter.BeatLooted), 1, "bob heard the loot")
	require.Empty(t, beatsNamed(bobs, encounter.BeatTaken), "and not what was taken")
	require.Len(t, beatsNamed(storyBeats(t, s.mgr, "sess", "alice"), encounter.BeatTaken), 1,
		"alice has her own record of it")

	// No act piece for it, to either of them: the beat folds into nothing.
	for _, member := range []string{"alice", "bob"} {
		for _, act := range s.acts.of(member) {
			raw, err := json.Marshal(act)
			require.NoError(t, err)
			require.NotContains(t, string(raw), `"taken"`, "%s's act stream has no taken piece", member)
			require.NotContains(t, strings.ToLower(string(raw)), strings.ToLower(cloakName), "%s's act stream names no item", member)
		}
	}
}

// Test 5. THE LAW: the answer is the same whether the body carried anything.
func TestLootAnswersTheSameWhateverTheBodyCarried(t *testing.T) {
	with := newCloakScene(t, cloakSite(t), "alice")
	without := newCloakScene(t, cloakSite(t, "        equipment: [lieutenants-cloak]\n", ""), "alice")

	gotWith, err := with.loot("alice")
	require.NoError(t, err)
	gotWithout, err := without.loot("alice")
	require.NoError(t, err)

	require.Len(t, with.stored("alice").Items, 1)
	require.Empty(t, without.stored("alice").Items)
	// Both sessions are "sess" and the body is "guard" in both, so nothing
	// in the answers names a body of its own: the two are compared whole.
	require.Equal(t, gotWithout, gotWith, "the whole LootOutput, Saved and Delivery, is equal")
	require.Equal(t, []string{"character:alice", "encounter:sess"}, gotWith.Saved.Written)
}

// Test 6, one level up: a delivery that fails refuses the loot, saves nothing,
// and the body keeps what it wore.
func TestADeliveryThatFailsRefusesTheLootAndTheBodyKeepsItsCloak(t *testing.T) {
	s := newCloakScene(t, cloakSite(t), "alice")

	s.repo.fail = true
	out, err := s.loot("alice")
	require.Error(t, err)
	require.Nil(t, out)
	require.Empty(t, s.stored("alice").Items, "nothing was saved")

	s.repo.fail = false
	_, err = s.loot("alice")
	require.NoError(t, err)
	require.Len(t, s.stored("alice").Items, 1, "the body still wore the cloak, so the loot finds it")
}

// Test 7. Every custom item is judged when a run starts.
func TestCheckContentJudgesEveryCustomItem(t *testing.T) {
	check := func(text string) *session.CheckContentOutput {
		compiled := compileCloakSite(t, text)
		out, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
		require.NoError(t, err)
		return out
	}

	require.Empty(t, check(cloakSite(t)).Items, "the walk site's cloak is read by the rules")

	cape := check(cloakSite(t, "slot: cloak", "slot: cape"))
	require.Len(t, cape.Items, 1)
	require.Equal(t, "lieutenants-cloak", cape.Items[0].ItemID)
	require.Equal(t, "customItems.lieutenants-cloak", cape.Items[0].Path)
	require.Contains(t, cape.Items[0].Message, "cape")
	require.Empty(t, cape.Refusals, "statuses are unchanged")

	third := check(cloakSite(t, "use: second wind", "use: third wind"))
	require.Len(t, third.Items, 1)
	require.Contains(t, third.Items[0].Message, "third wind")

	// The same item named by a creature and by a prop is judged once.
	both := cloakSite(t,
		"    items: []\n", "    items:\n      - id: cloak-on-the-peg\n        kind: prop\n        assetRef: dnd5e:props:scroll\n        label: The Cloak\n        transform: {x: -1.5, y: 0, z: 0.75, rotationY: 0}\n",
		"    propDeclarations: {}\n", "    propDeclarations:\n      cloak-on-the-peg:\n        blocksMovement: false\n        blocksLineOfSight: false\n        footprint: {width: 0.3, depth: 0.3, offsetX: 0, offsetZ: 0}\n",
		"    doorBindings:", "    propBindings:\n      cloak-on-the-peg:\n        item: lieutenants-cloak\n    doorBindings:",
		"slot: cloak", "slot: cape")
	require.Len(t, check(both).Items, 1, "judged once")

	// A prop alone is read too: the compiled site carries a placed prop's
	// item, and the check finds it with no creature naming it.
	propOnly := cloakSite(t,
		"        equipment: [lieutenants-cloak]\n", "",
		"    items: []\n", "    items:\n      - id: cloak-on-the-peg\n        kind: prop\n        assetRef: dnd5e:props:scroll\n        label: The Cloak\n        transform: {x: -1.5, y: 0, z: 0.75, rotationY: 0}\n",
		"    propDeclarations: {}\n", "    propDeclarations:\n      cloak-on-the-peg:\n        blocksMovement: false\n        blocksLineOfSight: false\n        footprint: {width: 0.3, depth: 0.3, offsetX: 0, offsetZ: 0}\n",
		"    doorBindings:", "    propBindings:\n      cloak-on-the-peg:\n        item: lieutenants-cloak\n    doorBindings:",
		"slot: cloak", "slot: cape")
	require.Len(t, check(propOnly).Items, 1, "a prop's item is judged with no creature naming it")

	// Starting a session on such a site is refused.
	compiled := compileCloakSite(t, cloakSite(t, "slot: cloak", "slot: cape"))
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &secondWindDice{},
		TurnDriver: session.Driver(), Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(secondWindFighter(t, "alice", 10, 40))})
	require.NoError(t, err)
	_, err = mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.ErrorIs(t, err, session.ErrUnknownContent)
	require.Contains(t, err.Error(), "lieutenants-cloak")
	require.Contains(t, err.Error(), "customItems.lieutenants-cloak")
}

// A creature launched from the site carries its equipment in the encounter
// the session saved, read from the saved data and not through a verb.
func TestALaunchedCreatureCarriesItsEquipment(t *testing.T) {
	ctx := context.Background()
	compiled := compileCloakSite(t, cloakSite(t))
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &secondWindDice{},
		TurnDriver: session.Driver(), Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(secondWindFighter(t, "alice", 10, 40))})
	require.NoError(t, err)
	_, err = mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.NoError(t, err)

	enc, err := mgr.LoadWorldForTest(ctx, "sess")
	require.NoError(t, err)
	var guard *encounter.MemberData
	data := enc.ToData()
	for i := range data.Members {
		if data.Members[i].ID == "guard" {
			guard = &data.Members[i]
		}
	}
	require.NotNil(t, guard, "the guard is in the saved encounter")
	require.Len(t, guard.Equipment, 1)
	require.Equal(t, "lieutenants-cloak", guard.Equipment[0].ID)
	require.Equal(t, cloakName, guard.Equipment[0].Name)
	require.Contains(t, string(guard.Equipment[0].Item), `"slot":"cloak"`)
	for _, m := range data.Members {
		if m.ID != "guard" {
			require.Empty(t, m.Equipment, "%s carries nothing", m.ID)
		}
	}
}
